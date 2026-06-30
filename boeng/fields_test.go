package boeng_test

import (
	"testing"

	"obs-brutal/boeng"
)

func extractForTest(v any) map[string]any {
	return boeng.ExtractFieldsForTest(v)
}

type loggableUser struct {
	UserID string
	Email  string
	Token  string
}

func (u loggableUser) LogFields() map[string]any {
	return map[string]any{
		"user_id": u.UserID,
		"email":   boeng.MaskEmail(u.Email),
	}
}

func TestExtractFields_LoggableWinsOverReflection(t *testing.T) {
	u := loggableUser{UserID: "u-1", Email: "john@doe.com", Token: "tok"}
	got := extractForTest(u)
	if got["user_id"] != "u-1" {
		t.Errorf("user_id = %v, want u-1", got["user_id"])
	}
	if got["email"] != "j**n@doe.com" {
		t.Errorf("email = %v, want j**n@doe.com", got["email"])
	}
	if _, ok := got["token"]; ok {
		t.Errorf("token leaked from Loggable; should be absent")
	}
}

type reflectUser struct {
	UserID   string
	Email    string
	Token    string
	Password string
	Phone    string
	Name     string
	HTTPCode int
	unsaid   string //nolint:unused // ensure unexported is skipped
}

func TestExtractFields_ReflectionDefaults(t *testing.T) {
	u := reflectUser{
		UserID:   "u-2",
		Email:    "alice@example.com",
		Token:    "supersecret",
		Password: "hunter2",
		Phone:    "0812345678",
		Name:     "Alice",
		HTTPCode: 200,
	}
	got := extractForTest(u)

	if got["user_id"] != "u-2" {
		t.Errorf("user_id = %v, want u-2", got["user_id"])
	}
	if got["name"] != "Alice" {
		t.Errorf("name = %v, want Alice", got["name"])
	}
	if got["http_code"] != 200 {
		t.Errorf("http_code = %v, want 200 (snake_case acronym)", got["http_code"])
	}
	if v, ok := got["email"].(string); !ok || v == "alice@example.com" {
		t.Errorf("email not masked: %v", got["email"])
	}
	if v, ok := got["token"].(string); !ok || v == "supersecret" {
		t.Errorf("token not masked: %v", got["token"])
	}
	if v, ok := got["password"].(string); !ok || v == "hunter2" {
		t.Errorf("password not masked: %v", got["password"])
	}
	if v, ok := got["phone"].(string); !ok || v == "0812345678" {
		t.Errorf("phone not masked: %v", got["phone"])
	}
	if _, ok := got["unsaid"]; ok {
		t.Errorf("unexported field leaked")
	}
}

func TestExtractFields_SkipsZeroByDefault(t *testing.T) {
	u := reflectUser{UserID: "u-3"} // everything else zero
	got := extractForTest(u)
	if _, ok := got["name"]; ok {
		t.Errorf("zero string emitted as 'name': %v", got["name"])
	}
	if _, ok := got["http_code"]; ok {
		t.Errorf("zero int emitted as 'http_code': %v", got["http_code"])
	}
	if got["user_id"] != "u-3" {
		t.Errorf("user_id = %v, want u-3", got["user_id"])
	}
}

func TestMaskEmail(t *testing.T) {
	cases := map[string]string{
		"john@doe.com": "j**n@doe.com",
		"a@b.co":       "*@b.co",
		"ab@c.io":      "a*@c.io",
		"abcde@x.com":  "a***e@x.com",
		"plainstring":  "***********",
		"@":            "*",
		"trailing@":    "*********",
		"":             "",
		// Multi-byte local parts must remain valid UTF-8 and rune-counted.
		"jöhn@doe.com":     "j**n@doe.com",
		"พ@example.com":    "*@example.com",
		"พี่@example.com":   "พ*่@example.com",
		"ñoñó@x.io":        "ñ**ó@x.io",
	}
	for in, want := range cases {
		got := boeng.MaskEmail(in)
		if got != want {
			t.Errorf("MaskEmail(%q) = %q, want %q", in, got, want)
		}
		if !utf8ValidForTest(got) {
			t.Errorf("MaskEmail(%q) produced invalid UTF-8: %q", in, got)
		}
	}
}

func utf8ValidForTest(s string) bool {
	for _, r := range s {
		if r == 0xFFFD {
			return false
		}
	}
	return true
}

func TestExtractFields_NilAndNonStruct(t *testing.T) {
	if got := extractForTest(nil); len(got) != 0 {
		t.Errorf("nil produced %v, want empty", got)
	}
	if got := extractForTest("just a string"); len(got) != 0 {
		t.Errorf("string produced %v, want empty", got)
	}
	if got := extractForTest(42); len(got) != 0 {
		t.Errorf("int produced %v, want empty", got)
	}
}

func TestExtractFields_MapPassthrough(t *testing.T) {
	m := map[string]any{"row_id": 7, "stage": "done"}
	got := extractForTest(m)
	if got["row_id"] != 7 || got["stage"] != "done" {
		t.Errorf("map passthrough lost data: %v", got)
	}
}
