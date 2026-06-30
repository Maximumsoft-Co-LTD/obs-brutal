# Instrumentation

Instrumentation is how you turn a system that "doesn't work" into a system that *tells you* what it's doing. Most stuck debugging sessions are stuck because the engineer is reasoning about what the code *should* do rather than what it *did*. A well-placed log, breakpoint, or trace replaces an hour of theorizing with a fact.

## The hierarchy

Use the lightest tool that gives you the answer:

1. **Print / log statements** — universal, dumb, fast. Always available. Almost always enough.
2. **Structured logs** — when you need to filter or correlate across requests.
3. **Debuggers / breakpoints** — when you want to walk through state interactively or inspect deep objects.
4. **Tracing (distributed tracing, APM)** — when the bug crosses service boundaries.
5. **System-level tracing** — `strace`, `dtrace`, `eBPF`, `tcpdump` — when the bug is at the OS or network layer and the application can't see it.

Don't reach for the heavy tool first. A `console.log` solves 80% of bugs; a debugger another 15%; eBPF is for the last 5% where the application itself is lying to you.

## Print debugging — done well

"Print debugging" gets a bad rap because it's often done badly. Done well, it's almost always the fastest path.

### What to print

For each suspect variable, print these in roughly this order:

1. **Existence.** `console.log('got here')` or `print('after fetch')` — proves the line executed.
2. **Identity.** `console.log('handler called for user', user.id)` — proves *which* invocation you're watching.
3. **Type.** `console.log(typeof x, x?.constructor?.name, x)` — most surprise bugs are "I thought this was a number, it's a string." The runtime type beats the declared type.
4. **Shape.** `console.log(Object.keys(x))` or `pp x.attributes` — what does the object *actually* have on it?
5. **Value.** The thing itself. Print it with structure (`JSON.stringify(x, null, 2)`, `pprint`, `%#v`) so nested objects don't render as `[Object]`.

The order matters because most "I expected this value" bugs are actually "I expected this *type* or this *shape*." Print the meta-info before you print the data and the bug often outs itself.

### Where to put them

Put logs at the **boundaries** of the suspect region: where data enters the function, where it leaves, and at any branch. If the boundary values are right, the bug is inside the boundary. If they're wrong, it's outside. That's the bisection move from `references/bisection.md` in miniature.

### Make logs distinguishable

A wall of `value: 12` lines tells you nothing. Tag every log:

```ts
console.log('[chargeOrder] before validate:', { orderId: order.id, total: order.total })
console.log('[chargeOrder] after validate:', { orderId: order.id, total: order.total })
```

When you run the repro and 200 lines fly by, you can `grep '[chargeOrder]'` and see only the relevant ones.

### Clean up

Every debug log is technical debt. After the bug is fixed, either:
- Delete them.
- Or, if they encode something genuinely useful (a structured event at a key boundary), promote them to proper logs with the project's logger and a sensible log level.

Don't leave naked `console.log('here 4')` in the codebase. Future-you will hate present-you.

## Structured logging

Once you graduate from one-off prints, structured logs are the next tool. Every log line is a JSON object with consistent fields:

```json
{"ts": "2026-05-13T14:32:01Z", "level": "info", "event": "order.charge.start",
 "request_id": "req_abc", "order_id": "ord_123", "user_id": "u_999",
 "total_cents": 1000, "currency": "USD"}
```

What this buys you during debugging:

- **Filter by `request_id`** to see one request's entire path through the system.
- **Filter by `event`** to see every charge attempt across all users.
- **Pivot in your log tool** (Datadog, Honeycomb, Cloud Logging, `jq`) — "show me all `order.charge.start` events where `total_cents == 0`."
- **Carry context across boundaries.** When the next service logs the same `request_id`, you can stitch the path.

If the codebase already has structured logging, learn the conventions (the field names, the levels, the propagation) before you add anything. If it doesn't, and you're debugging something cross-cutting, that may be the right fix.

### Log levels during debugging

- Default app log level in dev: bump to `debug` or `trace` for the suspect components.
- In prod: be careful. A debug log on a hot path can swamp the log pipeline or leak PII. Use sampling or temporary feature flags.
- In tests: many test frameworks suppress logs. Pass through or unbuffer when chasing a test bug.

## Debuggers / breakpoints

When the state is rich and the values are nested, a debugger beats a hundred prints.

- **Conditional breakpoints** — "break only when `order.id === 'ord_123'`." Crucial when the bug is one bad request in a hot path.
- **Logpoints** — most modern debuggers can print a value at a line without stopping. Like a print, but without recompiling.
- **Watch expressions** — keep an eye on `user.permissions.length` as you step.
- **Step into / over / out** — know the difference. Step *over* when you trust the callee; step *into* when you don't.

Debugger usage in agentic contexts is awkward (no interactive UI). When you can't use a debugger, fall back to dense, structured prints at every step the debugger would have shown.

## Tracing

For multi-service bugs, distributed tracing (OpenTelemetry, Jaeger, Tempo, Honeycomb, Datadog APM) is the only sane tool. The trace is a tree of spans, each with timing and attributes, propagated across service boundaries via trace headers.

What it lets you see:

- The exact path a request took across services.
- Which span was slow (the cause of a tail-latency bug).
- Which span errored (and what its attributes were).
- Where in the chain a value changed shape.

When a span is missing a useful attribute (the order id, the user id, the size of the payload), add it. Trace attributes during a debugging session are usually the lowest-effort, highest-value instrumentation in a distributed system.

If the system has no tracing and a distributed bug is wasting days, *the right fix is to add tracing*. It pays back on the next incident, not just this one.

## System-level tracing

When the app says "it succeeded" but the user sees nothing, the application's testimony is unreliable. Drop a layer:

- **`strace -p <pid>` / `dtruss`** — every system call the process makes. Slow but ground-truth. Great for "is the process even opening the file?" / "is it making the network call I think it is?"
- **`tcpdump` / Wireshark** — what bytes are actually going on the wire. The only honest answer when TLS, HTTP/2, or a proxy is misbehaving.
- **`eBPF` tools** (`bpftrace`, `bcc-tools`) — production-safe system tracing. `tcpconnect`, `opensnoop`, `execsnoop`, `biolatency`, etc.
- **`/proc/<pid>/`** — fds, maps, status. Useful when a process is doing something unexpected.
- **`lsof`, `netstat`, `ss`** — what's the process holding open, what's it connected to, what's it listening on.

These are heavyweight. Reach for them when the bug is *below* the application layer: weird timeouts that don't show in app logs, "file not found" when the file is there, DNS resolution misery, mysterious EAGAIN/EINTR loops.

## Production instrumentation

Some bugs only happen in production. Your only path is to make production carry the evidence on the next occurrence:

- Add a log at the failure point with full context (sanitized).
- Add a metric counter on the failure shape — `order.charge.zero_total` — so you can see frequency.
- Add a trace attribute so the next failing trace has the values you need.
- Deploy. Wait. The next occurrence is no longer wasted.

Resist the urge to deploy a "fix" instead of an instrumented build. A fix-without-repro is a guess; an instrumented build is a hypothesis-collector that pays off in hours.

## Anti-patterns

- **`console.log(x)` with no label.** Three of those in a hot loop and you can't tell which is which.
- **Logging only on the happy path.** The interesting log lines are usually in the error branch.
- **Logging *after* the crash.** The line after the throw doesn't execute. Put the log *before*.
- **Reading values in a debugger by mousing over them, but never writing them down.** The bug is often the pattern across calls, not one value. Capture; don't just look.
- **Removing instrumentation before you've confirmed the fix.** Keep the logs in until the regression test passes; then clean up.
