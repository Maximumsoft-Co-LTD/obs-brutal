// API Freeze test. The boeng public surface is frozen at v1 — adding
// a public symbol is fine, removing or renaming one is not. This file
// references every documented exported identifier exactly once so the
// Go compiler enforces the freeze: removing any symbol causes go build
// (and thus go test) to fail.
//
// When intentionally extending the API, add the new symbol below.
// When intentionally REMOVING one (major version), delete the
// corresponding line AND update the CHANGELOG.
package boeng_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/boeng"
	boenggin "github.com/Maximumsoft-Co-LTD/obs-brutal/boeng/gin"
	boenghttp "github.com/Maximumsoft-Co-LTD/obs-brutal/boeng/http"
	boengmongo "github.com/Maximumsoft-Co-LTD/obs-brutal/boeng/mongo"
	boengrabbit "github.com/Maximumsoft-Co-LTD/obs-brutal/boeng/rabbit"
	boengredis "github.com/Maximumsoft-Co-LTD/obs-brutal/boeng/redis"
)

// TestPublicAPI compiles a list of references to every public identifier
// boeng promises to expose. The test body never actually executes the
// references — it just exists so the compiler must resolve every name.
func TestPublicAPI(t *testing.T) {
	if testing.Short() {
		t.Skip("freeze surface is compile-time; nothing to do at runtime")
	}
	_ = freezeRefs
}

// freezeRefs is unexported but reachable from TestPublicAPI's body,
// which forces the compiler to keep every reference live. Removing or
// renaming any of these symbols breaks the build.
var freezeRefs = func() any {
	// ----- core package: boeng -----
	var (
		_ boeng.Config
		_ boeng.Obs
		_ boeng.Logger
		_ boeng.Level
		_ boeng.Loggable
		_ *boeng.Op
		_ boeng.FieldsConfig
		_ boeng.Sink
	)
	_ = boeng.SetSinkForTest
	_ = boeng.Init
	_ = boeng.D
	_ = (*boeng.Obs).Close
	_ = (*boeng.Obs).Log
	_ = boeng.L
	_ = boeng.Run
	_ = boeng.RunR[int] // generic instantiation must compile
	_ = boeng.Enter
	_ = boeng.EnterCtx
	_ = boeng.Emit
	_ = (*boeng.Op).Log
	_ = (*boeng.Op).Emit
	_ = (*boeng.Op).Step
	_ = (*boeng.Op).Fail
	_ = (*boeng.Op).Success
	_ = (*boeng.Op).Close
	_ = (*boeng.Op).CloseWith
	_ = (*boeng.Op).Context
	_ = boeng.MaskEmail
	_ = boeng.DebugLevel
	_ = boeng.InfoLevel
	_ = boeng.WarnLevel
	_ = boeng.ErrorLevel
	_ = boeng.FatalLevel

	// ----- adapter: boeng/gin -----
	_ = boenggin.Middleware
	_ = boenggin.L

	// ----- adapter: boeng/http -----
	_ = boenghttp.Middleware
	_ = boenghttp.Wrap
	_ = boenghttp.Transport

	// ----- adapter: boeng/mongo -----
	_ = boengmongo.CommandMonitor

	// ----- adapter: boeng/redis -----
	_ = boengredis.Hook

	// ----- adapter: boeng/rabbit -----
	_ = boengrabbit.Publish
	_ = boengrabbit.Consume
	_ = boengrabbit.HandlerFunc(func(ctx context.Context, _ amqp.Delivery) error { return nil })

	// Suppress unused-import warnings on adapter-only types that don't
	// otherwise appear in the symbol list above.
	var (
		_ gin.HandlerFunc
		_ http.RoundTripper
		_ = errors.New
	)
	return nil
}
