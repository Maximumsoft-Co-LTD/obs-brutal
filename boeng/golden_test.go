// Golden Trace test. We exercise a canonical operation tree and compare
// its log stream (with volatile fields redacted) against a checked-in
// JSON file under testdata/. The goal is to catch silent shape drift:
// if a future change accidentally drops a field, reorders entries, or
// changes the level of a log line, this test fails loudly.
//
// To regenerate the golden after an intentional change:
//
//	go test -run TestGoldenTrace -update ./boeng/...
package boeng_test

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/boeng"
	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/domain"
)

var updateGolden = flag.Bool("update", false, "rewrite golden testdata files")

func TestGoldenTrace_CreateUser(t *testing.T) {
	sink := &captureSink{}
	reset := boeng.SetSinkForTest(sink)
	defer reset()
	obs := boeng.Init(boeng.Config{Service: "golden", Env: "dev"})
	defer obs.Close()

	// Canonical operation tree: an outer business operation composed of
	// three named phases plus a single domain event.
	_ = boeng.Run(context.Background(), "create_user",
		map[string]any{"user_id": "u-42"},
		func(ctx context.Context) error {
			_, op := boeng.EnterCtx(ctx, "create_user_phases", nil)
			defer op.Close()
			_ = op.Step("validate", func() error { return nil })
			_ = op.Step("insert_db", func() error { return nil })
			op.Emit("user_created", map[string]any{"row_id": 1})
			_ = op.Step("publish", func() error { return errors.New("topic offline") })
			return nil
		},
	)

	got := redactForGolden(sink.snapshot())
	golden := filepath.Join("testdata", "golden_create_user.json")

	if *updateGolden {
		writeGolden(t, golden, got)
		t.Logf("rewrote %s", golden)
		return
	}

	want := readGolden(t, golden)
	if diff := jsonDiff(want, got); diff != "" {
		t.Fatalf("golden mismatch (run go test -update to refresh):\n%s", diff)
	}
}

// redactForGolden strips volatile fields (timestamps, durations, dynamic
// IDs) so the comparison is stable across runs. We keep level, msg, op,
// service, env, and other intentional fields.
func redactForGolden(entries []domain.LogEntry) []map[string]any {
	out := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		row := map[string]any{
			"level":   e.Level.String(),
			"msg":     e.Msg,
			"service": e.F["service"],
			"env":     e.F["env"],
			"op":      e.F["op"],
		}
		if e.UserID != "" {
			row["user_id"] = e.UserID
		}
		if e.Mod != "" {
			row["module"] = e.Mod
		}
		if e.TenantID != "" {
			row["tenant_id"] = e.TenantID
		}
		// Bring through stable, non-volatile fields from F.
		for _, k := range []string{"event", "error", "row_id"} {
			if v, ok := e.F[k]; ok {
				row[k] = v
			}
		}
		// Drop nil entries the way json.Marshal would.
		for k, v := range row {
			if v == nil {
				delete(row, k)
			}
		}
		out = append(out, row)
	}
	return out
}

func writeGolden(t *testing.T, path string, v any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	buf, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, append(buf, '\n'), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readGolden(t *testing.T, path string) []map[string]any {
	t.Helper()
	buf, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (regenerate with -update)", path, err)
	}
	var v []map[string]any
	if err := json.Unmarshal(buf, &v); err != nil {
		t.Fatalf("parse golden %s: %v", path, err)
	}
	return v
}

func jsonDiff(want, got []map[string]any) string {
	w, _ := json.MarshalIndent(want, "", "  ")
	g, _ := json.MarshalIndent(got, "", "  ")
	if string(w) == string(g) {
		return ""
	}
	// Show first differing index for readability.
	for i := 0; i < len(want) && i < len(got); i++ {
		wb, _ := json.Marshal(want[i])
		gb, _ := json.Marshal(got[i])
		if string(wb) != string(gb) {
			return "entry[" + intStr(i) + "]:\n  want: " + string(wb) + "\n  got:  " + string(gb)
		}
	}
	return "length differs: want=" + intStr(len(want)) + " got=" + intStr(len(got)) + "\nfull want:\n" + string(w) + "\nfull got:\n" + string(g)
}

func intStr(i int) string {
	return string([]byte{byte('0' + i)})
}

// keep sort imported (used to keep map iteration order stable in the
// future if we extend the comparator); avoids a lint diff later.
var _ = sort.Strings
