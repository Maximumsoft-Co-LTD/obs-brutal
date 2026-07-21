package boeng

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/Maximumsoft-Co-LTD/obs-brutal/internal/logtrc"
)

type ctxKey int

const keyOpState ctxKey = 1

type opState struct {
	mu    sync.RWMutex
	log   logtrc.LogBrt
	span  trace.Span
	start time.Time
	name  string
}

func (s *opState) logger() logtrc.LogBrt {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.log
}

func stateFrom(ctx context.Context) *opState {
	if ctx == nil {
		return nil
	}
	if v, ok := ctx.Value(keyOpState).(*opState); ok {
		return v
	}
	return nil
}

// setField mutates the current op's logger so the eventual completion log sees
// the new field. Unexported on purpose — Run/Emit/Log are the public surface;
// this exists for in-package wrappers like the Gin middleware.
func setField(ctx context.Context, key string, value any) {
	if s := stateFrom(ctx); s != nil {
		s.mu.Lock()
		s.log = s.log.F(key, value)
		s.mu.Unlock()
	}
}

// L returns the op-scoped logger from ctx, falling back to the package default.
// Most callers don't need this — Run/Emit/Log cover the common cases.
func L(ctx context.Context) logtrc.LogBrt {
	if s := stateFrom(ctx); s != nil {
		return s.logger()
	}
	if d := D(); d != nil {
		return d.log
	}
	return logtrc.NewDefault()
}

// Run wraps fn in one observability scope. It:
//   - opens an OTel span named name (parent = ctx span)
//   - extracts fields from subject (Loggable, map, or reflected struct)
//   - emits a DEBUG "<name> started" log + INFO/ERROR completion log with duration_ms
//   - records error + status on the span if fn returns one
//   - recovers panics, turns them into errors, and still completes the span
//   - records per-op metrics: <op>_total, <op>_duration_ms, <op>_error_total,
//     <op>_panic_total — labelled only with allowlisted subject keys
//     (Config.MetricLabels) plus service/env from Init
func Run(ctx context.Context, name string, subject any, fn func(context.Context) error) (err error) {
	ctx, finish := startOp(ctx, name, subject)
	var rec any
	defer func() {
		if rec != nil {
			panic(rec)
		}
	}()
	defer func() {
		panicked := false
		if r := recover(); r != nil {
			err = panicErr(r)
			panicked = true
			rec = r
		}
		finish(err, panicked)
	}()
	err = fn(ctx)
	return
}

// RunR is the generic variant of Run for functions that return (T, error).
func RunR[T any](ctx context.Context, name string, subject any, fn func(context.Context) (T, error)) (out T, err error) {
	ctx, finish := startOp(ctx, name, subject)
	var rec any
	defer func() {
		if rec != nil {
			panic(rec)
		}
	}()
	defer func() {
		panicked := false
		if r := recover(); r != nil {
			err = panicErr(r)
			panicked = true
			rec = r
		}
		finish(err, panicked)
	}()
	out, err = fn(ctx)
	return
}

// Emit records a one-shot domain event inside the current operation scope:
// a single structured log line, an event marker on the operation's trace,
// and a counter bump. Use for stage markers like "validated", "row_inserted",
// or for fire-and-forget events outside any operation.
//
// Inside a Run / Enter scope, prefer op.Emit which is the same call without
// needing ctx in hand.
func Emit(ctx context.Context, name string, subject any) {
	fields := extractFields(subject)
	fields["event"] = name
	if s := stateFrom(ctx); s != nil && s.span != nil {
		s.span.AddEvent(name, trace.WithAttributes(mapToAttrs(fields)...))
	}
	recordEvent(ctx, name, metricLabels(subject))
	L(ctx).Fs(fields).Info(name)
}

// logInScope writes an INFO line in the current operation's logger scope.
// Internal helper for op.Log — not exported. Manual logging belongs on the
// operation handle, not as a top-level verb (90% of useful log lines come
// out of Run/Enter automatically; manual logs are the 10% escape hatch).
func logInScope(ctx context.Context, msg string, subjects []any) {
	l := L(ctx)
	for _, s := range subjects {
		l = l.Fs(extractFields(s))
	}
	l.Info(msg)
}

func startOp(ctx context.Context, name string, subject any) (context.Context, func(error, bool)) {
	start := time.Now()

	d := D()
	var span trace.Span
	if d != nil && d.provider != nil {
		ctx, span = d.provider.Start(ctx, name)
	} else {
		span = noopSpan{}
	}

	fields := map[string]any{"op": name}
	maps.Copy(fields, extractFields(subject))

	if span.IsRecording() {
		span.SetAttributes(mapToAttrs(fields)...)
	}

	base := L(ctx).Fs(fields)
	if sc := span.SpanContext(); sc.IsValid() {
		base = base.TraceID(sc.TraceID().String()).F("span_id", sc.SpanID().String())
	}
	base.Debug(name + " started")

	state := &opState{log: base, span: span, start: start, name: name}
	ctx = context.WithValue(ctx, keyOpState, state)
	labels := metricLabels(subject)

	return ctx, func(err error, panicked bool) {
		dur := time.Since(start)
		l := state.logger().F("duration_ms", dur.Milliseconds())
		if err != nil {
			state.span.RecordError(err)
			state.span.SetStatus(codes.Error, err.Error())
			l.WithError(err).Error(state.name + " failed")
		} else {
			l.Info(state.name + " completed")
		}
		recordOp(ctx, state.name, dur, err != nil, panicked, labels)
		state.span.End()
	}
}

func panicErr(r any) error {
	if e, ok := r.(error); ok {
		return fmt.Errorf("panic: %w", e)
	}
	return errors.New("panic: " + fmt.Sprint(r))
}

func mapToAttrs(fields map[string]any) []attribute.KeyValue {
	out := make([]attribute.KeyValue, 0, len(fields))
	for k, v := range fields {
		out = append(out, attribute.String(k, fmt.Sprintf("%v", v)))
	}
	return out
}

