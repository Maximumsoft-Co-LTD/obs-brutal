package boengredis_test

import (
	"testing"

	boengredis "github.com/Maximumsoft-Co-LTD/obs-brutal/boeng/redis"
)

// TestHook_ReturnsHook is the smoke test — verifies the public API
// compiles against the current go-redis version. Behaviour with a live
// Redis is exercised via examples/redis + compose.
func TestHook_ReturnsHook(t *testing.T) {
	if boengredis.Hook() == nil {
		t.Fatal("Hook returned nil")
	}
}
