// Legacy-mode boeng demo: functions don't take or return context.
//
// Run:   go run ./examples/boeng_legacy
//
// The pattern is:
//
//	op := boeng.Enter("name", subject)
//	defer op.Close()
//	// ... do work, optionally op.Log / op.Emit / op.Fail ...
//
// No function signature changes are required. Trace nesting still works as
// long as you pass op.Context() into anything that calls boeng again.
package main

import (
	"errors"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/boeng"
)

// User is a plain DTO. No tags. The reflection fallback converts field
// names to snake_case (UserID -> user_id) and auto-masks "email", "token",
// "password" by name.
type User struct {
	UserID   string
	Email    string
	Token    string
	Name     string
	Password string
}

func main() {
	obs := boeng.Init(boeng.Config{Service: "legacy_demo", Env: "dev"})
	defer obs.Close()

	usr := User{
		UserID:   "u-123",
		Email:    "john@doe.com",
		Token:    "secret-xyz",
		Name:     "John",
		Password: "hunter2",
	}

	createUser(usr)
	_ = chargeCard(usr, -1)
}

// createUser uses Enter — no ctx in or out. Everything observability is
// driven through the *Op handle.
func createUser(usr User) {
	op := boeng.Enter("create_user", usr)
	defer op.Close()

	op.Log("validating user input", usr)
	op.Emit("row_inserted", map[string]any{"row_id": 42})
}

// chargeCard shows the error path. Call Fail(err) before Close to flip
// the completion log to ERROR and record the error on the span.
func chargeCard(usr User, amount int) error {
	op := boeng.Enter("charge_card", usr)
	defer op.Close()

	if amount <= 0 {
		err := errors.New("amount must be positive")
		op.Fail(err)
		return err
	}
	return nil
}
