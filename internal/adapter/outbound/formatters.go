package outbound

import (
	"encoding/json"
	"fmt"
	"obs-brutal/internal/core/domain"
	"obs-brutal/internal/core/port/outbound"
	"strings"
	"time"
)

// scrub sensitive values by key
func scrubFields(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		if isSensitiveKey(k) {
			out[k] = "***"
			continue
		}
		out[k] = v
	}
	return out
}

func isSensitiveKey(k string) bool {
	ks := strings.ToLower(k)
	switch ks {
	case "password", "passwd", "authorization", "authorization_header", "auth", "token", "access_token", "refresh_token", "api_key", "apikey", "secret", "client_secret":
		return true
	default:
		return false
	}
}

// JSONFormatter formats logs as JSON
type JSONFormatter struct {
	prettyPrint bool
	timeFormat  string
}

// NewJSONFormatter creates a new JSON formatter
func NewJSONFormatter() outbound.Formatter {
	return &JSONFormatter{
		prettyPrint: false,
		timeFormat:  time.RFC3339,
	}
}

// NewPrettyJSONFormatter creates a JSON formatter with pretty printing
func NewPrettyJSONFormatter() outbound.Formatter {
	return &JSONFormatter{
		prettyPrint: true,
		timeFormat:  time.RFC3339,
	}
}

func (f *JSONFormatter) Format(entry *domain.LogEntry) string {
	if entry == nil {
		return "{}"
	}

	// Build JSON object
	obj := make(map[string]interface{})

	// Standard fields
	obj["timestamp"] = entry.Timestamp.Format(f.timeFormat)
	obj["level"] = entry.Level.String()
	obj["message"] = entry.Message

	// Add correlation IDs
	if entry.TraceID != "" {
		obj["trace_id"] = entry.TraceID
	}
	if entry.SpanID != "" {
		obj["span_id"] = entry.SpanID
	}
	if entry.RequestID != "" {
		obj["request_id"] = entry.RequestID
	}
	if entry.UserID != "" {
		obj["user_id"] = entry.UserID
	}
	if entry.TenantID != "" {
		obj["tenant_id"] = entry.TenantID
	}
	if entry.Module != "" {
		obj["module"] = entry.Module
	}

	// Add custom fields (scrubbed)
	for k, v := range scrubFields(entry.Fields) {
		// Skip if already added as standard field
		if _, exists := obj[k]; !exists {
			obj[k] = v
		}
	}

	// Add error if present
	if entry.Error != nil {
		obj["error"] = entry.Error.Error()
		obj["error_type"] = fmt.Sprintf("%T", entry.Error)
	}

	// Marshal to JSON
	var data []byte
	var err error

	if f.prettyPrint {
		data, err = json.MarshalIndent(obj, "", "  ")
	} else {
		data, err = json.Marshal(obj)
	}

	if err != nil {
		// Fallback to simple format
		return fmt.Sprintf(`{"level":"%s","message":"%s","error":"failed to marshal: %v"}`,
			entry.Level.String(), entry.Message, err)
	}

	return string(data)
}

func (f *JSONFormatter) Name() string { return "json" }
func (f *JSONFormatter) Configure(config map[string]interface{}) error {
	if pretty, ok := config["pretty"].(bool); ok {
		f.prettyPrint = pretty
	}
	if format, ok := config["time_format"].(string); ok {
		f.timeFormat = format
	}
	return nil
}

// TextFormatter formats logs as human-readable text
type TextFormatter struct {
	template   string
	timeFormat string
	colorize   bool
}

// NewTextFormatter creates a new text formatter
func NewTextFormatter() outbound.Formatter {
	return &TextFormatter{template: "[{timestamp}] {level} {message}", timeFormat: "2006-01-02 15:04:05", colorize: false}
}

// NewColorTextFormatter creates a text formatter with colors
func NewColorTextFormatter() outbound.Formatter {
	return &TextFormatter{template: "[{timestamp}] {level} {message}", timeFormat: "2006-01-02 15:04:05", colorize: true}
}

func (f *TextFormatter) Format(entry *domain.LogEntry) string {
	if entry == nil {
		return ""
	}
	output := f.template
	output = strings.ReplaceAll(output, "{timestamp}", entry.Timestamp.Format(f.timeFormat))
	output = strings.ReplaceAll(output, "{message}", entry.Message)
	level := entry.Level.String()
	if f.colorize {
		level = f.colorizeLevel(entry.Level, level)
	}
	output = strings.ReplaceAll(output, "{level}", level)
	if len(entry.Fields) > 0 {
		var fields []string
		for k, v := range scrubFields(entry.Fields) {
			fields = append(fields, fmt.Sprintf("%s=%v", k, v))
		}
		output += " {" + strings.Join(fields, ", ") + "}"
	}
	if entry.Error != nil {
		errStr := fmt.Sprintf(" error=%v", entry.Error)
		if f.colorize {
			errStr = "\u001b[31m" + errStr + "\u001b[0m"
		}
		output += errStr
	}
	return output
}

func (f *TextFormatter) colorizeLevel(level domain.Level, text string) string {
	switch level {
	case domain.DebugLevel:
		return "\u001b[36m" + text + "\u001b[0m"
	case domain.InfoLevel:
		return "\u001b[32m" + text + "\u001b[0m"
	case domain.WarnLevel:
		return "\u001b[33m" + text + "\u001b[0m"
	case domain.ErrorLevel:
		return "\u001b[31m" + text + "\u001b[0m"
	case domain.FatalLevel:
		return "\u001b[35m" + text + "\u001b[0m"
	default:
		return text
	}
}

