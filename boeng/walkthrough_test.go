// Walkthrough verification.
//
// docs/walkthrough.md promises "every JSON line below is real output". This
// test makes that promise enforceable: it runs the same scenarios the doc
// describes, captures the runtime's actual log entries, and asserts both:
//
//   1. the runtime IS producing the (msg, level, op, fields) tuples the
//      doc claims it does, AND
//   2. docs/walkthrough.md actually contains a JSON line carrying those
//      tuples (no "made-up output" lines).
//
// If either side drifts — runtime changes shape, or someone edits the
// doc to claim something the code doesn't do — this test fails.
package boeng_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/boeng"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
)

// expectedLine is a fingerprint of one log entry the scenario must produce.
// Comparison is field-by-field, not byte-exact, so volatile data
// (timestamps, durations) is irrelevant.
type expectedLine struct {
	msg    string
	level  string            // "DEBUG" | "INFO" | "ERROR"
	op     string            // op field on F
	extras map[string]string // other (key,value) pairs that must coexist
}

type walkScenario struct {
	name            string
	run             func(t *testing.T)
	expectedLines   []expectedLine
	expectedMetrics []string // metric names the doc must mention
}

func walkScenarios() []walkScenario {
	return []walkScenario{
		{
			name: "01-run-simple",
			run: func(t *testing.T) {
				type User struct {
					UserID string
					Name   string
				}
				u := User{UserID: "u-1", Name: "Alice"}
				_ = boeng.Run(context.Background(), "create_user", u, func(ctx context.Context) error {
					return nil
				})
			},
			expectedLines: []expectedLine{
				{msg: "create_user started", level: "DEBUG", op: "create_user", extras: map[string]string{"user_id": "u-1"}},
				{msg: "create_user completed", level: "INFO", op: "create_user", extras: map[string]string{"user_id": "u-1"}},
			},
			expectedMetrics: []string{"create_user_total", "create_user_duration_ms"},
		},
		{
			name: "02-run-error",
			run: func(t *testing.T) {
				_ = boeng.Run(context.Background(), "charge_card", nil, func(ctx context.Context) error {
					return errors.New("amount must be positive")
				})
			},
			expectedLines: []expectedLine{
				{msg: "charge_card failed", level: "ERROR", op: "charge_card", extras: map[string]string{"error": "amount must be positive"}},
			},
			expectedMetrics: []string{"charge_card_error_total"},
		},
		{
			name: "03-run-panic",
			run: func(t *testing.T) {
				defer func() { _ = recover() }() // boeng re-raises; absorb so test continues
				_ = boeng.Run(context.Background(), "publish", nil, func(ctx context.Context) error {
					panic("kaboom")
				})
			},
			expectedLines: []expectedLine{
				{msg: "publish failed", level: "ERROR", op: "publish", extras: map[string]string{"error": "panic: kaboom"}},
			},
			expectedMetrics: []string{"publish_panic_total", "publish_error_total"},
		},
		{
			name: "04-enter-step-nested",
			run: func(t *testing.T) {
				ctx := context.Background()
				_, op := boeng.EnterCtx(ctx, "create_user", nil)
				defer op.Close()
				_ = op.Step("validate", func() error { return nil })
				_ = op.Step("insert_db", func() error { return nil })
			},
			expectedLines: []expectedLine{
				{msg: "validate completed", level: "INFO", op: "validate"},
				{msg: "insert_db completed", level: "INFO", op: "insert_db"},
				{msg: "create_user completed", level: "INFO", op: "create_user"},
			},
			expectedMetrics: []string{"validate_total", "insert_db_total", "create_user_total"},
		},
		{
			name: "05-emit-event",
			run: func(t *testing.T) {
				_ = boeng.Run(context.Background(), "save_user", nil, func(ctx context.Context) error {
					boeng.Emit(ctx, "row_inserted", map[string]any{"row_id": 42})
					return nil
				})
			},
			expectedLines: []expectedLine{
				{msg: "row_inserted", level: "INFO", op: "save_user", extras: map[string]string{"event": "row_inserted", "row_id": "42"}},
				{msg: "save_user completed", level: "INFO", op: "save_user"},
			},
			expectedMetrics: []string{"row_inserted_total", "save_user_total"},
		},
		{
			name: "06-loggable-masking",
			run: func(t *testing.T) {
				// Struct without LogFields() → reflection fallback applies:
				// snake_case names, zero values skipped, sensitive keys
				// (email/token) auto-masked.
				type User struct {
					UserID string
					Email  string
					Token  string
				}
				u := User{UserID: "u-1", Email: "john@doe.com", Token: "secret-xyz"}
				_ = boeng.Run(context.Background(), "load_user", u, func(ctx context.Context) error {
					return nil
				})
			},
			expectedLines: []expectedLine{
				{msg: "load_user completed", level: "INFO", op: "load_user", extras: map[string]string{
					"user_id": "u-1",
					"email":   "j**n@doe.com",
					"token":   "s********z",
				}},
			},
			expectedMetrics: []string{"load_user_total"},
		},
	}
}

