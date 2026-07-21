// Package boengmongo instruments go.mongodb.org/mongo-driver with
// boeng's Operation Runtime. Each MongoDB command becomes a boeng
// operation; duration and error are recorded automatically.
//
// Apply via the driver's option:
//
//	opts := options.Client().
//	    ApplyURI(uri).
//	    SetMonitor(boengmongo.CommandMonitor())
//	client, err := mongo.Connect(ctx, opts)
//
// Business code never imports OpenTelemetry; the monitor handles the
// trace/log/metric correlation.
package boengmongo

import (
	"context"
	"errors"
	"sync"

	"go.mongodb.org/mongo-driver/event"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/boeng"
)

// CommandMonitor returns a *event.CommandMonitor that opens a boeng
// operation per MongoDB command. The operation name is
// "mongo.<command_name>" (e.g. mongo.find, mongo.insert, mongo.update).
//
// The monitor stores per-request bookkeeping in memory keyed by
// MongoDB's RequestID so it can correlate Started/Succeeded/Failed
// events. Memory usage is bounded by the number of in-flight commands.
func CommandMonitor() *event.CommandMonitor {
	m := &monitor{inflight: map[int64]*inflight{}}
	return &event.CommandMonitor{
		Started:   m.started,
		Succeeded: m.succeeded,
		Failed:    m.failed,
	}
}

type inflight struct {
	op *boeng.Op
}

type monitor struct {
	mu       sync.Mutex
	inflight map[int64]*inflight
}

type commandFields struct {
	Command    string
	Collection string
	Database   string
}

func (c commandFields) LogFields() map[string]any {
	return map[string]any{
		"db.system":     "mongodb",
		"db.command":    c.Command,
		"db.collection": c.Collection,
		"db.name":       c.Database,
	}
}

func (m *monitor) started(ctx context.Context, e *event.CommandStartedEvent) {
	_, op := boeng.EnterCtx(ctx, "mongo."+e.CommandName, commandFields{
		Command:    e.CommandName,
		Collection: collectionFrom(e),
		Database:   e.DatabaseName,
	})
	m.mu.Lock()
	m.inflight[e.RequestID] = &inflight{op: op}
	m.mu.Unlock()
}

func (m *monitor) succeeded(_ context.Context, e *event.CommandSucceededEvent) {
	m.close(e.RequestID, nil)
}

func (m *monitor) failed(_ context.Context, e *event.CommandFailedEvent) {
	m.close(e.RequestID, errors.New(e.Failure))
}

func (m *monitor) close(requestID int64, err error) {
	m.mu.Lock()
	state, ok := m.inflight[requestID]
	if ok {
		delete(m.inflight, requestID)
	}
	m.mu.Unlock()
	if !ok {
		return
	}
	if err != nil {
		state.op.Fail(err)
	}
	state.op.Close()
}

// collectionFrom looks up the collection field from the command BSON.
// Mongo's CommandStartedEvent.Command is a raw bson.Raw — we extract
// best-effort. For commands that don't have a collection (e.g. ping),
// returns "".
func collectionFrom(e *event.CommandStartedEvent) string {
	if e.Command == nil {
		return ""
	}
	// Common commands name the collection in the field matching the
	// command name: {"find": "users", ...}, {"insert": "orders", ...}.
	v, err := e.Command.LookupErr(e.CommandName)
	if err != nil {
		return ""
	}
	if s, ok := v.StringValueOK(); ok {
		return s
	}
	return ""
}
