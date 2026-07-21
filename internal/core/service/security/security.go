package security

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/port"
	svc "github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/service"
)

// ===== PII DETECTION & MASKING =====

type PIIMasker struct {
	patterns    map[string]*MaskingPattern
	fieldRules  map[string]MaskingRule
	globalRules []GlobalMaskingRule
	mu          sync.RWMutex
	enabled     bool
}

type MaskingPattern struct {
	Name        string
	Pattern     *regexp.Regexp
	Replacement string
	Severity    PIISeverity
}

type MaskingRule struct {
	Action   MaskingAction
	Pattern  string
	Severity PIISeverity
}

type GlobalMaskingRule struct {
	FieldPattern *regexp.Regexp
	ValuePattern *regexp.Regexp
	Action       MaskingAction
	Severity     PIISeverity
}

type PIISeverity int

const (
	PIILow PIISeverity = iota
	PIIMedium
	PIIHigh
	PIICritical
)

type MaskingAction int

const (
	MaskPartial MaskingAction = iota
	MaskFull
	MaskHash
	MaskRemove
	MaskEncrypt
)

func NewPIIMasker() *PIIMasker {
	m := &PIIMasker{patterns: map[string]*MaskingPattern{}, fieldRules: map[string]MaskingRule{}, globalRules: []GlobalMaskingRule{}, enabled: true}
	m.addDefaultPatterns()
	return m
}

func (pm *PIIMasker) addDefaultPatterns() {
	patterns := map[string]MaskingPattern{
		"thai_id":         {Name: "Thai National ID", Pattern: regexp.MustCompile(`\b\d{1}-\d{4}-\d{5}-\d{2}-\d{1}\b`), Replacement: "x-xxxx-xxxxx-xx-x", Severity: PIIHigh},
		"thai_id_compact": {Name: "Thai National ID (compact)", Pattern: regexp.MustCompile(`\b\d{13}\b`), Replacement: "xxxxxxxxxxxxx", Severity: PIIHigh},
		"thai_phone":      {Name: "Thai Phone Number", Pattern: regexp.MustCompile(`(?:\+66|0)[\s-]?\d{2}[\s-]?\d{3}[\s-]?\d{4}\b`), Replacement: "xxx-xxx-xxxx", Severity: PIIMedium},
		"email":           {Name: "Email", Pattern: regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`), Replacement: "***@***.***", Severity: PIIMedium},
		"credit_card":     {Name: "Credit Card", Pattern: regexp.MustCompile(`\b(?:\d{4}[-\s]?){3}\d{4}\b`), Replacement: "****-****-****-****", Severity: PIIHigh},
		"ssn":             {Name: "SSN", Pattern: regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`), Replacement: "***-**-****", Severity: PIIHigh},
		"passport":        {Name: "Passport", Pattern: regexp.MustCompile(`\b[A-Z]{1,2}\d{6,9}\b`), Replacement: "XX######", Severity: PIIHigh},
		"api_key":         {Name: "API Key", Pattern: regexp.MustCompile(`\b[A-Za-z0-9]{32,}\b`), Replacement: "********************************", Severity: PIICritical},
		"jwt_token":       {Name: "JWT", Pattern: regexp.MustCompile(`eyJ[A-Za-z0-9-_]+\.[A-Za-z0-9-_]+\.[A-Za-z0-9-_]*`), Replacement: "eyJ***.***.***", Severity: PIICritical},
	}
	for k := range patterns {
		p := patterns[k]
		pm.patterns[k] = &p
	}
	pm.fieldRules["password"] = MaskingRule{Action: MaskFull, Severity: PIICritical}
	pm.fieldRules["secret"] = MaskingRule{Action: MaskFull, Severity: PIICritical}
	pm.fieldRules["private_key"] = MaskingRule{Action: MaskRemove, Severity: PIICritical}
	pm.fieldRules["access_token"] = MaskingRule{Action: MaskHash, Severity: PIIHigh}
	pm.fieldRules["refresh_token"] = MaskingRule{Action: MaskHash, Severity: PIIHigh}
}

func (pm *PIIMasker) MaskFields(fields map[string]interface{}) map[string]interface{} {
	if !pm.enabled {
		return fields
	}
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	masked := make(map[string]interface{}, len(fields))
	for k, v := range fields {
		masked[k] = pm.maskValue(k, v)
	}
	return masked
}

