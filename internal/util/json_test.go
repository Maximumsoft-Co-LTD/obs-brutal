package util_test

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"obs-brutal/internal/core/domain"
	"obs-brutal/internal/util"
)

func TestWriteJSONToWriterPropagatesMarshalError(t *testing.T) {
	entry := &domain.LogEntry{
		Level:     domain.InfoLevel,
		Msg:       "test",
		Timestamp: time.Unix(0, 0),
		F: map[string]interface{}{
			"bad": func() {},
		},
	}
	var buf bytes.Buffer
	if err := util.WriteJSONToWriter(&buf, entry); err == nil {
		t.Fatalf("expected error from WriteJSONToWriter, got nil")
	}
	if buf.Len() != 0 {
		t.Fatalf("expected buffer to remain empty, got %d bytes", buf.Len())
	}
}

func TestWriteJSONToWriterPromotedFields(t *testing.T) {
	cases := []struct {
		name      string
		mutate    func(*domain.LogEntry)
		wantKeys  map[string]string
		emptyKeys []string
	}{
		{
			name:      "emits user_id when set",
			mutate:    func(e *domain.LogEntry) { e.UserID = "u-123" },
			wantKeys:  map[string]string{"user_id": "u-123"},
			emptyKeys: []string{"module", "tenant_id"},
		},
		{
			name:      "emits module when set",
			mutate:    func(e *domain.LogEntry) { e.Mod = "billing" },
			wantKeys:  map[string]string{"module": "billing"},
			emptyKeys: []string{"user_id", "tenant_id"},
		},
		{
			name:      "emits tenant_id when set",
			mutate:    func(e *domain.LogEntry) { e.TenantID = "t-7" },
			wantKeys:  map[string]string{"tenant_id": "t-7"},
			emptyKeys: []string{"user_id", "module"},
		},
		{
			name:      "omits all three when empty",
			mutate:    func(*domain.LogEntry) {},
			emptyKeys: []string{"user_id", "module", "tenant_id"},
		},
		{
			name: "does not double-emit when entry.F also contains user_id",
			mutate: func(e *domain.LogEntry) {
				e.UserID = "u-123"
				e.F["user_id"] = "shadow"
				e.F["module"] = "shadow"
				e.F["tenant_id"] = "shadow"
			},
			wantKeys: map[string]string{"user_id": "u-123"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := &domain.LogEntry{
				Level:     domain.InfoLevel,
				Msg:       "hello",
				Timestamp: time.Unix(0, 0),
				F:         map[string]interface{}{},
			}
			tc.mutate(entry)

			var buf bytes.Buffer
			if err := util.WriteJSONToWriter(&buf, entry); err != nil {
				t.Fatalf("WriteJSONToWriter: %v", err)
			}
			var out map[string]any
			if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
				t.Fatalf("invalid JSON: %v (raw=%q)", err, buf.String())
			}
			for k, want := range tc.wantKeys {
				got, ok := out[k]
				if !ok {
					t.Errorf("missing key %q in output: %q", k, buf.String())
					continue
				}
				if got != want {
					t.Errorf("key %q = %v, want %v", k, got, want)
				}
			}
			for _, k := range tc.emptyKeys {
				if _, ok := out[k]; ok {
					t.Errorf("expected key %q to be omitted, got %v in: %q", k, out[k], buf.String())
				}
			}
		})
	}
}
