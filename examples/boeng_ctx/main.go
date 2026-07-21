// Context-mode boeng demo: functions thread context.Context but still use
// the imperative Enter/EnterCtx API rather than closures.
//
// Run:   go run ./examples/boeng_ctx
//
// Compared to boeng_legacy, EnterCtx makes nested spans automatic — every
// downstream boeng call under the returned ctx becomes a child span.
// Compared to examples/boeng (the closure example), this style fits codebases
// where wrapping a whole function body in a closure is impractical.
package main

import (
	"context"
	"errors"
	"time"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/boeng"
)

// User opts into Loggable: that gives the author full control of field
// names + masking instead of relying on the reflection fallback's defaults.
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
	obs := boeng.Init(boeng.Config{Service: "ctx_demo", Env: "dev"})
	defer obs.Close()

	ctx := context.Background()
	usr := User{UserID: "u-123", Email: "john@doe.com", Token: "secret-xyz", Name: "John"}

	_ = createUser(ctx, usr)
	_ = chargeCard(ctx, usr, -1)
}

// createUser composes the operation from named steps. Each Step gets its
// own child span, its own metrics (<step>_total / _duration_ms /
// _error_total / _panic_total), and start/completion log lines. A failing
// step automatically cascades the error to the parent op.
func createUser(ctx context.Context, usr User) (err error) {
	ctx, op := boeng.EnterCtx(ctx, "create_user", usr)
	defer op.CloseWith(&err)

	if err = op.Step("validate", func() error {
		op.Log("validating user input", usr)
		return nil
	}); err != nil {
		return err
	}
	return op.Step("save", func() error { return saveUser(ctx, usr) })
}

// saveUser nests because we passed ctx down. Its span is a child of
// create_user without any extra wiring.
func saveUser(ctx context.Context, usr User) (err error) {
	ctx, op := boeng.EnterCtx(ctx, "save_user", usr)
	defer op.CloseWith(&err)

	time.Sleep(20 * time.Millisecond)
	op.Emit("row_inserted", map[string]any{"row_id": 42})
	_ = ctx
	return nil
}

func chargeCard(ctx context.Context, usr User, amount int) (err error) {
	_, op := boeng.EnterCtx(ctx, "charge_card", usr)
	defer op.CloseWith(&err)

	if amount <= 0 {
		return errors.New("amount must be positive")
	}
	return nil
}
