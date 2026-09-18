package boeng_test

import (
	"os/exec"
	"strings"
	"testing"
)

// Importing the core boeng package must not link web/database frameworks
// into the consumer's binary. Adapters live in their own sub-packages
// (boeng/gin, boeng/mongo, ...) so a service pays only for what it uses.
func TestCoreBoengLinksNoFrameworks(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to go list")
	}
	out, err := exec.Command("go", "list", "-deps", "github.com/Maximumsoft-Co-LTD/obs-brutal/boeng").Output()
	if err != nil {
		t.Skipf("go list unavailable: %v", err)
	}
	banned := []string{
		"github.com/gin-gonic/gin",
		"github.com/quic-go/quic-go",
		"go.mongodb.org/mongo-driver",
		"github.com/rabbitmq/amqp091-go",
		"github.com/redis/go-redis",
	}
	for _, line := range strings.Split(string(out), "\n") {
		for _, b := range banned {
			if strings.HasPrefix(line, b) {
				t.Errorf("core boeng links %s", line)
			}
		}
	}
}
