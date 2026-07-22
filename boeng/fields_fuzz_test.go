package boeng_test

import (
	"testing"
	"unicode/utf8"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/boeng"
)

// FuzzMaskEmail hardens the masking path against arbitrary input: it
// must never panic and must always return valid UTF-8 (the CHANGELOG
// already records one real multibyte bug here — จ/ö local parts).
func FuzzMaskEmail(f *testing.F) {
	f.Add("john@doe.com")
	f.Add("jöhn@doe.com")
	f.Add("พี่@example.co.th")
	f.Add("")
	f.Add("@")
	f.Add("no-at-sign")
	f.Add("a@b@c@d")
	f.Add("\xff\xfe@\x80")
	f.Fuzz(func(t *testing.T, s string) {
		out := boeng.MaskEmail(s)
		if utf8.ValidString(s) && !utf8.ValidString(out) {
			t.Fatalf("MaskEmail(%q) produced invalid UTF-8: %q", s, out)
		}
	})
}
