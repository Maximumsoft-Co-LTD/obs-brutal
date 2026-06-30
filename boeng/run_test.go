package boeng_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"obs-brutal/boeng"
)

func TestRun_SuccessReturnsNilError(t *testing.T) {
	boeng.Init(boeng.Config{Service: "t"})
	err := boeng.Run(context.Background(), "ok_op", nil, func(ctx context.Context) error {
		return nil
	})
	if err != nil {
		t.Fatalf("Run returned err: %v", err)
	}
}

func TestRun_PropagatesError(t *testing.T) {
	boeng.Init(boeng.Config{Service: "t"})
	want := errors.New("boom")
	err := boeng.Run(context.Background(), "err_op", nil, func(ctx context.Context) error {
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("Run err = %v, want %v", err, want)
	}
}

func TestRun_RecordsAndReraisesPanic(t *testing.T) {
	boeng.Init(boeng.Config{Service: "t"})
	// Run records the panic against the op then re-raises it so the
	// outer stack unwinds normally. Callers who want the error value
	// instead of unwinding must call recover() themselves.
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("Run swallowed panic; expected re-raise")
		}
		if s, _ := r.(string); s != "kaboom" {
			t.Fatalf("panic value changed: %v want kaboom", r)
		}
	}()
	_ = boeng.Run(context.Background(), "panic_op", nil, func(ctx context.Context) error {
		panic("kaboom")
	})
}

// silence unused import on strings — kept because earlier test variant used it.
var _ = strings.Contains

func TestRunR_ReturnsValueAndError(t *testing.T) {
	boeng.Init(boeng.Config{Service: "t"})
	out, err := boeng.RunR(context.Background(), "rr_ok", nil, func(ctx context.Context) (int, error) {
		return 42, nil
	})
	if err != nil || out != 42 {
		t.Fatalf("RunR ok = (%d, %v), want (42, nil)", out, err)
	}

	out2, err2 := boeng.RunR(context.Background(), "rr_err", nil, func(ctx context.Context) (string, error) {
		return "fallback", errors.New("nope")
	})
	if err2 == nil {
		t.Fatalf("RunR error path lost the error")
	}
	if out2 != "fallback" {
		t.Fatalf("RunR didn't preserve T on error: got %q", out2)
	}
}