func (pm *PIIMasker) maskValue(fieldName string, value interface{}) interface{} {
	if rule, ok := pm.fieldRules[strings.ToLower(fieldName)]; ok {
		return pm.applyMaskingAction(value, rule.Action)
	}
	for _, rule := range pm.globalRules {
		if rule.FieldPattern.MatchString(fieldName) {
			if s, ok := value.(string); ok && rule.ValuePattern.MatchString(s) {
				return pm.applyMaskingAction(value, rule.Action)
			}
		}
	}
	if s, ok := value.(string); ok {
		return pm.maskStringPatterns(s)
	}
	if pm.isStructWithTags(value) {
		return pm.maskStructFields(value)
	}
	return value
}

func (pm *PIIMasker) applyMaskingAction(value interface{}, action MaskingAction) interface{} {
	s, ok := value.(string)
	if !ok {
		return value
	}
	switch action {
	case MaskPartial:
		return pm.maskPartially(s)
	case MaskFull:
		return strings.Repeat("*", len(s))
	case MaskHash:
		return pm.hashValue(s)
	case MaskRemove:
		return "[REMOVED]"
	case MaskEncrypt:
		return pm.encryptValue(s)
	default:
		return value
	}
}

func (pm *PIIMasker) maskPartially(str string) string {
	n := len(str)
	if n <= 2 {
		return strings.Repeat("*", n)
	}
	if n <= 6 {
		return string(str[0]) + strings.Repeat("*", n-2) + string(str[n-1])
	}
	return str[:2] + strings.Repeat("*", n-4) + str[n-2:]
}
func (pm *PIIMasker) hashValue(str string) string {
	h := sha256.Sum256([]byte(str))
	return "[HASH:" + hex.EncodeToString(h[:8]) + "]"
}
func (pm *PIIMasker) encryptValue(str string) string {
	return "[ENCRYPTED:" + pm.hashValue(str)[7:15] + "]"
}
func (pm *PIIMasker) maskStringPatterns(s string) string {
	out := s
	for _, p := range pm.patterns {
		if p.Pattern.MatchString(out) {
			out = p.Pattern.ReplaceAllString(out, p.Replacement)
		}
	}
	return out
}
func (pm *PIIMasker) isStructWithTags(v interface{}) bool {
	if v == nil {
		return false
	}
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Kind() == reflect.Struct
}
func (pm *PIIMasker) maskStructFields(v interface{}) interface{} {
	rv := reflect.ValueOf(v)
	rt := reflect.TypeOf(v)
	if rt.Kind() == reflect.Ptr {
		rv = rv.Elem()
		rt = rt.Elem()
	}
	if rt.Kind() != reflect.Struct {
		return v
	}
	fields := make(map[string]interface{})
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		fv := rv.Field(i)
		if pii := f.Tag.Get("pii"); pii != "" {
			name := f.Name
			if j := f.Tag.Get("json"); j != "" {
				name = strings.Split(j, ",")[0]
			}
			fields[name] = pm.applyPIITag(pii, fv.Interface())
			continue
		}
		if lt := f.Tag.Get("log"); lt != "" {
			if lt == "-" {
				continue
			}
			name := f.Name
			if j := f.Tag.Get("json"); j != "" {
				name = strings.Split(j, ",")[0]
			}
			opts := strings.Split(lt, ",")
			base := opts[0]
			if base == "" {
				base = name
			}
			fields[base] = pm.applyTagMasking(fv.Interface(), opts[1:])
		}
	}
	return fields
}
func (pm *PIIMasker) applyPIITag(pii string, val interface{}) interface{} {
	s, ok := val.(string)
	if !ok {
		return val
	}
	switch strings.ToLower(pii) {
	case "email":
		return pm.maskStringPatterns(s)
	case "phone":
		return pm.maskStringPatterns(s)
	case "password":
		return strings.Repeat("*", len(s))
	default:
		return pm.maskPartially(s)
	}
}
func (pm *PIIMasker) applyTagMasking(val interface{}, opts []string) interface{} {
	for _, o := range opts {
		switch {
		case o == "sensitive=true":
			return pm.applyMaskingAction(val, MaskFull)
		case strings.HasPrefix(o, "mask="):
			return pm.applyCustomMask(val, strings.TrimPrefix(o, "mask="))
		case o == "hash=true":
			return pm.applyMaskingAction(val, MaskEncrypt)
		case o == "encrypt=true":
			return pm.applyMaskingAction(val, MaskEncrypt)
		}
	}
	return val
}
func (pm *PIIMasker) applyCustomMask(val interface{}, pat string) interface{} {
	s, ok := val.(string)
	if !ok {
		return val
	}
	if len(pat) == 0 {
		return pm.maskPartially(s)
	}
	out := make([]byte, len(s))
	plen := len(pat)
	for i, ch := range []byte(s) {
		pi := i % plen
		if pi < len(pat) {
			switch pat[pi] {
			case 'x', 'X':
				out[i] = ch
			case '*':
				out[i] = '*'
			default:
				out[i] = pat[pi]
			}
		} else {
			out[i] = '*'
		}
	}
	return string(out)
}

