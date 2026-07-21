package boeng

import (
	"context"
	"sync/atomic"
)

// Op is the imperative handle returned by Enter / EnterCtx. It exposes the
// same observability primitives Run provides (log, event, error capture,
// span close, metrics) but in a form that fits legacy code paths where you
// can't wrap the function body in a closure.
//
// Always pair Enter/EnterCtx with `defer op.Close()`. Use Fail or CloseWith
// to record an error before the span ends.
type Op struct {
	ctx    context.Context
	state  *opState
	finish func(error, bool)
	err    atomic.Value // holds error
	closed atomic.Bool
}

// Enter starts an operation without a caller context. Use this in legacy
// code that doesn't propagate context.Context. A fresh context is created
// internally; child operations inside the same call tree should use the ctx
// returned by op.Context() to keep trace lineage.
//
//	op := boeng.Enter("create_user", user)
//	defer op.Close()
//	op.Log(user, "validating")
//	if err := repo.Save(user); err != nil { op.Fail(err); return err }
//
// subject is optional; pass at most one. If subject implements Loggable,
// its LogFields() determines the attached log/span fields; otherwise the
// reflection fallback applies.
func Enter(name string, subject ...any) *Op {
	return enterFromCtx(context.Background(), name, subject...)
}

// EnterCtx is the context-aware sibling of Enter. The returned ctx carries
// the new span so nested Enter/EnterCtx/Run calls become child spans.
//
//	ctx, op := boeng.EnterCtx(ctx, "create_user", user)
//	defer op.Close()
//	doStuff(ctx)        // any further boeng.* calls under this ctx nest in
func EnterCtx(ctx context.Context, name string, subject ...any) (context.Context, *Op) {
	op := enterFromCtx(ctx, name, subject...)
	return op.ctx, op
}

func enterFromCtx(ctx context.Context, name string, subject ...any) *Op {
	var subj any
	if len(subject) > 0 {
		subj = subject[0]
	}
	newCtx, finish := startOp(ctx, name, subj)
	return &Op{ctx: newCtx, state: stateFrom(newCtx), finish: finish}
}

// Context returns the operation-scoped context. Pass it into nested calls
// (other Enter/Run invocations, downstream libraries) so spans nest and
// the operation logger propagates.
func (o *Op) Context() context.Context {
	if o == nil {
		return context.Background()
	}
	return o.ctx
}

// Log writes an INFO line inside the operation's scope. The first argument
// is the message; any further arguments are subjects whose fields are merged
// into the log entry (via Loggable or reflection). Use for ad-hoc context
// inside an operation — stage notes, decision points, cache hits/misses —
// that doesn't deserve its own named event.
//
//	op.Log("cache miss")
//	op.Log("validated", usr)
//	op.Log("retry", attemptInfo, usr)
func (o *Op) Log(msg string, subjects ...any) {
	if o == nil {
		return
	}
	logInScope(o.ctx, msg, subjects)
}

// Emit records a one-shot domain event inside the operation's scope: a
// structured log line, an event marker on the trace, and a counter bump.
// Use for stage markers like "validated", "row_inserted", "publish_failed".
func (o *Op) Emit(name string, subject any) {
	if o == nil {
		return
	}
	Emit(o.ctx, name, subject)
}

// Success marks the operation as having completed successfully. It is the
// explicit counterpart of Fail: calling Close (or CloseWith with nil error)
// already implies success, so Success exists purely for code that wants the
// happy path to be visually explicit.
//
//	if err := charge(); err != nil { op.Fail(err); return err }
//	op.Success()
//
// Calling Success after Fail is a no-op (the recorded error wins). Calling
// Success multiple times is also a no-op.
func (o *Op) Success() {
	if o == nil {
		return
	}
	// Clear any prior Fail only if nothing was stored — Success must not
	// overwrite a real error.
	if o.loadErr() != nil {
		return
	}
	// No-op otherwise; Close handles the actual completion log.
}

// Step opens a child operation inside this op and runs fn through the full
// observability pipeline — child span, start/completion log, duration
// histogram, per-step metrics (<name>_total / _duration_ms / _error_total /
// _panic_total), panic recovery. Use it to instrument the phases of a
// larger operation without breaking up the function:
//
//	op := boeng.Enter(ctx, "create_user", usr)
//	defer op.Close()
//	if err := op.Step("validate", func() error { return validate(usr) }); err != nil {
//	    return err
//	}
//	if err := op.Step("insert_db", func() error { return repo.Insert(usr) }); err != nil {
//	    return err
//	}
//
// Behavior:
//   - fn returning a non-nil error marks the step failed AND records the
//     same error on the parent op (so closing the parent without explicit
//     Fail still logs the parent at ERROR).
//   - A panic inside fn is recorded on the step's span/metrics, propagated
//     to the parent op's error, and re-raised so normal unwinding occurs.
//   - On a nil receiver, Step degrades to just calling fn().
func (o *Op) Step(name string, fn func() error) (err error) {
	if o == nil {
		return fn()
	}
	_, finish := startOp(o.ctx, name, nil)
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
		if err != nil {
			o.err.Store(errBox{err: err})
		}
		finish(err, panicked)
	}()
	err = fn()
	return
}

// Fail marks the op as failed without closing it. The provided error
// surfaces in the completion log (level ERROR) and on the span (RecordError
// + Status=Error). Calling Fail multiple times keeps the latest non-nil
// error. The op is still closed by Close / CloseWith.
func (o *Op) Fail(err error) {
	if o == nil || err == nil {
		return
	}
	o.err.Store(errBox{err: err})
}

// Close finishes the op: stops the span, emits the completion log
// (INFO when no error, ERROR when Fail was called), bumps duration +
// counters. A panic in flight is recovered, recorded as the op's error,
// and then re-raised so unwinding continues normally.
//
// Safe to call on a nil *Op (no-op). Safe to call twice (subsequent calls
// are no-ops).
func (o *Op) Close() {
	if o == nil {
		return
	}
	if !o.closed.CompareAndSwap(false, true) {
		return
	}
	if r := recover(); r != nil {
		e := panicErr(r)
		o.finish(e, true)
		panic(r)
	}
	o.finish(o.loadErr(), false)
}

// CloseWith closes the op and lets the caller pin the outcome to a named
// return error in one line:
//
//	func createUser(ctx context.Context) (err error) {
//	    ctx, op := boeng.EnterCtx(ctx, "create_user")
//	    defer op.CloseWith(&err)
//	    ...
//	}
//
// If a panic is in flight, *errp is overwritten with the panic-derived
// error before the panic re-raises.
func (o *Op) CloseWith(errp *error) {
	if o == nil {
		return
	}
	if !o.closed.CompareAndSwap(false, true) {
		return
	}
	if r := recover(); r != nil {
		e := panicErr(r)
		if errp != nil {
			*errp = e
		}
		o.finish(e, true)
		panic(r)
	}
	var err error
	if errp != nil {
		err = *errp
	}
	if err == nil {
		err = o.loadErr()
	}
	o.finish(err, false)
}

type errBox struct{ err error }

func (o *Op) loadErr() error {
	if v := o.err.Load(); v != nil {
		if b, ok := v.(errBox); ok {
			return b.err
		}
	}
	return nil
}
