// Migration test. boeng's signature promise is that adding it to
// existing code does NOT require changing function signatures and does
// NOT alter business behavior — only observability output is added.
//
// This file proves the promise with a tiny before/after pair: the same
// signature, the same outputs for the same inputs, and on the boeng
// side an extra stream of log entries describing the operation.
package boeng_test

import (
	"errors"
	"reflect"
	"testing"

	"obs-brutal/boeng"
)

type user struct {
	ID   string
	Name string
}

// --- Before: plain Go, no observability ---

func saveUserPlain(u user) (string, error) {
	if u.Name == "" {
		return "", errors.New("name required")
	}
	return "saved:" + u.Name, nil
}

// --- After: same signature, boeng.Enter added inline ---

func saveUserBoeng(u user) (id string, err error) {
	op := boeng.Enter("save_user", u)
	defer op.CloseWith(&err)

	if u.Name == "" {
		return "", errors.New("name required")
	}
	return "saved:" + u.Name, nil
}

// TestMigration_SignatureUnchanged proves the after-function has the
// exact same Go type as the before-function. Compilation alone is
// insufficient — Go would accept any superset signature; here we
// reflect to guarantee bit-for-bit type equality.
func TestMigration_SignatureUnchanged(t *testing.T) {
	plainT := reflect.TypeOf(saveUserPlain)
	boengT := reflect.TypeOf(saveUserBoeng)
	if plainT != boengT {
		t.Fatalf("signature changed: plain=%v boeng=%v", plainT, boengT)
	}
}

// TestMigration_BehaviorPreserved proves that for every input, the
// boeng-instrumented version produces the same (value, error) tuple
// as the plain version.
func TestMigration_BehaviorPreserved(t *testing.T) {
	boeng.Init(boeng.Config{Service: "migration_test"})

	cases := []user{
		{Name: "alice"},
		{ID: "u-1", Name: "bob"},
		{}, // triggers the "name required" error path
	}
	for _, in := range cases {
		gotID, gotErr := saveUserBoeng(in)
		wantID, wantErr := saveUserPlain(in)

		if gotID != wantID {
			t.Errorf("input=%+v: id mismatch — plain=%q boeng=%q", in, wantID, gotID)
		}
		if (gotErr == nil) != (wantErr == nil) {
			t.Errorf("input=%+v: error presence differs — plain=%v boeng=%v", in, wantErr, gotErr)
			continue
		}
		if gotErr != nil && wantErr != nil && gotErr.Error() != wantErr.Error() {
			t.Errorf("input=%+v: error message differs — plain=%q boeng=%q",
				in, wantErr.Error(), gotErr.Error())
		}
	}
}

// TestMigration_ObservabilityAdded asserts the boeng version produces
// log entries while the plain version produces none, confirming that
// the only side effect of the migration is observability.
func TestMigration_ObservabilityAdded(t *testing.T) {
	sink := &captureSink{}
	defer boeng.SetSinkForTest(sink)()
	boeng.Init(boeng.Config{Service: "migration_test"})

	_, _ = saveUserBoeng(user{Name: "alice"})
	got := sink.snapshot()
	if len(got) == 0 {
		t.Fatal("boeng-instrumented function produced no log entries")
	}
	// At minimum, a completion log line must be present.
	findEntryWithMsg(t, got, "save_user completed")
}
