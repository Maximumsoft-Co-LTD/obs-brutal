// AI Transformation Test.
//
// boeng's product claim is that an AI coding assistant — or a human
// reading nothing but the README rules — can add observability to
// existing Go code with a tiny, mechanical edit. This file pins the
// SHAPE of that edit by storing canonical before/after pairs under
// boeng/testdata/transform/ and checking three things per pair:
//
//  1. Both files parse as Go (we don't ship broken examples).
//  2. The "after" file imports boeng (and only boeng / its adapters)
//     for observability — never log/slog, zap, zerolog, or
//     go.opentelemetry.io/otel directly.
//  3. The edit budget — LOC delta — is small enough that a sane diff
//     reviewer would approve it as "just instrumentation".
//
// If a future change to boeng makes the canonical edit longer or
// requires a forbidden import, this test fails and forces the team
// to either fix boeng or update the published budget.
package boeng_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type transformCase struct {
	name       string
	maxAddedLines int      // upper bound on net LOC added by the boeng transform
	requireImports []string  // imports the "after" file MUST contain
}

// banned is the set of imports the after.txt MUST NOT contain. boeng's
// promise: business code never sees these names. If any case needs one,
// the boeng API has a hole — fix it before relaxing this list.
var banned = []string{
	"log/slog",
	"go.uber.org/zap",
	"github.com/rs/zerolog",
	"go.opentelemetry.io/otel",
	"go.opentelemetry.io/otel/trace",
	"go.opentelemetry.io/otel/metric",
}

func TestAITransformation_Fixtures(t *testing.T) {
	cases := []transformCase{
		{name: "simple", maxAddedLines: 3, requireImports: []string{"github.com/Maximumsoft-Co-LTD/obs-brutal/boeng"}},
		{name: "errret", maxAddedLines: 4, requireImports: []string{"github.com/Maximumsoft-Co-LTD/obs-brutal/boeng"}},
		{name: "panic_recover", maxAddedLines: 5, requireImports: []string{"github.com/Maximumsoft-Co-LTD/obs-brutal/boeng"}},
		{name: "http_handler", maxAddedLines: 3, requireImports: []string{"github.com/Maximumsoft-Co-LTD/obs-brutal/boeng/http"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			beforePath := filepath.Join("testdata", "transform", tc.name, "before.txt")
			afterPath := filepath.Join("testdata", "transform", tc.name, "after.txt")
			before := mustReadFile(t, beforePath)
			after := mustReadFile(t, afterPath)

			// 1. Both snippets must parse as valid Go.
			fset := token.NewFileSet()
			if _, err := parser.ParseFile(fset, beforePath, before, parser.AllErrors); err != nil {
				t.Fatalf("before.txt does not parse: %v", err)
			}
			afterFile, err := parser.ParseFile(fset, afterPath, after, parser.AllErrors)
			if err != nil {
				t.Fatalf("after.txt does not parse: %v", err)
			}

			// 2. after.txt imports the required boeng package(s) and
			// does NOT import any of the banned observability libraries.
			imports := map[string]bool{}
			for _, imp := range afterFile.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				imports[path] = true
			}
			for _, req := range tc.requireImports {
				if !imports[req] {
					t.Errorf("after.txt missing required import %q (has %v)", req, keys(imports))
				}
			}
			for _, b := range banned {
				if imports[b] {
					t.Errorf("after.txt leaks observability internals via banned import %q", b)
				}
			}

			// 3. LOC delta must stay within budget. Non-blank lines added
			// is the honest measure — empty lines and `}` braces don't
			// count.
			added := nonBlankLineDelta(string(before), string(after))
			if added > tc.maxAddedLines {
				t.Errorf("transform too heavy: %d non-blank lines added (budget %d)", added, tc.maxAddedLines)
			}
			t.Logf("case=%s loc_delta=%d budget=%d", tc.name, added, tc.maxAddedLines)
		})
	}
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// nonBlankLineDelta returns max(0, after_non_blank - before_non_blank).
// We don't do a structural diff because the goal is to budget the cost
// of instrumentation, not to enforce a specific edit shape.
func nonBlankLineDelta(before, after string) int {
	return nonBlankLineCount(after) - nonBlankLineCount(before)
}

func nonBlankLineCount(s string) int {
	n := 0
	for _, line := range strings.Split(s, "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		// Standalone braces don't carry intent — skip them so the budget
		// reflects actual logic added, not formatting.
		if t == "{" || t == "}" || t == "(" || t == ")" {
			continue
		}
		n++
	}
	return n
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
