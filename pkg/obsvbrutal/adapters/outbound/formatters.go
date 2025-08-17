package outbound

import (
	"encoding/json"
	"fmt"
	"obs-brutal/pkg/obsvbrutal/core/domain"
	"obs-brutal/pkg/obsvbrutal/core/ports/outbound"
	"strings"
	"time"
)

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

	// Add custom fields
	for k, v := range entry.Fields {
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

func (f *JSONFormatter) Name() string {
	return "json"
}

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
	return &TextFormatter{
		template:   "[{timestamp}] {level} {message}",
		timeFormat: "2006-01-02 15:04:05",
		colorize:   false,
	}
}

// NewColorTextFormatter creates a text formatter with colors
func NewColorTextFormatter() outbound.Formatter {
	return &TextFormatter{
		template:   "[{timestamp}] {level} {message}",
		timeFormat: "2006-01-02 15:04:05",
		colorize:   true,
	}
}

func (f *TextFormatter) Format(entry *domain.LogEntry) string {
	if entry == nil {
		return ""
	}

	// Start with template
	output := f.template

	// Replace placeholders
	output = strings.ReplaceAll(output, "{timestamp}", entry.Timestamp.Format(f.timeFormat))
	output = strings.ReplaceAll(output, "{message}", entry.Message)

	// Format level with color if enabled
	level := entry.Level.String()
	if f.colorize {
		level = f.colorizeLevel(entry.Level, level)
	}
	output = strings.ReplaceAll(output, "{level}", level)

	// Add fields
	if len(entry.Fields) > 0 {
		var fields []string
		for k, v := range entry.Fields {
			fields = append(fields, fmt.Sprintf("%s=%v", k, v))
		}
		output += " {" + strings.Join(fields, ", ") + "}"
	}

	// Add error
	if entry.Error != nil {
		errStr := fmt.Sprintf(" error=%v", entry.Error)
		if f.colorize {
			errStr = "\033[31m" + errStr + "\033[0m" // Red color
		}
		output += errStr
	}

	return output
}

func (f *TextFormatter) colorizeLevel(level domain.Level, text string) string {
	switch level {
	case domain.DebugLevel:
		return "\033[36m" + text + "\033[0m" // Cyan
	case domain.InfoLevel:
		return "\033[32m" + text + "\033[0m" // Green
	case domain.WarnLevel:
		return "\033[33m" + text + "\033[0m" // Yellow
	case domain.ErrorLevel:
		return "\033[31m" + text + "\033[0m" // Red
	case domain.FatalLevel:
		return "\033[35m" + text + "\033[0m" // Magenta
	default:
		return text
	}
}

func (f *TextFormatter) Name() string {
	return "text"
}

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
type LogfmtFormatter struct {
	timeFormat string
}

// NewLogfmtFormatter creates a new logfmt formatter
func NewLogfmtFormatter() outbound.Formatter {
	return &LogfmtFormatter{
		timeFormat: "2006-01-02T15:04:05.000Z07:00",
	}
}

func (f *LogfmtFormatter) Format(entry *domain.LogEntry) string {
	if entry == nil {
		return ""
	}

	var pairs []string

	// Add standard fields
	pairs = append(pairs,
		fmt.Sprintf("timestamp=%s", entry.Timestamp.Format(f.timeFormat)),
		fmt.Sprintf("level=%s", strings.ToLower(entry.Level.String())),
		fmt.Sprintf("msg=%q", entry.Message),
	)

	// Add correlation IDs
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

	// Add custom fields
	for k, v := range entry.Fields {
		pairs = append(pairs, fmt.Sprintf("%s=%v", k, formatValue(v)))
	}

	// Add error
	if entry.Error != nil {
		pairs = append(pairs, fmt.Sprintf("error=%q", entry.Error.Error()))
	}

	return strings.Join(pairs, " ")
}

func (f *LogfmtFormatter) Name() string {
	return "logfmt"
}

func (f *LogfmtFormatter) Configure(config map[string]interface{}) error {
	if format, ok := config["time_format"].(string); ok {
		f.timeFormat = format
	}
	return nil
}

// Helper function to format values for logfmt
func formatValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		// Quote strings if they contain spaces or special characters
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
	return &CEFFormatter{
		deviceVendor:  vendor,
		deviceProduct: product,
		deviceVersion: version,
	}
}

func (f *CEFFormatter) Format(entry *domain.LogEntry) string {
	if entry == nil {
		return ""
	}

	// CEF:Version|Device Vendor|Device Product|Device Version|Device Event Class ID|Name|Severity|Extension
	severity := f.mapSeverity(entry.Level)

	// Build extension
	var extensions []string

	// Add timestamp
	extensions = append(extensions, fmt.Sprintf("rt=%d", entry.Timestamp.UnixMilli()))

	// Add message
	extensions = append(extensions, fmt.Sprintf("msg=%s", escapeExtension(entry.Message)))

	// Add fields
	for k, v := range entry.Fields {
		extensions = append(extensions, fmt.Sprintf("%s=%s", k, escapeExtension(fmt.Sprintf("%v", v))))
	}

	// Add error
	if entry.Error != nil {
		extensions = append(extensions, fmt.Sprintf("reason=%s", escapeExtension(entry.Error.Error())))
	}

	return fmt.Sprintf("CEF:0|%s|%s|%s|%s|%s|%d|%s",
		f.deviceVendor,
		f.deviceProduct,
		f.deviceVersion,
		"LOG",
		entry.Level.String(),
		severity,
		strings.Join(extensions, " "),
	)
}

func (f *CEFFormatter) mapSeverity(level domain.Level) int {
	// CEF severity: 0-10
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
	// Escape special characters for CEF extension
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "=", "\\=")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	return s
}

func (f *CEFFormatter) Name() string {
	return "cef"
}

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