func TestWalkthrough_DocAndRuntimeMatch(t *testing.T) {
	docPath := filepath.Join("..", "docs", "walkthrough.md")
	docBytes, err := os.ReadFile(docPath)
	doc := string(docBytes)
	if err != nil {
		// Run runtime checks anyway so the author can see what the
		// runtime actually produces while drafting the doc.
		t.Logf("read %s: %v (running runtime checks only)", docPath, err)
		doc = ""
	}

	for _, sc := range walkScenarios() {
		t.Run(sc.name, func(t *testing.T) {
			sink := &captureSink{}
			reset := boeng.SetSinkForTest(sink)
			defer reset()
			boeng.Init(boeng.Config{Service: "demo"})

			sc.run(t)
			entries := sink.snapshot()

			for _, want := range sc.expectedLines {
				if !runtimeProduced(entries, want) {
					t.Errorf("RUNTIME did not produce: %s\ncaptured:\n%s", describe(want), dumpEntries(entries))
				}
				if !docShowsLine(doc, want) {
					t.Errorf("DOC docs/walkthrough.md does not show a JSON line for: %s", describe(want))
				}
			}
			for _, metric := range sc.expectedMetrics {
				if !strings.Contains(doc, metric) {
					t.Errorf("DOC docs/walkthrough.md does not mention metric %q", metric)
				}
			}
		})
	}
}

// TestWalkthrough_CardinalityGuard is a special case — it doesn't capture
// log entries but proves the doc's claims about which subject fields make
// it into metric labels.
func TestWalkthrough_CardinalityGuard(t *testing.T) {
	docPath := filepath.Join("..", "docs", "walkthrough.md")
	docBytes, _ := os.ReadFile(docPath)
	doc := string(docBytes)

	boeng.Init(boeng.Config{
		Service:      "demo",
		Env:          "prod",
		MetricLabels: []string{"tier"},
	})
	labels := boeng.MetricLabelsForTest(map[string]any{
		"user_id":  "u-99999",   // high-cardinality, MUST be filtered
		"order_id": "ord-12345", // high-cardinality, MUST be filtered
		"tier":     "premium",   // allowlisted, MUST survive
	})

	keys := map[string]bool{}
	for _, kv := range labels {
		keys[string(kv.Key)] = true
	}

	// Runtime check
	if !keys["service"] || !keys["env"] || !keys["tier"] {
		t.Errorf("runtime missing required labels: %v", keys)
	}
	if keys["user_id"] || keys["order_id"] {
		t.Errorf("runtime leaked high-cardinality label: %v", keys)
	}

	// Doc check — walkthrough must explain this exact behavior
	for _, must := range []string{"user_id", "order_id", "tier", "MetricLabels"} {
		if !strings.Contains(doc, must) {
			t.Errorf("DOC docs/walkthrough.md must mention %q in the cardinality scenario", must)
		}
	}
}

