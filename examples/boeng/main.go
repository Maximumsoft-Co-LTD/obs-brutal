// Demo of the boeng/ Operation Runtime — closure style.
//
// Run:   go run ./examples/boeng
//
// The mental model is "operations". A function that describes one business
// operation wraps its body in boeng.Run; everything else (log start/end,
// span, duration, per-op metrics, panic recovery, error recording) happens
// automatically. The top-level verbs are: Init, Run, Enter, Emit. Manual
// logging mid-flow lives on the operation handle (see examples/boeng_ctx).
//
// No struct tags. Domain types control their log shape by implementing
// LogFields(); types without that method fall back to reflection over
// non-zero exported fields, with snake_case naming and sensitive-key masking.
package main

import (
	"context"
	"errors"
	"time"

	"obs-brutal/boeng"
)

// User is a plain DTO. Observability concerns live in LogFields() — Token
// never reaches a log because it isn't returned, Email is masked at the
// source so it's masked everywhere.
type User struct {
	UserID string
	Email  string
	Token  string
	Name   string
}

func (u User) LogFields() map[string]any {
	return map[string]any{
		"user_id": u.UserID,
		"name":    u.Name,
		"email":   boeng.MaskEmail(u.Email),
	}
}

func main() {
	obs := boeng.Init(boeng.Config{
		Service: "demo",
		Version: "0.1.0",
		Env:     "dev",
		// OTel: "localhost:4317",
	})
	defer obs.Close()

	ctx := context.Background()
	usr := User{UserID: "u-123", Email: "john@doe.com", Token: "secret-xyz", Name: "John"}

	_ = createUser(ctx, usr)
	_ = chargeCard(ctx, usr, -1)
}

// One observability line per function: `return boeng.Run(...)`.
// Trace span, start/completion logs, duration, error, panic recovery, and
// metrics are all handled inside Run. If you need to log a mid-flow note,
// switch this function to use boeng.EnterCtx and call op.Log (see
// examples/boeng_ctx).
func createUser(ctx context.Context, usr User) error {
	return boeng.Run(ctx, "create_user", usr, func(ctx context.Context) error {
		return saveUser(ctx, usr)
	})
}

// Nested Run becomes a child operation automatically because ctx carries
// the parent's context. boeng.Emit records a named domain event inside an
// operation — log line + event counter + trace event in one call.
func saveUser(ctx context.Context, usr User) error {
	return boeng.Run(ctx, "save_user", usr, func(ctx context.Context) error {
		time.Sleep(20 * time.Millisecond)
		boeng.Emit(ctx, "row_inserted", map[string]any{"row_id": 42})
		return nil
	})
}

// Error path: returning a non-nil error flips the completion log to ERROR
// and records the error on the span. No extra plumbing.
func chargeCard(ctx context.Context, usr User, amount int) error {
	return boeng.Run(ctx, "charge_card", usr, func(ctx context.Context) error {
		if amount <= 0 {
			return errors.New("amount must be positive")
		}
		return nil
	})
}