// ===== AUDIT & ACCESS CONTROL =====

type AuditTrail struct {
	entries []AuditEntry
	mu      sync.RWMutex
	maxSize int
}
type AuditEntry struct {
	Timestamp                time.Time
	UserID, Action, Resource string
	Result                   AuditResult
	Details                  map[string]interface{}
	TraceID, SpanID          string
	PIIAccessed              []string
	Severity                 PIISeverity
}
type AuditResult string

const (
	AuditSuccess AuditResult = "SUCCESS"
	AuditFailure AuditResult = "FAILURE"
	AuditDenied  AuditResult = "DENIED"
)

func NewAuditTrail(maxSize int) *AuditTrail {
	return &AuditTrail{entries: make([]AuditEntry, 0, maxSize), maxSize: maxSize}
}
func (at *AuditTrail) RecordAudit(userID, action, resource string, result AuditResult, details map[string]interface{}, traceID, spanID string) {
	at.mu.Lock()
	defer at.mu.Unlock()
	e := AuditEntry{Timestamp: time.Now(), UserID: userID, Action: action, Resource: resource, Result: result, Details: details, TraceID: traceID, SpanID: spanID}
	e.PIIAccessed = at.detectPIIAccess(details)
	if len(e.PIIAccessed) > 0 {
		e.Severity = PIIHigh
	}
	at.entries = append(at.entries, e)
	if len(at.entries) > at.maxSize {
		copy(at.entries, at.entries[1:])
		at.entries = at.entries[:len(at.entries)-1]
	}
}
func (at *AuditTrail) detectPIIAccess(details map[string]interface{}) []string {
	keys := []string{"email", "phone", "id_card", "passport", "ssn", "credit_card", "bank_account", "address", "birth_date"}
	out := []string{}
	for k := range details {
		kl := strings.ToLower(k)
		for _, p := range keys {
			if strings.Contains(kl, p) {
				out = append(out, k)
				break
			}
		}
	}
	return out
}
func (at *AuditTrail) GetAuditTrail(limit int) []AuditEntry {
	at.mu.RLock()
	defer at.mu.RUnlock()
	if limit <= 0 || limit > len(at.entries) {
		limit = len(at.entries)
	}
	start := len(at.entries) - limit
	out := make([]AuditEntry, limit)
	copy(out, at.entries[start:])
	return out
}

type AccessControlManager struct {
	rules map[string]AccessRule
	mu    sync.RWMutex
}
type AccessRule struct {
	AllowedRoles, DeniedRoles []string
	RequiredAuth, LogAccess   bool
}

func NewAccessControlManager() *AccessControlManager {
	return &AccessControlManager{rules: map[string]AccessRule{}}
}
func (acm *AccessControlManager) SetFieldAccess(field string, rule AccessRule) {
	acm.mu.Lock()
	defer acm.mu.Unlock()
	acm.rules[field] = rule
}
func (acm *AccessControlManager) CheckFieldAccess(field, role string, authed bool) bool {
	acm.mu.RLock()
	defer acm.mu.RUnlock()
	r, ok := acm.rules[field]
	if !ok {
		return true
	}
	if r.RequiredAuth && !authed {
		return false
	}
	for _, d := range r.DeniedRoles {
		if d == role {
			return false
		}
	}
	if len(r.AllowedRoles) > 0 {
		for _, a := range r.AllowedRoles {
			if a == role {
				return true
			}
		}
		return false
	}
	return true
}
func (acm *AccessControlManager) FilterFieldsByAccess(fields map[string]interface{}, role string, authed bool, audit *AuditTrail, userID, traceID, spanID string) map[string]interface{} {
	filtered := map[string]interface{}{}
	pii := []string{}
	for k, v := range fields {
		if acm.CheckFieldAccess(k, role, authed) {
			filtered[k] = v
			if acm.isPIIField(k) {
				pii = append(pii, k)
			}
		}
	}
	if len(pii) > 0 && audit != nil {
		audit.RecordAudit(userID, "FIELD_ACCESS", "LOG_FIELDS", AuditSuccess, map[string]interface{}{"accessed_fields": pii, "user_role": role}, traceID, spanID)
	}
	return filtered
}
func (acm *AccessControlManager) isPIIField(name string) bool {
	keys := []string{"email", "phone", "id", "card", "passport", "ssn", "address", "birth", "name", "personal"}
	nl := strings.ToLower(name)
	for _, k := range keys {
		if strings.Contains(nl, k) {
			return true
		}
	}
	return false
}

