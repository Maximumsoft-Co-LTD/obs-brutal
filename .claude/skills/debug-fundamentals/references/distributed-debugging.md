# Distributed-system debugging

Single-process bugs are about *what* went wrong. Distributed bugs are about *when*, *where*, and *in what order*. The hardest production bugs almost all live here, and they share a small set of recurring shapes. Recognizing the shape early shortcuts the investigation by hours.

## The first move: correlate

Before anything else, get a **correlation id** through the system. A request id, a trace id, a job id — anything that lets you say "show me everything that touched this one request, across all services."

If the system already has one (X-Request-Id, OpenTelemetry traceparent, etc.), use it. If it doesn't, add one. Cross-service debugging without correlation is reading nine random log streams in parallel and squinting; correlation collapses it to one timeline.

What good correlation looks like in practice:
- Generated at the edge (or accepted from upstream if present and trusted).
- Propagated on every outbound call (HTTP header, message header, RPC metadata).
- Logged on every log line emitted while serving that request.
- Surfaced to the user in error responses ("If you contact support, include request id `req_abc123`").

If you're investigating an incident and there's no correlation id, your first fix may be to add one — even mid-investigation. The current incident may not benefit, but the next one will.

## The common shapes

Most distributed bugs are one of these. When investigating, run through them as a checklist — the symptom often matches one shape immediately.

### Duplicate delivery

A message was delivered more than once. The consumer ran twice. The user got charged twice, the row got inserted twice, the email got sent twice.

Tells:
- Two log lines with the same message id and different timestamps.
- A `UNIQUE` constraint violation in the DB that "shouldn't be possible."
- A counter that jumps by 2 when it should jump by 1.

Cause: almost always at-least-once delivery semantics meeting a non-idempotent consumer. See [[queue-fundamentals]] principle 3.

Fix shape: make the consumer idempotent — `INSERT ... ON CONFLICT DO NOTHING`, dedup table on a natural key, idempotency-key column. The fix doesn't belong in the producer; almost no broker offers true exactly-once.

### Lost message

A message was sent (or "appeared to be sent") and never processed.

Tells:
- "I sent it, the API returned 200, but no consumer log line."
- A row in the source DB without the matching row downstream.
- Counter on producer side > counter on consumer side.

Causes, in rough order of likelihood:
1. **The send succeeded in the app, failed at the broker, and the app didn't check.** Look for un-awaited promises, ignored error returns, `_, _ = client.Send(...)`.
2. **Dual-write inconsistency.** The DB write succeeded but the broker send failed (or vice versa), and the app didn't compensate. See `references/outbox.md` in queue-fundamentals.
3. **Consumer ack-on-receive.** The consumer acknowledged before processing, then crashed. The broker considers it delivered; the work didn't happen.
4. **Filtering / routing.** The message landed on a topic / partition / queue that no one was subscribed to.

Fix shape: outbox pattern, or ack-after-process, or fix the subscription. Don't add a retry loop without finding which of these is happening.

### Out-of-order processing

Events were processed in the wrong order. Update arrived before create. Cancel arrived before charge.

Tells:
- "The state went backwards."
- A "created" event arrives after an "updated" event for the same entity.
- Latency-sensitive flows where retries scramble ordering.

Causes:
- The broker doesn't guarantee global order (almost none do). It may guarantee per-partition or per-queue order if you use the right partition key.
- Retries with backoff: message 1 fails and retries; message 2 succeeds; message 1 finally lands second.
- Multiple producers; no shared clock.

Fix shape: don't rely on order across partitions/queues. Either (a) partition by the entity that needs ordering (user id, order id) so all events for it go to the same partition, or (b) make the consumer order-independent — process by event timestamp, or LWW with versioning, or model state as a fold of all events.

### Race condition (read-modify-write)

Two requests read the same value, both modify it, both write back. The second write clobbers the first.

Tells:
- "Sometimes the count is wrong by one."
- "We charged the user twice for the same item."
- Anything involving `count = count + 1` in application code rather than `count = count + 1` in SQL.

