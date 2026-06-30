package boeng

import (
	"reflect"
	"strings"
	"sync"
	"unicode"
)

// Loggable lets a type control how it appears in boeng logs and span attributes.
// Implement it on domain structs to centralize field naming and masking in one
// place — the domain — instead of scattering struct tags across the codebase.
// boeng calls LogFields() once per Run/Emit/Log/Op call.
//
//	func (u User) LogFields() map[string]any {
//	    return map[string]any{
//	        "user_id": u.UserID,
//	        "email":   boeng.MaskEmail(u.Email),
//	    }
//	}
//
// Types that don't implement Loggable fall back to reflection: exported fields
// only, names lowered to snake_case, sensitive keys auto-masked, zero values
// skipped (toggle via Config.IncludeZeroFields).
type Loggable interface {
	LogFields() map[string]any
}

// defaultSensitiveKeys is the case-insensitive substring set the reflection
// fallback auto-masks. Implementers of Loggable are not affected — the rule
// is intentionally only applied to the implicit "I didn't write LogFields"
// path so authors keep full control when they opt in.
var defaultSensitiveKeys = []string{"password", "passwd", "secret", "token", "apikey", "api_key", "authorization", "auth", "email", "phone", "ssn", "credit_card", "card_number"}

// extractFields turns subject into a flat map for log fields and span attributes.
// Priority:
//  1. nil → empty map
//  2. implements Loggable → use LogFields()
//  3. map[string]any → returned as-is
//  4. struct or *struct → reflect exported fields
//  5. anything else → empty map
func extractFields(v any) map[string]any {
	if v == nil {
		return map[string]any{}
	}
	if lg, ok := v.(Loggable); ok {
		if f := lg.LogFields(); f != nil {
			return f
		}
		return map[string]any{}
	}
	if m, ok := v.(map[string]any); ok {
		return m
	}
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return map[string]any{}
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return map[string]any{}
	}
	skipZero := !fieldsConfig().IncludeZeroFields
	rt := rv.Type()
	out := make(map[string]any, rt.NumField())
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if !f.IsExported() {
			continue
		}
		fv := rv.Field(i)
		if skipZero && fv.IsZero() {
			continue
		}
		key := fieldName(f)
		out[key] = maybeMask(key, fv.Interface())
	}
	return out
}

// fieldName returns the log-friendly name of a struct field:
//   - first token of `json:"..."` tag if present (e.g. `json:"user_id"`)
//   - otherwise the field name converted from CamelCase to snake_case
func fieldName(f reflect.StructField) string {
	if tag := f.Tag.Get("json"); tag != "" && tag != "-" {
		if comma := strings.IndexByte(tag, ','); comma >= 0 {
			tag = tag[:comma]
		}
		if tag != "" {
			return tag
		}
	}
	return camelToSnake(f.Name)
}

// camelToSnake converts CamelCase / camelCase / mixed identifiers to
// snake_case while preserving runs of uppercase letters as acronyms
// ("UserID" → "user_id", "HTTPStatus" → "http_status").
func camelToSnake(s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s) + 4)
	runes := []rune(s)
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) {
			prev := runes[i-1]
			next := rune(0)
			if i+1 < len(runes) {
				next = runes[i+1]
			}
			boundary := unicode.IsLower(prev) || unicode.IsDigit(prev) ||
				(unicode.IsUpper(prev) && next != 0 && unicode.IsLower(next))
			if boundary {
				b.WriteByte('_')
			}
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// maybeMask returns v masked when key matches a sensitive name. Strings are
// reduced to a length-preserving star pattern; non-strings are replaced with
// the literal "***".
func maybeMask(key string, v any) any {
	if !isSensitiveKey(key) {
		return v
	}
	if s, ok := v.(string); ok {
		if strings.Contains(key, "email") {
			return MaskEmail(s)
		}
		return maskString(s)
	}
	return "***"
}

func isSensitiveKey(key string) bool {
	low := strings.ToLower(key)
	for _, sk := range defaultSensitiveKeys {
		if strings.Contains(low, sk) {
			return true
		}
	}
	return false
}

func maskString(s string) string {
	switch {
	case s == "":
		return ""
	case len(s) <= 2:
		return strings.Repeat("*", len(s))
	default:
		return s[:1] + strings.Repeat("*", len(s)-2) + s[len(s)-1:]
	}
}

// MaskEmail returns an email with the local part collapsed: "j**n@doe.com".
// If s doesn't look like an email, the whole string is starred.
//
// Operates on runes, not bytes, so multi-byte local parts ("jöhn@doe.com")
// remain valid UTF-8 after masking. The mask length is rune-count, not
// byte-count.
func MaskEmail(s string) string {
	if s == "" {
		return ""
	}
	at := strings.LastIndex(s, "@")
	if at <= 0 || at == len(s)-1 {
		// Whole string starred (one star per rune)
		return strings.Repeat("*", runeCount(s))
	}
	local, domain := s[:at], s[at:]
	lr := []rune(local)
	switch len(lr) {
	case 1:
		return "*" + domain
	case 2:
		return string(lr[0]) + "*" + domain
	default:
		return string(lr[0]) + strings.Repeat("*", len(lr)-2) + string(lr[len(lr)-1]) + domain
	}
}

func runeCount(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}

// FieldsConfig tunes the reflection fallback. Exposed via Init so callers can
// flip behavior at process start without touching call sites.
type FieldsConfig struct {
	// IncludeZeroFields controls whether zero-valued exported fields are
	// emitted. Default false (zeros are skipped).
	IncludeZeroFields bool
}

var (
	fieldsCfgMu sync.RWMutex
	fieldsCfg   FieldsConfig
)

func fieldsConfig() FieldsConfig {
	fieldsCfgMu.RLock()
	defer fieldsCfgMu.RUnlock()
	return fieldsCfg
}

func setFieldsConfig(c FieldsConfig) {
	fieldsCfgMu.Lock()
	fieldsCfg = c
	fieldsCfgMu.Unlock()
}