func (f *TextFormatter) Name() string { return "text" }
func (f *TextFormatter) Configure(config map[string]interface{}) error {
	if template, ok := config["template"].(string); ok {
		f.template = template
	}
	if format, ok := config["time_format"].(string); ok {
		f.timeFormat = format
	}
	if colorize, ok := config["colorize"].(bool); ok {
		f.colorize = colorize
	}
	return nil
}

// LogfmtFormatter formats logs in logfmt format
type LogfmtFormatter struct{ timeFormat string }

// NewLogfmtFormatter creates a new logfmt formatter
func NewLogfmtFormatter() outbound.Formatter {
	return &LogfmtFormatter{timeFormat: "2006-01-02T15:04:05.000Z07:00"}
}

func (f *LogfmtFormatter) Format(entry *domain.LogEntry) string {
	if entry == nil {
		return ""
	}
	var pairs []string
	pairs = append(pairs, fmt.Sprintf("timestamp=%s", entry.Timestamp.Format(f.timeFormat)), fmt.Sprintf("level=%s", strings.ToLower(entry.Level.String())), fmt.Sprintf("msg=%q", entry.Message))
	if entry.TraceID != "" {
		pairs = append(pairs, fmt.Sprintf("trace_id=%s", entry.TraceID))
	}
	if entry.SpanID != "" {
		pairs = append(pairs, fmt.Sprintf("span_id=%s", entry.SpanID))
	}
	if entry.RequestID != "" {
		pairs = append(pairs, fmt.Sprintf("request_id=%s", entry.RequestID))
	}
	if entry.UserID != "" {
		pairs = append(pairs, fmt.Sprintf("user_id=%s", entry.UserID))
	}
	if entry.TenantID != "" {
		pairs = append(pairs, fmt.Sprintf("tenant_id=%s", entry.TenantID))
	}
	if entry.Module != "" {
		pairs = append(pairs, fmt.Sprintf("module=%s", entry.Module))
	}
	for k, v := range scrubFields(entry.Fields) {
		pairs = append(pairs, fmt.Sprintf("%s=%v", k, formatValue(v)))
	}
	if entry.Error != nil {
		pairs = append(pairs, fmt.Sprintf("error=%q", entry.Error.Error()))
	}
	return strings.Join(pairs, " ")
}

func (f *LogfmtFormatter) Name() string { return "logfmt" }
func (f *LogfmtFormatter) Configure(config map[string]interface{}) error {
	if format, ok := config["time_format"].(string); ok {
		f.timeFormat = format
	}
	return nil
}

func formatValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		if strings.ContainsAny(val, " \t\n\r\"") {
			return fmt.Sprintf("%q", val)
		}
		return val
	case error:
		return fmt.Sprintf("%q", val.Error())
	default:
		return fmt.Sprintf("%v", val)
	}
}

// CEFFormatter formats logs in Common Event Format
type CEFFormatter struct {
	deviceVendor  string
	deviceProduct string
	deviceVersion string
}

// NewCEFFormatter creates a new CEF formatter
func NewCEFFormatter(vendor, product, version string) outbound.Formatter {
	return &CEFFormatter{deviceVendor: vendor, deviceProduct: product, deviceVersion: version}
}

func (f *CEFFormatter) Format(entry *domain.LogEntry) string {
	if entry == nil {
		return ""
	}
	severity := f.mapSeverity(entry.Level)
	var extensions []string
	extensions = append(extensions, fmt.Sprintf("rt=%d", entry.Timestamp.UnixMilli()))
	extensions = append(extensions, fmt.Sprintf("msg=%s", escapeExtension(entry.Message)))
	for k, v := range entry.Fields {
		extensions = append(extensions, fmt.Sprintf("%s=%s", k, escapeExtension(fmt.Sprintf("%v", v))))
	}
	if entry.Error != nil {
		extensions = append(extensions, fmt.Sprintf("reason=%s", escapeExtension(entry.Error.Error())))
	}
	return fmt.Sprintf("CEF:0|%s|%s|%s|%s|%s|%d|%s", f.deviceVendor, f.deviceProduct, f.deviceVersion, "LOG", entry.Level.String(), severity, strings.Join(extensions, " "))
}

func (f *CEFFormatter) mapSeverity(level domain.Level) int {
	switch level {
	case domain.DebugLevel:
		return 1
	case domain.InfoLevel:
		return 3
	case domain.WarnLevel:
		return 5
	case domain.ErrorLevel:
		return 7
	case domain.FatalLevel:
		return 10
	default:
		return 0
	}
}

func escapeExtension(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "=", "\\=")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	return s
}
func (f *CEFFormatter) Name() string { return "cef" }
func (f *CEFFormatter) Configure(config map[string]interface{}) error {
	if vendor, ok := config["vendor"].(string); ok {
		f.deviceVendor = vendor
	}
	if product, ok := config["product"].(string); ok {
		f.deviceProduct = product
	}
	if version, ok := config["version"].(string); ok {
		f.deviceVersion = version
	}
	return nil
}