Causes: read and write are not atomic, and concurrent callers interleave between them.

Fix shape:
- Push the operation into the database as a single statement: `UPDATE accounts SET balance = balance - 100 WHERE id = ?`.
- Or use a transaction with `SELECT ... FOR UPDATE`.
- Or optimistic locking: `UPDATE ... WHERE version = ?` and check `rowcount`.
- Or move to a queue with single-consumer-per-key partitioning.

See [[database-fundamentals]] for the transaction-level patterns and [[queue-fundamentals]] for the partition-by-key pattern.

### Time-skew and clock bugs

Two hosts disagree on what time it is. Or one host's clock jumped (NTP correction, VM pause, daylight saving).

Tells:
- Timestamps go backwards in a log.
- A token "expires" before it was issued.
- A scheduled job runs twice or zero times around DST transitions.
- "It works in UTC but breaks in production where the host is on local time."

Fixes:
- Use UTC everywhere internally. Convert only at the user-facing edge.
- Use monotonic clocks for *durations*, wall clocks only for *points in time*.
- Don't compare times across hosts without a tolerance.
- For "expires at" use a duration from issue time, not an absolute time, when possible.

### Retry storms / cascading failure

One service degrades. Its clients retry. The retries amplify the load. The service collapses; its clients now also degrade; their clients retry; the whole system topples.

Tells:
- Latency on service B spikes; CPU on services A, C, D all spike at the same time.
- Error rates on multiple services climb together with no shared deploy.
- "Recovery" doesn't happen even after the root cause is fixed.

Fixes:
- Bounded retries with exponential backoff *and jitter*. Three retries, not unlimited.
- Circuit breakers — when a downstream is failing, fail fast for a window rather than piling on.
- Backpressure — when overloaded, shed load; don't queue it indefinitely.
- Don't retry on 4xx responses; they won't succeed.

### Partial failure / split brain

The system is in an inconsistent state because one half thinks one thing happened, the other half thinks something else.

Tells:
- Two replicas disagree about who's primary.
- An order shows `status=charged` in one service and `status=pending` in another.
- A flag is true in cache, false in the DB.

Fixes are case-by-case but the discipline is: have a single source of truth, and treat the others as derived views that must reconcile. When they drift, the source wins and the others rebuild.

## Investigation flow

When facing a distributed bug:

1. **Get the correlation id of one failing case.** One specific incident, one request id, one timestamp.
2. **Pull every log line carrying that id across all services.** Sort by timestamp. This is your timeline.
3. **Walk the timeline forward.** At each step, ask: is this what I expected at this moment? The first surprise is where to dig.
4. **Match the surprise to a shape above.** Duplicate? Out of order? Race? Lost? Time skew? Retry storm?
5. **Form one hypothesis. One.** Then design the smallest experiment that distinguishes "shape X" from "shape Y." Run it.

Resist the urge to debug *all* failing requests at once. The shapes don't co-occur cleanly; you'll confuse signals from two unrelated bugs. Pick one, finish it, move on.

## Anti-patterns

- **"It's a flake, just retry."** Almost never true in a real distributed system. Flakes are bugs; retries are the cope.
- **Adding retries because something failed.** First understand *why* it failed. Retrying a non-idempotent operation is how duplicate-charge incidents are born.
- **Looking at a single service's logs.** The bug is at the boundary; the boundary is in two services.
- **Trusting "the API returned 200."** The response says "I accepted your request." It doesn't say "the work happened." Check the side effect, not the response code.
- **Debugging without a correlation id.** Stop; add one; resume.

## Relation to other skills

- [[queue-fundamentals]] — most of the shapes above (duplicate, lost, out-of-order, retry storm) are queue concerns. The fix lives there; this skill helps you recognize which one you're hitting.
- [[database-fundamentals]] — race conditions and partial failures often want a transaction or a constraint as the real fix. Don't paper over them in the app layer.
- [[hexagonal-backend]] — when the bug crosses an adapter boundary, the right test is at the boundary itself. The adapter is usually where serialization, retries, and timeouts live.
