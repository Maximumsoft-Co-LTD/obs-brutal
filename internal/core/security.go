// Package core provides enterprise-grade security features
package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"obs-brutal/internal/core/domain"
)

// ===== PII DETECTION & MASKING =====

// PIIMasker provides enterprise-grade PII detection and masking
type PIIMasker struct {
	patterns    map[string]*MaskingPattern
	fieldRules  map[string]MaskingRule
	globalRules []GlobalMaskingRule
	mu          sync.RWMutex
	enabled     bool
}

// MaskingPattern defines regex pattern for PII detection
type MaskingPattern struct {
	Name        string
	Pattern     *regexp.Regexp
	Replacement string
	Severity    PIISeverity
}

// MaskingRule defines field-specific masking behavior
type MaskingRule struct {
	Action   MaskingAction
	Pattern  string
	Severity PIISeverity
}

// GlobalMaskingRule applies to all fields matching criteria
type GlobalMaskingRule struct {
	FieldPattern *regexp.Regexp // Field name pattern
	ValuePattern *regexp.Regexp // Field value pattern
	Action       MaskingAction
	Severity     PIISeverity
}

// PIISeverity levels for compliance
type PIISeverity int

const (
	PIILow PIISeverity = iota
	PIIMedium
	PIIHigh
	PIICritical
)

// MaskingAction defines what to do with PII
type MaskingAction int

const (
	MaskPartial MaskingAction = iota
	MaskFull
	MaskHash
	MaskRemove
	MaskEncrypt
)

// NewPIIMasker creates enterprise PII masker
func NewPIIMasker() *PIIMasker {
	masker := &PIIMasker{
		patterns:    make(map[string]*MaskingPattern),
		fieldRules:  make(map[string]MaskingRule),
		globalRules: make([]GlobalMaskingRule, 0),
		enabled:     true,
	}

	// Add default enterprise patterns
	masker.addEnterprisePatterns()

	return masker
}

