// Package boeng is an Operation Runtime for Go.
//
// Developers describe business operations once. From that single declaration,
// boeng's runtime dispatches the work through a configurable pipeline —
// logger, tracer, metrics, panic recovery, correlation, sampling, security,
// exporter, sink — and produces consistent observability output across all
// of them. Application code never touches OpenTelemetry concepts.
//
// # Architecture
//
//	Business code
//	    │ describes
//	    ▼
//	Operation  ── name + subject + outcome
//	    │ dispatched into
//	    ▼
//	Pipeline
//	    ├── Logger        (structured log entries)
//	    ├── Trace         (spans, correlation, propagation)
//	    ├── Metric        (per-op counters + duration histograms)
//	    ├── Recover       (panic capture + re-raise)
//	    ├── Sampling      (configurable, post-MVP)
//	    ├── Security      (PII masking)
//	    ├── Exporter      (OTLP, optional)
//	    └── Sink          (stdout JSON, Loki, file, alerts)
//
// Future additions to the pipeline (audit, profiling, anomaly detection) do
// not change the public API. Application code stays unchanged.
//
// # Four verbs
//
// Top-level API is intentionally tiny:
//
//	boeng.Init(cfg)                              // process startup, once
//	boeng.Run(ctx, name, subject, fn)            // wrap a function in an operation
//	boeng.Enter(name, subject) / EnterCtx(...)   // open an operation from inside a function
//	boeng.Emit(ctx, name, subject)               // standalone domain event
//
// Run vs Enter is intent, not technology:
//
//   - Run when you want to wrap an entire function body in one operation.
//   - Enter when you're already inside a function and you want to open an
//     operation (and possibly steps within it) by hand.
//
// Methods on the operation handle:
//
//	op.Step(name, fn)        // child operation through the full pipeline
//	op.Log(msg, subjects...) // ad-hoc log line in the op's scope
//	op.Emit(name, subject)   // domain event in the op's scope
//	op.Fail(err)             // mark failed; Close still required
//	op.Success()             // explicit happy-path marker (optional)
//	op.Close() / CloseWith(*error)
//	op.Context()
//
// # Three usage modes
//
// Greenfield — wrap a function body:
//
//	err := boeng.Run(ctx, "create_user", usr, func(ctx context.Context) error {
//	    return repo.Create(ctx, usr)
//	})
//
// Context-aware imperative — keep a handle, optionally compose Steps:
//
//	func createUser(ctx context.Context, usr User) (err error) {
//	    ctx, op := boeng.EnterCtx(ctx, "create_user", usr)
//	    defer op.CloseWith(&err)
//	    if err = op.Step("validate", func() error { return validate(usr) }); err != nil {
//	        return err
//	    }
//	    return op.Step("insert_db", func() error { return repo.Insert(ctx, usr) })
//	}
//
// Legacy — no function-signature change required:
//
//	op := boeng.Enter("create_user", usr)
//	defer op.Close()
//	_ = op.Step("validate", validate)
//	_ = op.Step("insert_db", insertDB)
//
// # Subjects
//
// A subject is the business data an operation acts on. Pass a struct,
// pointer to struct, map[string]any, or nil. Resolution priority:
//
//  1. nil  → no fields attached
//  2. implements Loggable → use LogFields()
//  3. map[string]any → used as-is
//  4. struct → reflect exported fields (snake_case, skip zeros, auto-mask
//     sensitive keys)
//
// Implement Loggable on domain types so observability lives in the domain
// layer:
//
//	func (u User) LogFields() map[string]any {
//	    return map[string]any{
//	        "user_id": u.UserID,
//	        "email":   boeng.MaskEmail(u.Email),
//	    }
//	}
//
// # Metrics and cardinality
//
// By default every operation auto-emits four metrics: <op>_total,
// <op>_duration_ms, <op>_error_total, <op>_panic_total. Step does the
// same for child operations. Config.MetricSchema = LabeledMetrics
// switches to one fixed family with the operation as a label
// (boeng_operation_duration_seconds{op,outcome}, boeng_events_total{event})
// so PromQL can aggregate across operations; BothMetrics emits both.
// The metric label set is fail-closed: only keys in Config.MetricLabels
// (plus service + env) are promoted to labels. Subject fields like
// user_id stay in logs and traces but are never silently elevated to
// metric labels.
//
// # Log levels
//
// Config.Level filters everything. Config.QuietOps demotes the
// "<op> completed" line to DEBUG (failures stay ERROR); Config.EmitLevel
// raises the level of Emit event lines (INFO by default).
//
// # Where telemetry goes
//
// Config.OTel set → boeng builds and installs its own exporting
// TracerProvider (http:// plaintext, https:// TLS). Config.OTel empty
// and the application already installed an SDK TracerProvider → boeng
// opens spans on it and touches nothing global. Otherwise, with
// OTEL_EXPORTER_OTLP_ENDPOINT in the environment boeng exports and lets
// the SDK read the env; with nothing at all it still opens spans (valid
// trace_id, W3C propagation) but exports nothing. An unreachable
// collector never panics; logs keep flowing to the configured sinks.
package boeng
