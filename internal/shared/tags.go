package shared

import (
	"fmt"
	"reflect"
	"strings"

	pin "github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/port/inbound"
)

// StructLogger provides struct tag-based logging
type StructLogger struct {
	logger pin.Logger
}

// NewStructLogger creates a new struct logger
func NewStructLogger(logger pin.Logger) *StructLogger {
	return &StructLogger{logger: logger}
}

// LogStruct logs struct fields based on tags
func (sl *StructLogger) LogStruct(v interface{}) pin.Logger {
	logger := sl.logger

	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return logger
	}

	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		fieldValue := val.Field(i)

		// Skip unexported fields
		if !fieldValue.CanInterface() {
			continue
		}

		// Check for log tag
		logTag := field.Tag.Get("log")
		if logTag == "" || logTag == "-" {
			continue
		}

		// Parse tag options
		parts := strings.Split(logTag, ",")
		fieldName := parts[0]
		if fieldName == "" {
			fieldName = strings.ToLower(field.Name)
		}

		value := fieldValue.Interface()

		// Check for sensitive flag
		for _, part := range parts[1:] {
			switch {
			case part == "sensitive=true":
				value = "***"
			case strings.HasPrefix(part, "mask="):
				// Apply mask pattern
				maskPattern := strings.TrimPrefix(part, "mask=")
				value = applyMask(fmt.Sprintf("%v", value), maskPattern)
			}
		}

		logger = logger.F(fieldName, value)
	}

	return logger
}

// applyMask applies a mask pattern to a value
func applyMask(value string, pattern string) string {
	if len(value) == 0 {
		return value
	}

	// Simple mask implementation
	// Pattern like "00000000xxx" means show last 3 chars
	if strings.Contains(pattern, "xxx") {
		// Count x's at the end
		xCount := 0
		for i := len(pattern) - 1; i >= 0 && pattern[i] == 'x'; i-- {
			xCount++
		}

		if len(value) > xCount {
			masked := strings.Repeat("*", len(value)-xCount)
			return masked + value[len(value)-xCount:]
		}
	}

	return "***"
}

// ExtractFields extracts fields from struct based on log tags
func ExtractFields(v interface{}) map[string]interface{} {
	fields := make(map[string]interface{})

	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return fields
	}

	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		fieldValue := val.Field(i)

		// Skip unexported fields
		if !fieldValue.CanInterface() {
			continue
		}

		// Check for log tag
		logTag := field.Tag.Get("log")
		if logTag == "" || logTag == "-" {
			continue
		}

		// Parse tag options
		parts := strings.Split(logTag, ",")
		fieldName := parts[0]
		if fieldName == "" {
			fieldName = strings.ToLower(field.Name)
		}

		value := fieldValue.Interface()

		// Check for sensitive flag
		for _, part := range parts[1:] {
			switch {
			case part == "sensitive=true":
				value = "***"
			case strings.HasPrefix(part, "mask="):
				// Apply mask pattern
				maskPattern := strings.TrimPrefix(part, "mask=")
				value = applyMask(fmt.Sprintf("%v", value), maskPattern)
			}
		}

		fields[fieldName] = value
	}

	return fields
}

// LogStructFields logs struct fields automatically
func LogStructFields(logger pin.Logger, v interface{}) pin.Logger {
	fields := ExtractFields(v)
	for k, v := range fields {
		logger = logger.F(k, v)
	}
	return logger
}

// AutoLogStructFields automatically logs struct fields from request bodies
func AutoLogStructFields(logger pin.Logger) func(interface{}) pin.Logger {
	return func(v interface{}) pin.Logger {
		return LogStructFields(logger, v)
	}
}