// addEnterprisePatterns adds comprehensive PII patterns
func (pm *PIIMasker) addEnterprisePatterns() {
	patterns := map[string]MaskingPattern{
		// Thai specific
		"thai_id": {
			Name:        "Thai National ID",
			Pattern:     regexp.MustCompile(`\b\d{1}-\d{4}-\d{5}-\d{2}-\d{1}\b`),
			Replacement: "x-xxxx-xxxxx-xx-x",
			Severity:    PIIHigh,
		},
		"thai_id_compact": {
			Name:        "Thai National ID (compact)",
			Pattern:     regexp.MustCompile(`\b\d{13}\b`),
			Replacement: "xxxxxxxxxxxxx",
			Severity:    PIIHigh,
		},
		"thai_phone": {
			Name:        "Thai Phone Number",
			Pattern:     regexp.MustCompile(`(?:\+66|0)[\s-]?\d{2}[\s-]?\d{3}[\s-]?\d{4}\b`),
			Replacement: "xxx-xxx-xxxx",
			Severity:    PIIMedium,
		},

		// International
		"email": {
			Name:        "Email Address",
			Pattern:     regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`),
			Replacement: "***@***.***",
			Severity:    PIIMedium,
		},
		"credit_card": {
			Name:        "Credit Card",
			Pattern:     regexp.MustCompile(`\b(?:\d{4}[-\s]?){3}\d{4}\b`),
			Replacement: "****-****-****-****",
			Severity:    PIIHigh,
		},
		"ssn": {
			Name:        "Social Security Number",
			Pattern:     regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
			Replacement: "***-**-****",
			Severity:    PIIHigh,
		},
		"passport": {
			Name:        "Passport Number",
			Pattern:     regexp.MustCompile(`\b[A-Z]{1,2}\d{6,9}\b`),
			Replacement: "XX######",
			Severity:    PIIHigh,
		},
		"api_key": {
			Name:        "API Key",
			Pattern:     regexp.MustCompile(`\b[A-Za-z0-9]{32,}\b`),
			Replacement: "********************************",
			Severity:    PIICritical,
		},
		"jwt_token": {
			Name:        "JWT Token",
			Pattern:     regexp.MustCompile(`eyJ[A-Za-z0-9-_]+\.[A-Za-z0-9-_]+\.[A-Za-z0-9-_]*`),
			Replacement: "eyJ***.***.***",
			Severity:    PIICritical,
		},
	}

	for name, pattern := range patterns {
		pm.patterns[name] = &pattern
	}

	// Add field-specific rules
	pm.fieldRules["password"] = MaskingRule{Action: MaskFull, Severity: PIICritical}
	pm.fieldRules["secret"] = MaskingRule{Action: MaskFull, Severity: PIICritical}
	pm.fieldRules["private_key"] = MaskingRule{Action: MaskRemove, Severity: PIICritical}
	pm.fieldRules["access_token"] = MaskingRule{Action: MaskHash, Severity: PIIHigh}
	pm.fieldRules["refresh_token"] = MaskingRule{Action: MaskHash, Severity: PIIHigh}
}

// MaskFields applies enterprise PII masking to fields
func (pm *PIIMasker) MaskFields(fields map[string]interface{}) map[string]interface{} {
	if !pm.enabled {
		return fields
	}

	pm.mu.RLock()
	defer pm.mu.RUnlock()

	// Go 1.25: Efficient map operations
	masked := make(map[string]interface{}, len(fields))

	for k, v := range fields {
		masked[k] = pm.maskValue(k, v)
	}

	return masked
}

// maskValue applies appropriate masking to a single value
func (pm *PIIMasker) maskValue(fieldName string, value interface{}) interface{} {
	// Check field-specific rules first
	if rule, exists := pm.fieldRules[strings.ToLower(fieldName)]; exists {
		return pm.applyMaskingAction(value, rule.Action)
	}

	// Check global rules
	for _, rule := range pm.globalRules {
		if rule.FieldPattern.MatchString(fieldName) {
			if str, ok := value.(string); ok {
				if rule.ValuePattern.MatchString(str) {
					return pm.applyMaskingAction(value, rule.Action)
				}
			}
		}
	}

	// Apply pattern-based masking
	if str, ok := value.(string); ok {
		return pm.maskStringPatterns(str)
	}

	// Check struct tags if it's a struct (supports `pii` and `log`)
	if pm.isStructWithTags(value) {
		return pm.maskStructFields(value)
	}

	return value
}

// applyMaskingAction applies specific masking action
func (pm *PIIMasker) applyMaskingAction(value interface{}, action MaskingAction) interface{} {
	str, ok := value.(string)
	if !ok {
		return value
	}

	switch action {
	case MaskPartial:
		return pm.maskPartially(str)
	case MaskFull:
		return strings.Repeat("*", len(str))
	case MaskHash:
		return pm.hashValue(str)
	case MaskRemove:
		return "[REMOVED]"
	case MaskEncrypt:
		return pm.encryptValue(str)
	default:
		return value
	}
}

// maskPartially masks part of the string
func (pm *PIIMasker) maskPartially(str string) string {
	length := len(str)
	if length <= 2 {
		return strings.Repeat("*", length)
	}

	// Show first and last characters
	if length <= 6 {
		return string(str[0]) + strings.Repeat("*", length-2) + string(str[length-1])
	}

	// Show first 2 and last 2 characters
	return str[:2] + strings.Repeat("*", length-4) + str[length-2:]
}

// hashValue creates SHA256 hash for audit trail
func (pm *PIIMasker) hashValue(str string) string {
	hash := sha256.Sum256([]byte(str))
	return "[HASH:" + hex.EncodeToString(hash[:8]) + "]" // First 8 bytes for brevity
}

// encryptValue encrypts sensitive data (simplified)
func (pm *PIIMasker) encryptValue(str string) string {
	// In production, use proper encryption
	return "[ENCRYPTED:" + pm.hashValue(str)[7:15] + "]"
}

// maskStringPatterns applies all pattern-based masking
func (pm *PIIMasker) maskStringPatterns(str string) string {
	result := str

	for _, pattern := range pm.patterns {
		if pattern.Pattern.MatchString(result) {
			result = pattern.Pattern.ReplaceAllString(result, pattern.Replacement)
		}
	}

	return result
}

// isStructWithTags checks if value is a struct with log tags
func (pm *PIIMasker) isStructWithTags(value interface{}) bool {
	if value == nil {
		return false
	}

	rt := reflect.TypeOf(value)
	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}

	return rt.Kind() == reflect.Struct
}

// maskStructFields extracts and masks struct fields based on tags
func (pm *PIIMasker) maskStructFields(value interface{}) interface{} {
	rv := reflect.ValueOf(value)
	rt := reflect.TypeOf(value)

	if rt.Kind() == reflect.Ptr {
		rv = rv.Elem()
		rt = rt.Elem()
	}

	if rt.Kind() != reflect.Struct {
		return value
	}

	// Extract fields with log tags
	fields := make(map[string]interface{})

	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		fieldValue := rv.Field(i)

		// Check pii tag (email/phone/password/...) first
		if pii := field.Tag.Get("pii"); pii != "" {
			fieldName := field.Name
			if jsonTag := field.Tag.Get("json"); jsonTag != "" {
				fieldName = strings.Split(jsonTag, ",")[0]
			}
			masked := pm.applyPIITag(pii, fieldValue.Interface())
			fields[fieldName] = masked
			continue
		}

		// Check log tag
		if logTag := field.Tag.Get("log"); logTag != "" {
			if logTag == "-" {
				continue // Skip this field
			}

			fieldName := field.Name
			if jsonTag := field.Tag.Get("json"); jsonTag != "" {
				fieldName = strings.Split(jsonTag, ",")[0]
			}

			// Parse log tag options
			options := strings.Split(logTag, ",")
			baseFieldName := options[0]
			if baseFieldName == "" {
				baseFieldName = fieldName
			}

			// Apply tag-based masking
			maskedValue := pm.applyTagMasking(fieldValue.Interface(), options[1:])
			fields[baseFieldName] = maskedValue
		}
	}

	return fields
}

// applyPIITag applies masking per pii tag type
func (pm *PIIMasker) applyPIITag(pii string, val interface{}) interface{} {
	str, ok := val.(string)
	if !ok {
		return val
	}
	switch strings.ToLower(pii) {
	case "email":
		return pm.maskStringPatterns(str)
	case "phone":
		return pm.maskStringPatterns(str)
	case "password":
		return strings.Repeat("*", len(str))
	default:
		return pm.maskPartially(str)
	}
}

// applyTagMasking applies masking based on struct tag options
func (pm *PIIMasker) applyTagMasking(value interface{}, options []string) interface{} {
	for _, option := range options {
		switch {
		case option == "sensitive=true":
			return pm.applyMaskingAction(value, MaskFull)
		case strings.HasPrefix(option, "mask="):
			pattern := strings.TrimPrefix(option, "mask=")
			return pm.applyCustomMask(value, pattern)
		case option == "hash=true":
			return pm.applyMaskingAction(value, MaskHash)
		case option == "encrypt=true":
			return pm.applyMaskingAction(value, MaskEncrypt)
		}
	}

	return value
}

// applyCustomMask applies custom masking pattern
func (pm *PIIMasker) applyCustomMask(value interface{}, pattern string) interface{} {
	str, ok := value.(string)
	if !ok {
		return value
	}

	// Parse pattern (e.g., "****-****-****-xxxx" where x = keep, * = mask)
	if len(pattern) == 0 {
		return pm.maskPartially(str)
	}

	result := make([]byte, len(str))
	patternLen := len(pattern)

	for i, char := range []byte(str) {
		patternIndex := i % patternLen

		if patternIndex < len(pattern) {
			switch pattern[patternIndex] {
			case 'x', 'X':
				result[i] = char // Keep original
			case '*':
				result[i] = '*' // Mask
			default:
				result[i] = pattern[patternIndex] // Use pattern character
			}
		} else {
			result[i] = '*' // Default mask
		}
	}

	return string(result)
}

// ===== AUDIT TRAIL =====

// AuditTrail provides enterprise audit logging
type AuditTrail struct {
	entries []AuditEntry
	mu      sync.RWMutex
	maxSize int
}

// AuditEntry represents a single audit event
type AuditEntry struct {
	Timestamp   time.Time              `json:"timestamp"`
	UserID      string                 `json:"user_id"`
	Action      string                 `json:"action"`
	Resource    string                 `json:"resource"`
	Result      AuditResult            `json:"result"`
	Details     map[string]interface{} `json:"details"`
	TraceID     string                 `json:"trace_id"`
	SpanID      string                 `json:"span_id"`
	PIIAccessed []string               `json:"pii_accessed,omitempty"`
	Severity    PIISeverity            `json:"severity"`
}

// AuditResult represents audit event result
type AuditResult string

const (
	AuditSuccess AuditResult = "SUCCESS"
	AuditFailure AuditResult = "FAILURE"
	AuditDenied  AuditResult = "DENIED"
)

func NewAuditTrail(maxSize int) *AuditTrail {
	return &AuditTrail{
		entries: make([]AuditEntry, 0, maxSize),
		maxSize: maxSize,
	}
}

// RecordAudit records an audit event
func (at *AuditTrail) RecordAudit(userID, action, resource string, result AuditResult,
	details map[string]interface{}, traceID, spanID string) {

	at.mu.Lock()
	defer at.mu.Unlock()

	entry := AuditEntry{
		Timestamp: time.Now(),
		UserID:    userID,
		Action:    action,
		Resource:  resource,
		Result:    result,
		Details:   details,
		TraceID:   traceID,
		SpanID:    spanID,
	}

	// Detect PII access
	entry.PIIAccessed = at.detectPIIAccess(details)
	if len(entry.PIIAccessed) > 0 {
		entry.Severity = PIIHigh
	}

	// Add to trail
	at.entries = append(at.entries, entry)

	// Maintain size limit
	if len(at.entries) > at.maxSize {
		// Go 1.25: Efficient slice operations
		copy(at.entries, at.entries[1:])
		at.entries = at.entries[:len(at.entries)-1]
	}
}

// detectPIIAccess detects if PII data was accessed
func (at *AuditTrail) detectPIIAccess(details map[string]interface{}) []string {
	piiFields := []string{
		"email", "phone", "id_card", "passport", "ssn",
		"credit_card", "bank_account", "address", "birth_date",
	}

	var accessed []string
	for field := range details {
		fieldLower := strings.ToLower(field)
		for _, piiField := range piiFields {
			if strings.Contains(fieldLower, piiField) {
				accessed = append(accessed, field)
				break
			}
		}
	}

	return accessed
}

// GetAuditTrail returns recent audit entries
func (at *AuditTrail) GetAuditTrail(limit int) []AuditEntry {
	at.mu.RLock()
	defer at.mu.RUnlock()

	if limit <= 0 || limit > len(at.entries) {
		limit = len(at.entries)
	}

	// Return last N entries
	start := len(at.entries) - limit
	result := make([]AuditEntry, limit)
	copy(result, at.entries[start:])

	return result
}

// ===== ACCESS CONTROL =====

// AccessControlManager manages field-level access control
type AccessControlManager struct {
	rules map[string]AccessRule
	mu    sync.RWMutex
}

// AccessRule defines access control for fields
type AccessRule struct {
	AllowedRoles []string
	DeniedRoles  []string
	RequiredAuth bool
	LogAccess    bool
}

func NewAccessControlManager() *AccessControlManager {
	return &AccessControlManager{
		rules: make(map[string]AccessRule),
	}
}

// SetFieldAccess sets access control for a field
func (acm *AccessControlManager) SetFieldAccess(field string, rule AccessRule) {
	acm.mu.Lock()
	defer acm.mu.Unlock()
	acm.rules[field] = rule
}

// CheckFieldAccess checks if user can access field
func (acm *AccessControlManager) CheckFieldAccess(field, userRole string, isAuthenticated bool) bool {
	acm.mu.RLock()
	defer acm.mu.RUnlock()

	rule, exists := acm.rules[field]
	if !exists {
		return true // Allow by default
	}

	// Check authentication requirement
	if rule.RequiredAuth && !isAuthenticated {
		return false
	}

	// Check denied roles
	for _, deniedRole := range rule.DeniedRoles {
		if deniedRole == userRole {
			return false
		}
	}

	// Check allowed roles
	if len(rule.AllowedRoles) > 0 {
		for _, allowedRole := range rule.AllowedRoles {
			if allowedRole == userRole {
				return true
			}
		}
		return false // Not in allowed list
	}

	return true // Allow by default
}

// FilterFieldsByAccess filters fields based on access control
func (acm *AccessControlManager) FilterFieldsByAccess(fields map[string]interface{},
	userRole string, isAuthenticated bool, auditTrail *AuditTrail,
	userID, traceID, spanID string) map[string]interface{} {

	filtered := make(map[string]interface{})
	accessedPII := make([]string, 0)

	for k, v := range fields {
		if acm.CheckFieldAccess(k, userRole, isAuthenticated) {
			filtered[k] = v

			// Track PII access for audit
			if acm.isPIIField(k) {
				accessedPII = append(accessedPII, k)
			}
		}
	}

	// Record audit event if PII was accessed
	if len(accessedPII) > 0 && auditTrail != nil {
		auditTrail.RecordAudit(
			userID,
			"FIELD_ACCESS",
			"LOG_FIELDS",
			AuditSuccess,
			map[string]interface{}{
				"accessed_fields": accessedPII,
				"user_role":       userRole,
			},
			traceID,
			spanID,
		)
	}

	return filtered
}

// isPIIField checks if field contains PII
func (acm *AccessControlManager) isPIIField(fieldName string) bool {
	piiKeywords := []string{
		"email", "phone", "id", "card", "passport", "ssn",
		"address", "birth", "name", "personal",
	}

	fieldLower := strings.ToLower(fieldName)
	for _, keyword := range piiKeywords {
		if strings.Contains(fieldLower, keyword) {
			return true
		}
	}

	return false
}

// ===== SECURITY LogBrt =====

// SecurityLogBrt combines enterprise security features on top of OTEL LogBrt
type SecurityLogBrt struct {
	*OTelLogBrt
	piiMasker       *PIIMasker
	accessControl   *AccessControlManager
	auditTrail      *AuditTrail
	securityEnabled bool
}

// NewSecurityLogBrt creates LogBrt with full enterprise security
func NewSecurityLogBrt(serviceName, version, environment, endpoint string,
	level Level, sinks ...Sink) (*SecurityLogBrt, error) {

	// Create OTEL LogBrt
	otelLogBrt, err := NewOTelLogBrt(serviceName, version, environment, endpoint, level, sinks...)
	if err != nil {
		return nil, err
	}

	// Add enterprise security components
	piiMasker := NewPIIMasker()
	accessControl := NewAccessControlManager()
	auditTrail := NewAuditTrail(10000)

	// Add enterprise masking strategy
	otelLogBrt.AddMasker(piiMasker)

	return &SecurityLogBrt{
		OTelLogBrt:      otelLogBrt,
		piiMasker:       piiMasker,
		accessControl:   accessControl,
		auditTrail:      auditTrail,
		securityEnabled: true,
	}, nil
}

// LogWithSecurity logs with full enterprise security checks
func (el *SecurityLogBrt) LogWithSecurity(level domain.Level, msg string,
	userID, userRole string, isAuthenticated bool) {

	if !el.securityEnabled {
		el.log(level, msg)
		return
	}

	// Extract trace info for audit
	traceID := ""
	spanID := ""
	if ctx, ok := el.fields["context"].(context.Context); ok {
		traceID, spanID = el.otelProvider.ExtractTraceInfo(ctx)
	}

	// Apply access control to fields
	filteredFields := el.accessControl.FilterFieldsByAccess(
		el.fields, userRole, isAuthenticated,
		el.auditTrail, userID, traceID, spanID,
	)

	// Create LogBrt with filtered fields
	secureLogBrt := &SecurityLogBrt{
		OTelLogBrt:      el.OTelLogBrt,
		piiMasker:       el.piiMasker,
		accessControl:   el.accessControl,
		auditTrail:      el.auditTrail,
		securityEnabled: el.securityEnabled,
	}

	// Update fields on a cloned instance to avoid mutating the original logger state
	secureLogBrt = secureLogBrt.clone()
	for k, v := range filteredFields {
		secureLogBrt.fields[k] = v
	}

	// Log with security
	secureLogBrt.log(level, msg)
}

// GetAuditTrail returns audit trail for compliance
func (el *SecurityLogBrt) GetAuditTrail(limit int) []AuditEntry {
	return el.auditTrail.GetAuditTrail(limit)
}

// EnableSecurity enables/disables security features
func (el *SecurityLogBrt) EnableSecurity(enabled bool) {
	el.securityEnabled = enabled
}

// Backward compatibility alias
type EnterpriseLogBrt = SecurityLogBrt

func NewEnterpriseLogBrt(serviceName, version, environment, endpoint string,
	level Level, sinks ...Sink) (*EnterpriseLogBrt, error) {
	sl, err := NewSecurityLogBrt(serviceName, version, environment, endpoint, level, sinks...)
	return (*EnterpriseLogBrt)(sl), err
}

func (pm *PIIMasker) Name() string { return "enterprise_pii_masker" }

func (pm *PIIMasker) Configure(config map[string]interface{}) error {
	if enabled, ok := config["enabled"].(bool); ok {
		pm.enabled = enabled
	}
	return nil
}

// ===== DERIVED BUILDER OVERRIDES: preserve SecurityLogBrt type =====

// clone creates SecurityLogBrt preserving security components
func (el *SecurityLogBrt) clone() *SecurityLogBrt {
	base := el.OTelLogBrt.clone()
	return &SecurityLogBrt{
		OTelLogBrt:      base,
		piiMasker:       el.piiMasker,
		accessControl:   el.accessControl,
		auditTrail:      el.auditTrail,
		securityEnabled: el.securityEnabled,
	}
}

func (el *SecurityLogBrt) F(key string, value interface{}) LogBrt {
	clone := el.clone()
	clone.fields[key] = value
	return clone
}

func (el *SecurityLogBrt) Fs(fields map[string]interface{}) LogBrt {
	if len(fields) == 0 {
		return el
	}
	clone := el.clone()
	for k, v := range fields {
		clone.fields[k] = v
	}
	return clone
}

func (el *SecurityLogBrt) Ctx(ctx context.Context) LogBrt {
	if ctx == nil {
		return el
	}
	clone := el.clone()
	clone.fields["context"] = ctx
	if traceID := extractFromContext(ctx, "trace_id"); traceID != "" {
		clone.fields["trace_id"] = traceID
	}
	if userID := extractFromContext(ctx, "user_id"); userID != "" {
		clone.fields["user_id"] = userID
	}
	if requestID := extractFromContext(ctx, "request_id"); requestID != "" {
		clone.fields["request_id"] = requestID
	}
	return clone
}

func (el *SecurityLogBrt) TraceID(id string) LogBrt   { return el.F("trace_id", id) }
func (el *SecurityLogBrt) UserID(id string) LogBrt    { return el.F("user_id", id) }
func (el *SecurityLogBrt) RequestID(id string) LogBrt { return el.F("request_id", id) }

func (el *SecurityLogBrt) WithError(err error) LogBrt {
	if err == nil {
		return el
	}
	return el.F("error", err.Error())
}
