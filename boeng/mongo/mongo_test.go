package boengmongo_test

import (
	"testing"

	"go.mongodb.org/mongo-driver/event"

	boengmongo "github.com/Maximumsoft-Co-LTD/obs-brutal/boeng/mongo"
)

// TestCommandMonitor_HasStartedSucceededFailed verifies the surface of
// the returned monitor without requiring a live MongoDB instance.
// Integration coverage lives in the examples + compose stack.
func TestCommandMonitor_HasStartedSucceededFailed(t *testing.T) {
	m := boengmongo.CommandMonitor()
	if m == nil {
		t.Fatal("CommandMonitor returned nil")
	}
	if m.Started == nil || m.Succeeded == nil || m.Failed == nil {
		t.Fatalf("missing callback(s): started=%v succeeded=%v failed=%v",
			m.Started != nil, m.Succeeded != nil, m.Failed != nil)
	}
}

// TestCommandMonitor_HandlesMissingPair makes sure a Succeeded/Failed
// event for an unknown request ID doesn't blow up.
func TestCommandMonitor_HandlesMissingPair(t *testing.T) {
	m := boengmongo.CommandMonitor()
	// Calling Succeeded without a prior Started for that RequestID
	// must be a no-op, not a panic.
	m.Succeeded(nil, &event.CommandSucceededEvent{
		CommandFinishedEvent: event.CommandFinishedEvent{RequestID: 9999},
	})
	m.Failed(nil, &event.CommandFailedEvent{
		CommandFinishedEvent: event.CommandFinishedEvent{RequestID: 9999},
		Failure:              "missing",
	})
}
