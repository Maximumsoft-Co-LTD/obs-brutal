package boeng_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/boeng"
)

func TestEnter_CloseSucceeds(t *testing.T) {
	boeng.Init(boeng.Config{Service: "t"})
	op := boeng.Enter("legacy_op", map[string]any{"k": "v"})
	op.Log("midway", map[string]any{"step": "midway"})
	op.Emit("validated", map[string]any{"ok": true})
	op.Close()
	op.Close() // second call must be a no-op, not a panic
}

func TestEnter_FailRecordsError(t *testing.T) {
	boeng.Init(boeng.Config{Service: "t"})
	op := boeng.Enter("legacy_err")
	op.Fail(errors.New("nope"))
	op.Close()
}

func TestEnterCtx_CloseWithCapturesNamedReturn(t *testing.T) {
	boeng.Init(boeng.Config{Service: "t"})

	want := errors.New("downstream")
	got := callWithCloseWith(want)
	if !errors.Is(got, want) {
		t.Fatalf("CloseWith lost named return: got %v, want %v", got, want)
	}

	if got := callWithCloseWith(nil); got != nil {
		t.Fatalf("CloseWith fabricated error on success: %v", got)
	}
}

func callWithCloseWith(in error) (err error) {
	_, op := boeng.EnterCtx(context.Background(), "with_close_with")
	defer op.CloseWith(&err)
	if in != nil {
		return in
	}
	return nil
}

func TestEnterCtx_PanicReraisedAfterClose(t *testing.T) {
	boeng.Init(boeng.Config{Service: "t"})
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic to re-raise out of Close")
		}
		s, _ := r.(string)
		if s != "kaboom" {
			t.Fatalf("panic value changed: got %v want kaboom", r)
		}
	}()
	func() {
		_, op := boeng.EnterCtx(context.Background(), "panic_close")
		defer op.Close()
		panic("kaboom")
	}()
}

func TestEnterCtx_PanicSetsErrorOnCloseWith(t *testing.T) {
	boeng.Init(boeng.Config{Service: "t"})

	var captured error
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic to re-raise out of CloseWith")
		}
		if captured == nil || !strings.Contains(captured.Error(), "panic") {
			t.Fatalf("CloseWith did not pin panic error into errp: %v", captured)
		}
	}()

	func() (err error) {
		// outer defer reads err AFTER CloseWith has set it from the panic
		defer func() { captured = err }()
		_, op := boeng.EnterCtx(context.Background(), "panic_close_with")
		defer op.CloseWith(&err)
		panic("explode")
	}()
}

func TestOp_StepSuccess(t *testing.T) {
	boeng.Init(boeng.Config{Service: "t"})
	op := boeng.Enter("with_steps")
	calls := 0
	if err := op.Step("validate", func() error { calls++; return nil }); err != nil {
		t.Fatalf("validate step: %v", err)
	}
	if err := op.Step("insert_db", func() error { calls++; return nil }); err != nil {
		t.Fatalf("insert_db step: %v", err)
	}
	op.Close()
	if calls != 2 {
		t.Fatalf("expected 2 step invocations, got %d", calls)
	}
}

func TestOp_StepErrorCascadesToParent(t *testing.T) {
	boeng.Init(boeng.Config{Service: "t"})
	want := errors.New("invalid input")
	// Even though we discard the step return value here, the parent op
	// must still log at ERROR because Step cascades the error.
	op := boeng.Enter("cascade_test")
	_ = op.Step("validate", func() error { return want })
	op.Close()
}

func TestOp_StepReturnsErrorToCaller(t *testing.T) {
	boeng.Init(boeng.Config{Service: "t"})
	want := errors.New("downstream")
	op := boeng.Enter("step_returns_err")
	got := op.Step("publish", func() error { return want })
	if !errors.Is(got, want) {
		t.Fatalf("Step returned %v, want %v", got, want)
	}
	op.Close()
}

func TestOp_StepPanicReraisesAndCascades(t *testing.T) {
	boeng.Init(boeng.Config{Service: "t"})
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("Step did not re-raise panic")
		}
		if s, _ := r.(string); s != "kaboom" {
			t.Fatalf("panic value changed: %v", r)
		}
	}()
	op := boeng.Enter("step_panic")
	defer op.Close()
	_ = op.Step("publish", func() error { panic("kaboom") })
}

func TestOp_StepOnNilReceiverJustCallsFn(t *testing.T) {
	var op *boeng.Op
	called := false
	if err := op.Step("noop", func() error { called = true; return nil }); err != nil {
		t.Fatalf("nil-receiver Step returned err: %v", err)
	}
	if !called {
		t.Fatalf("nil-receiver Step did not run fn")
	}
}
