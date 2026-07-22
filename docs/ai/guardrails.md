# AI guardrails

Rules for any AI coding assistant editing Go code in a project that
imports `github.com/Maximumsoft-Co-LTD/obs-brutal/boeng`. Apply these verbatim; they keep the
Runtime Guarantees (G1–G7) intact.

These rules are the deep version of the "For AI coding assistants"
section in [`../../README.md`](../../README.md). When the two
disagree, the README wins (it is the user-facing summary); update this
file to match.

## Core thesis

> Application code declares an Operation.
> The runtime produces the observability.

If you find yourself about to write a `tracer.Start`, `defer span.End`,
`meter.Counter`, `slog.Info`, or `defer func(){ if r := recover(); … }`
ladder, **stop**. boeng already does that — you're writing it twice.

## DO

- **Wrap a function body in `boeng.Run`**:
  ```go
  err := boeng.Run(ctx, "create_user", user, func(ctx context.Context) error {
      return repo.Create(ctx, user)
  })
  ```
  The op name is snake_case. The subject is the business struct
  (`user`). No `Loggable` method is required — boeng reflects exported
  fields, converts to snake_case, skips zero values, auto-masks keys
  containing `password` / `token` / `secret` / `email` / `phone` / etc.

- **Use `boeng.Enter` / `EnterCtx` when you cannot wrap the body**:
  ```go
  func createUser(ctx context.Context, user User) (err error) {
      ctx, op := boeng.EnterCtx(ctx, "create_user", user)
      defer op.CloseWith(&err)
      // ... existing logic, unchanged ...
  }
  ```
  Signature is unchanged. Behaviour is unchanged. Observability is
  added. This is the **legacy migration pattern**.

- **Break a long operation into named `op.Step`s**:
  ```go
  if err = op.Step("validate", func() error { return validate(user) }); err != nil {
      return err
  }
  if err = op.Step("insert_db", func() error { return repo.Insert(ctx, user) }); err != nil {
      return err
  }
  ```
  Each step gets its own span, log, duration histogram, and metric
  family (`validate_total`, `validate_duration_ms`, etc.). A step that
  fails marks the parent op as failed automatically.

- **Use adapters for framework boundaries**:
  - HTTP server: `boenghttp.Middleware(handler)` or `boenghttp.Wrap("name", handler)`.
  - HTTP client: `&http.Client{Transport: boenghttp.Transport(nil)}`.
  - Gin: `r.Use(boenggin.Middleware())` then `boenggin.L(c)` for the request logger.
  - MongoDB: `options.Client().SetMonitor(boengmongo.CommandMonitor())`.
  - Redis: `client.AddHook(boengredis.Hook())`.
  - RabbitMQ: `boengrabbit.Publish(ctx, ch, exchange, key, msg)` /
    `boengrabbit.Consume("queue_name", handler)`.

- **Use `boeng.L(ctx).F("k", v).Info("msg")` only for ad-hoc mid-flow
  log lines**. Most logs are automatic via `Run`/`Enter`.

## DO NOT

- **Do not import any other logging library** in business code:
  - `log/slog`, `go.uber.org/zap`, `github.com/rs/zerolog`, `github.com/sirupsen/logrus`.
  boeng **is** the application-facing logger.

- **Do not import `go.opentelemetry.io/otel/...` in business code**.
  Tracer, Span, Propagator, Meter, Counter, Histogram, Status, RecordError
  — every one of those lives behind boeng. If you want one, you want a
  boeng verb.

- **Do not write observability ladders**:
  ```go
  // ALL OF THIS IS REDUNDANT WITH boeng.Run
  ctx, span := tracer.Start(ctx, "create_user")
  defer span.End()
  start := time.Now()
  defer func() {
      if r := recover(); r != nil {
          span.RecordError(fmt.Errorf("panic: %v", r))
          panic(r)
      }
      durationHist.Record(ctx, float64(time.Since(start).Milliseconds()))
  }()
  defer func() {
      if err != nil { totalErrors.Add(ctx, 1); span.SetStatus(codes.Error, err.Error()) }
  }()
  slog.InfoContext(ctx, "starting create_user", "user_id", user.ID)
  ```

- **Do not put high-cardinality fields in `Config.MetricLabels`**.
  Never `user_id`, `order_id`, `request_id`, `email`, IP, or free-form
  text. They belong in log lines and span attributes (cardinality is
  fine there). The metric label allowlist is fail-closed by design.

- **Do not call `boeng.SetSinkForTest` outside `*_test.go` files**.
  It is a test helper that disables real sinks; production callers
  silently lose all output.

- **Do not import `github.com/Maximumsoft-Co-LTD/obs-brutal/internal/...`** in any code that is not
  inside the same `internal/` tree. The whole `internal/` subtree is
  excluded from the v1.x semver contract — see
  [`../../COMPATIBILITY.md`](../../COMPATIBILITY.md). If a symbol you
  need is only reachable via `internal/`, that is the library's bug;
  open an issue.

- **Do not remove a public symbol** that appears in
  `boeng/api_freeze_test.go` without a documented major version bump.
  The test will fail; CI will fail; the PR will be rejected.

## When migrating existing code

The canonical transformations live as fixtures under
[`../../boeng/testdata/transform/`](../../boeng/testdata/transform/).
There are four canonical cases — pick the one whose shape matches the
function you're editing:

| Case          | Use when                                          |
| ------------- | ------------------------------------------------- |
| `simple`      | The function returns no error                     |
| `errret`      | The function returns `error` (or `(T, error)`)    |
| `panic_recover` | You explicitly want panic safety                |
| `http_handler` | The function is an `http.HandlerFunc`            |

Each fixture has a published LOC budget (3–5 added lines). The
`TestAITransformation_Fixtures` test enforces the budget; if your edit
needs more lines, you are probably doing something the four-verb API
already does for free.

## When in doubt

Re-read these two files in order:

1. [`../../boeng/README.md`](../../boeng/README.md) — full API surface
2. [`../../ARCHITECTURE.md`](../../ARCHITECTURE.md) — mental model

If after reading both the right boeng call is not obvious, that is
information: open an issue describing the use case. boeng's invariant
is "the right call is obvious"; gaps in that invariant are bugs in
boeng, not in your code.

## Open questions

None at the moment.

> Verified against `0d0a832` · 2026-07-22