// ===== SECURITY LOGGER =====

type SecurityLogBrt struct {
	*svc.OTelLogBrt
	piiMasker       *PIIMasker
	accessControl   *AccessControlManager
	auditTrail      *AuditTrail
	securityEnabled bool
}

// NewSecurityLogBrt constructs a security logger with a no-op telemetry provider (core-only path).
// For production, prefer constructing an OTEL provider in adapter and calling NewSecurityLogBrtWithOTel.
func NewSecurityLogBrt(serviceName, version, environment, endpoint string, level domain.Level, sinks ...port.Sink) (*SecurityLogBrt, error) {
	ot := svc.NewOTelLogBrtWithProvider(noopTP{}, level, sinks...)
	return NewSecurityLogBrtWithOTel(ot)
}

// noop TelemetryProvider for core-only construction
type noopTP struct{}

func (noopTP) Tracer() port.Tracer                                                                 { return noopTracer{} }
func (noopTP) Meter() port.Meter                                                                   { return noopMeter{} }
func (noopTP) Propagator() port.Propagator                                                         { return noopProp{} }
func (noopTP) ExtractTraceInfo(ctx context.Context) (string, string)                               { return "", "" }
func (noopTP) RecordLog(ctx context.Context, _ domain.Level, _ time.Duration, _ map[string]string) {}
func (noopTP) Shutdown(ctx context.Context) error                                                  { return nil }

type noopTracer struct{}

func (noopTracer) StartSpan(ctx context.Context, name string, options ...any) (context.Context, any) {
	return ctx, nil
}

type noopMeter struct{}

func (noopMeter) IncCounter(ctx context.Context, name string, labels map[string]string) {}
func (noopMeter) ObserveHistogram(ctx context.Context, name string, value float64, labels map[string]string) {
}

type noopProp struct{}

func (noopProp) Inject(ctx context.Context, carrier any)                  {}
func (noopProp) Extract(ctx context.Context, carrier any) context.Context { return ctx }

// NewSecurityLogBrtWithOTel composes security features over an existing OTel logger
func NewSecurityLogBrtWithOTel(ot *svc.OTelLogBrt) (*SecurityLogBrt, error) {
	if ot == nil {
		return nil, errors.New("nil OTelLogBrt")
	}
	pm := NewPIIMasker()
	ac := NewAccessControlManager()
	at := NewAuditTrail(10000)
	ot.AddMasker(maskingAdapter{pm})
	return &SecurityLogBrt{OTelLogBrt: ot, piiMasker: pm, accessControl: ac, auditTrail: at, securityEnabled: true}, nil
}

func (el *SecurityLogBrt) LogWithSecurity(level domain.Level, msg, userID, userRole string, isAuthenticated bool) {
	if !el.securityEnabled {
		el.OTelLogBrt.Infof("%s", msg)
		return
	}
	traceID, spanID := "", ""
	if ctx := el.OTelLogBrt.Context(); ctx != nil && el.OTelLogBrt.GetTelemetryProvider() != nil {
		traceID, spanID = el.OTelLogBrt.GetTelemetryProvider().ExtractTraceInfo(ctx)
	}
	original := el.OTelLogBrt.FieldsCopy()
	filtered := el.accessControl.FilterFieldsByAccess(original, userRole, isAuthenticated, el.auditTrail, userID, traceID, spanID)
	sec := el.OTelLogBrt.WithOnlyFields(filtered)
	// use masker via strategies
	sec.Infof("%s", msg)
}

// maskingAdapter adapts PIIMasker to MaskingStrategy with Configure
type maskingAdapter struct{ *PIIMasker }

func (m maskingAdapter) MaskFields(f map[string]interface{}) map[string]interface{} {
	return m.PIIMasker.MaskFields(f)
}
func (m maskingAdapter) Name() string { return "pii_masker" }
func (m maskingAdapter) Configure(cfg map[string]interface{}) error {
	if v, ok := cfg["enabled"].(bool); ok {
		m.enabled = v
	}
	return nil
}

// Exported strategy constructor compatible with service surface
func NewPIIMaskerStrategy() svc.MaskingStrategy { return maskingAdapter{NewPIIMasker()} }