// runtimeProduced searches the captured entries for one whose msg, level,
// op, and extras ALL match the expected line.
func runtimeProduced(entries []domain.LogEntry, want expectedLine) bool {
	for _, e := range entries {
		if e.Msg != want.msg {
			continue
		}
		if want.level != "" && e.Level.String() != want.level {
			continue
		}
		if want.op != "" {
			gotOp, _ := entryFieldValue(e, "op")
			if gotOp != want.op {
				continue
			}
		}
		ok := true
		for k, v := range want.extras {
			got, present := entryFieldValue(e, k)
			if !present || got != v {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		return true
	}
	return false
}

// entryFieldValue reads a field from a LogEntry, knowing that promoted IDs
// live in dedicated columns (UserID, Mod, TenantID, TraceID, SpanID,
// RequestID) instead of in F after Promote() deletes them.
func entryFieldValue(e domain.LogEntry, key string) (string, bool) {
	switch key {
	case "user_id":
		if e.UserID != "" {
			return e.UserID, true
		}
		return "", false
	case "module":
		if e.Mod != "" {
			return e.Mod, true
		}
		return "", false
	case "tenant_id":
		if e.TenantID != "" {
			return e.TenantID, true
		}
		return "", false
	case "trace_id":
		if e.TraceID != "" {
			return e.TraceID, true
		}
		return "", false
	case "span_id":
		if e.SpanID != "" {
			return e.SpanID, true
		}
		return "", false
	case "request_id":
		if e.RequestID != "" {
			return e.RequestID, true
		}
		return "", false
	default:
		if v, ok := e.F[key]; ok {
			return fmt.Sprintf("%v", v), true
		}
	}
	return "", false
}

// docShowsLine asserts that docs/walkthrough.md contains a single line of
// text where ALL of the expected fragments coexist. Substring containment
// on key=value JSON fragments — order independent, timestamp-tolerant.
func docShowsLine(doc string, want expectedLine) bool {
	fragments := []string{
		fmt.Sprintf(`"msg":"%s"`, want.msg),
	}
	if want.level != "" {
		fragments = append(fragments, fmt.Sprintf(`"level":"%s"`, want.level))
	}
	if want.op != "" {
		fragments = append(fragments, fmt.Sprintf(`"op":"%s"`, want.op))
	}
	for k, v := range want.extras {
		fragments = append(fragments, fmt.Sprintf(`"%s":"%s"`, k, v))
		// integers may be written without quotes in JSON; accept that too
		fragments = append(fragments, fmt.Sprintf(`"%s":%s`, k, v))
	}

	for _, line := range strings.Split(doc, "\n") {
		matched := 0
		needed := 1 + boolN(want.level != "") + boolN(want.op != "") + len(want.extras)
		// for extras we accept either quoted or unquoted form (one OR the other)
		seen := map[string]bool{}
		for _, f := range fragments {
			if strings.Contains(line, f) {
				key := keyOf(f)
				if !seen[key] {
					seen[key] = true
					matched++
				}
			}
		}
		if matched >= needed {
			return true
		}
	}
	return false
}

func boolN(b bool) int {
	if b {
		return 1
	}
	return 0
}

func keyOf(fragment string) string {
	// fragment is of form `"name":"value"` or `"name":value` — return name
	if i := strings.Index(fragment, `":"`); i > 0 {
		return fragment[1:i]
	}
	if i := strings.Index(fragment, `":`); i > 0 {
		return fragment[1:i]
	}
	return fragment
}

func describe(want expectedLine) string {
	return fmt.Sprintf("msg=%q level=%q op=%q extras=%v", want.msg, want.level, want.op, want.extras)
}

func dumpEntries(entries []domain.LogEntry) string {
	var b strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&b, "  msg=%q level=%s op=%v userID=%q F=%v\n",
			e.Msg, e.Level.String(), e.F["op"], e.UserID, e.F)
	}
	return b.String()
}
