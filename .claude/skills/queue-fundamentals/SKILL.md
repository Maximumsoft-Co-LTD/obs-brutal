---
name: queue-fundamentals
description: Apply the cross-cutting fundamentals that govern any queue-based system — broker selection, delivery semantics, idempotent consumers, ack discipline, retries and dead-letter queues, ordering guarantees, and the outbox pattern for DB + broker writes. Use this skill BEFORE introducing, modifying, or debugging any queue, message broker, event stream, background job, async worker, or pub/sub topic — in any language, with any broker (SQS, RabbitMQ, Kafka, Redis Streams, NATS, Google Pub/Sub, BullMQ, Sidekiq, Celery, in-memory channels, etc.). Triggers on requests involving "queue", "message", "broker", "consumer", "producer", "worker", "job", "task", "event", "pub/sub", "publish", "subscribe", "stream", "topic", "partition", "DLQ", "dead-letter", "retry", "backoff", "fan-out", "outbox", "Kafka", "SQS", "RabbitMQ", "BullMQ", "Sidekiq", "Celery", "async processing", "background job", "event-driven", "redrive", or whenever async/decoupled processing comes up. Use it even when the user does not name a specific broker — the trigger is any queue-shaped problem. The skill provides 7 principles with worked examples (TypeScript and Go), plus reference files on broker selection, idempotency patterns, the outbox pattern, and operating queues in production. Skip only when using a List/Deque as a pure in-process data structure with no concurrency, persistence, or cross-process concerns.
---

# Queue Fundamentals

## Why this exists

Queues are the joints of a distributed system. Every async boundary — between a web request and a slow job, between two services, between a write and its downstream consumers — is a queue. Almost every "ghost in the machine" bug in a production backend traces back to the same handful of missed queue fundamentals: silent message loss, duplicate processing that double-charged a customer, an ordering assumption that held in dev and broke under load, a dead-letter queue that nobody set up so a single bad payload wedged the whole pipeline, a DB write that "succeeded" but the matching event never published.

This skill is a **pre-flight** for anything queue-shaped: read it before you put a queue in your system, not after the first incident. The principles are broker-agnostic — they apply equally to SQS, Kafka, RabbitMQ, BullMQ, an in-memory Go channel, or a hand-rolled job table in Postgres. The mechanics differ; the contract you have to think about does not.

Programs this skill sits next to:
- [[programming-fundamentals]] — fundamentals of the code that runs inside a consumer. Apply that first.
- [[database-fundamentals]] — the outbox table, dedup tables for idempotency, and DB-backed job tables are schema decisions with their own indexes, constraints, and transaction concerns. Apply when you're shaping those tables.
- [[hexagonal-backend]] — where the queue lives in the architecture (producers and consumers are adapters; the use case sees ports). The outbox pattern is mentioned there in passing; this skill expands on it.
- [[architecture-fundamentals]] — decides *whether* a given interaction belongs on a queue at all (sync vs async, its principle 3) and runs **before** this skill in the construction order; this skill operates the queue once that decision is made.
- [[ddd-strategic]] — draws the line between internal domain events (stay inside one bounded context) and cross-context integration events (the versioned contract that crosses a broker); this skill operates whichever event actually crosses the channel.

## The 7 principles

Each principle has a one-line rule, a *why*, and a worked example. Apply them in roughly this order — the early ones unblock the later ones.

---

### 1. Pick the right queue for the purpose

**Rule:** Decide what the queue is *for* before you decide which broker to use. Different purposes need different shapes; the wrong broker turns a normal feature into an operational nightmare.

**Why:** "We use Kafka" is not an architecture. A queue can be doing any of: smoothing bursts (buffer), decoupling services (async API), distributing work across a worker pool (task queue), fanning a single event out to many independent consumers (pub/sub), or storing a replayable history of events (log). Each purpose has a different natural fit, and forcing the wrong tool into the wrong role is where most queue pain comes from — Kafka used as a task queue, SQS used as an event log, an in-memory channel used to survive crashes.

**How to apply:**
- Name the purpose in one sentence before picking a broker. "Fan out user-signup events to N independent consumers" → log/pub-sub. "Run a slow image-resize off the request thread" → task queue. "Buffer a burst of webhooks so we don't drop any" → durable broker.
- Match the durability to the cost of loss. If losing a message is fine (metrics, best-effort notifications), an in-memory channel is OK. If losing a message means losing money, the queue must survive a broker crash and a consumer crash.
- Match the topology to who reads. Single worker pool draining a queue (point-to-point) is a task queue. Many independent subscribers each seeing every message is pub/sub. Replayable history is a log.
- See [[broker-selection]] for the decision matrix.

**Example:**
```
"Send a welcome email when a user signs up"
  Wrong: write directly to Kafka, because Kafka is what we have.
  Right: a task queue (SQS, BullMQ, Sidekiq) with retries and a DLQ — point-to-point work, no replay needed.

"Notify analytics, billing, and the recommender when an order is placed"
  Wrong: have the order service call all three over HTTP.
  Right: publish OrderPlaced to a log/pub-sub topic. Each consumer reads independently
         at its own pace; adding a fourth consumer doesn't touch the order service.
```

---

### 2. Know your delivery semantics

**Rule:** Pick one of {at-most-once, at-least-once, exactly-once} consciously, and know which one your broker actually delivers. "Exactly-once" is almost always at-least-once + an idempotent consumer.

**Why:** Almost every production broker is **at-least-once** by default. That means duplicates happen — on retries, on consumer crashes, on rebalances, on network blips. If you wrote your handler assuming "this runs once per message," you wrote a bug. Every queue-bug postmortem with the words "we double-charged" or "we sent the email twice" started here.

**How to apply:**
- Default mental model: at-least-once. Build for it (see principle 3).
- At-most-once is fine *only* when loss is cheaper than duplicates (high-volume telemetry, ephemeral UI hints). Be honest about which side of that line you're on.
- "Exactly-once" exists narrowly *within* a single broker's domain (Kafka transactions + idempotent producer + `isolation.level=read_committed` consumer, set up correctly, deliver true end-to-end EOS for Kafka-to-Kafka pipelines; the idempotent producer has been on by default since Kafka 3.0). It breaks the moment you cross to an external system (DB, HTTP, email, another broker). Treat any guarantee that ends at the broker's boundary as at-least-once for the rest of your code, and reach for the outbox pattern (principle 7) when the cross-system boundary matters.
- Read your broker's docs for the *exact* semantics, including under failure: leader re-elections, consumer rebalances, ack timeouts. The defaults differ.

**Example:**
```
SQS Standard:        at-least-once, no ordering.            → idempotent consumer required.
SQS FIFO:            exactly-once *delivery* within 5 min,  → still treat as at-least-once
                     per MessageGroupId.                       outside that window.
RabbitMQ (ack mode): at-least-once.                          → idempotent consumer required.
Kafka (default):     at-least-once.                          → idempotent consumer required.
Kafka EOS:           exactly-once *between Kafka topics*     → still at-least-once if you
                     within one transaction.                    write to a DB or call HTTP.
```

---

### 3. Make consumers idempotent

**Rule:** Every message handler must produce the same observable effect whether called once, twice, or N times with the same message.

**Why:** Because at-least-once means duplicates *will* happen. This is the single highest-leverage line of defense in a queue-based system — it turns retries, redeliveries, replays, and rebalances from incidents into non-events. A consumer without an idempotency story is one consumer crash away from a billing incident.

**How to apply:**
- Prefer **naturally idempotent** operations: `SET balance = 100` (idempotent) vs `INCREMENT balance BY 5` (not). When the domain allows, model state as absolute rather than relative.
- Use **conditional writes** to make non-idempotent operations idempotent: `UPDATE order SET status='shipped' WHERE id=? AND status='paid'`. The second call updates zero rows — a no-op, not a duplicate ship.
- When neither works, add an **idempotency key** (often the message ID, or a domain-level operation ID the producer chose). On receive, the consumer checks a dedup table inside the same DB transaction as the side effect; if the key is present, drop the message.
- For external side effects (charging a card, sending an email), pass the idempotency key *through* to the external API when it supports one (Stripe, SendGrid, etc.). When it doesn't, record "I started this work" before the call and "I finished it" after — recovery checks the marker.
- **Your own public mutation API should accept an `Idempotency-Key` header too.** The Stripe-style contract (UUID v4 from the client, server stores key + response for ≥24h, replays cached status+body on duplicate) is now an IETF draft and a near-universal expectation for any POST/DELETE that costs money or sends an irreversible side effect. This is the HTTP-layer cousin of the queue-consumer dedup table — same idea, different boundary. See [[idempotency]] for the on-the-wire contract.
- See [[idempotency]] for patterns and code.

**Example (TypeScript, dedup table + conditional write):**
```ts
// Bad — at-least-once will double-charge eventually.
async function handleOrderPaid(msg: OrderPaidMessage) {
  await stripe.charge(msg.customerId, msg.amount)
  await db.query('UPDATE orders SET status=$1 WHERE id=$2', ['paid', msg.orderId])
}

// Good — idempotent at every step.
async function handleOrderPaid(msg: OrderPaidMessage) {
  // 1. Pass the broker's message ID to Stripe as their idempotency key.
  //    Stripe will collapse retries to the same charge.
  await stripe.charges.create(
    { customer: msg.customerId, amount: msg.amount },
    { idempotencyKey: msg.messageId },
  )
  // 2. Conditional write — second call updates 0 rows, harmlessly.
  await db.query(
    `UPDATE orders SET status='paid' WHERE id=$1 AND status='pending'`,
    [msg.orderId],
  )
}
```

---

### 4. Ack only after the work is durably done — and respect the visibility timeout

**Rule:** Acknowledge a message *after* its effect is durable. Until then, the broker should consider the message in-flight and redeliver it if you crash. If your work might exceed the broker's visibility timeout, extend it explicitly.

**Why:** This is where delivery semantics actually live in code. **Ack-before-work** loses messages on consumer crashes (the broker thinks it's done, the work never ran). **Ack-after-work** is correct but causes duplicates on crash — which is exactly what principle 3 was for. The third trap is the visibility timeout: brokers (SQS, RabbitMQ with consumer ack, etc.) hide a delivered message for a window; if you don't ack within that window, they redeliver to another consumer. A 30-second visibility timeout on a 5-minute job means the job runs three times in parallel before the first one finishes.

**How to apply:**
- Ack pattern: receive → do work → write result → **then** ack. Never the other order.
- Set the visibility timeout longer than your p99 work time, with margin. If work time is unpredictable, **heartbeat / extend** the visibility window from inside the handler.
- For long-running jobs (minutes+), consider a "claim" model: write `job.status='running', claimed_at=now()` in your DB before starting, and have a separate watchdog reclaim stuck jobs by timestamp, independent of the broker's visibility timeout.
- Never `await broker.ack()` inside a `try` block whose `catch` is wide — a thrown exception after ack leaves you in an "acked but unfinished" state, which is silent message loss.
- For Kafka, the analog is **commit offsets after** the work is durable, not on every poll. Auto-commit is a footgun; turn it off for anything that matters.

**Example (Go, SQS-style):**
```go
// Bad — acks before the side effect commits. Crash between ack and DB write loses the message.
func handle(ctx context.Context, msg Message) error {
    if err := broker.Ack(ctx, msg); err != nil { return err }
    return processAndSave(ctx, msg) // if this panics, message is gone
}

// Good — ack only after the durable effect is committed.
func handle(ctx context.Context, msg Message) error {
    if err := processAndSave(ctx, msg); err != nil {
        return err // do not ack; broker will redeliver after visibility timeout
    }
    return broker.Ack(ctx, msg)
}

// Better — for long work, extend visibility while we run, then ack at the end.
func handleLong(ctx context.Context, msg Message) error {
    stop := startHeartbeat(ctx, msg, 20*time.Second) // extend visibility every 20s
    defer stop()
    if err := processAndSave(ctx, msg); err != nil {
        return err
    }
    return broker.Ack(ctx, msg)
}
```

---

### 5. Plan for poison messages: retries, backoff, and a dead-letter queue

**Rule:** Every consumer needs three numbers and one destination: max attempts, backoff strategy, jitter, and a DLQ. Decide these before the first message flows.

**Why:** Some messages will always fail — bad payloads, schema drift, removed dependencies, references to deleted rows. Without a cap, a single poison message can spin forever, burn budget, and starve healthy messages behind it. Without backoff, a downstream blip turns into a thundering herd. Without a DLQ, you have no visibility into what's broken and no way to replay it after a fix. These are not optional; they are the operational floor.

**How to apply:**
- **Cap retries.** Pick a max attempts (commonly 3–10 depending on side-effect cost). After the cap, send the message to a DLQ instead of retrying forever.
- **Exponential backoff with jitter.** Doubles the gap on each retry (1s, 2s, 4s, 8s...) plus a random fraction so retries don't synchronize across consumers. Most brokers support this natively; turn it on.
- **Distinguish retryable from permanent errors.** A 5xx from a downstream service or a network timeout is retryable. A schema-validation failure or a 4xx from a downstream is not — those should DLQ on the first attempt. Retrying a permanent failure is just delaying the inevitable while wasting attempts.
- **DLQ is a human inbox.** Alert on DLQ depth > 0 (or > N for noisier systems). Have a runbook for triaging: inspect payload, fix the bug or the data, redrive. A DLQ nobody watches is the same as no DLQ.
- See [[operating]] for DLQ runbooks and alerting thresholds.

**Example (config sketch):**
```yaml
consumer:
  max_attempts: 5
  backoff:
    initial: 1s
    multiplier: 2
    max: 5m
    jitter: 0.2          # ±20% randomization
  dead_letter:
    target: orders-dlq
    on:
      - any error after max_attempts
      - SchemaValidationError on first attempt   # don't retry permanent errors
      - DependencyDeletedError on first attempt
alerts:
  - dlq_depth > 0 for 5m   → page on-call
  - oldest_message_age > 1h → page on-call
```

---

### 6. Ordering is opt-in, not a default

**Rule:** Most brokers do not preserve global message order. If your handler assumes order, prove the broker delivers it for the keys you care about — or make the handler order-tolerant.

**Why:** "Process messages in the order they were sent" is the most-broken hidden assumption in queue code. SQS Standard is unordered. RabbitMQ preserves per-queue order but not across consumers (multiple workers race). Kafka guarantees order **per partition**, not per topic. The moment you scale consumers horizontally, "in order" stops holding unless you explicitly designed for it. Code that worked in dev with one worker breaks the day you add a second.

**How to apply:**
- Ask "for what subset of messages does order need to hold?" — almost never *all* of them. Order usually matters per-entity (this user's events in order; this order's state transitions in order), not globally.
- Route by **partition key / message group ID** so all messages for one entity go to one partition/consumer. Kafka partition key, Kinesis shard key, SQS FIFO `MessageGroupId`, RabbitMQ consistent-hash exchange.
- For SQS-style queues, **SQS FIFO** gives ordering within a `MessageGroupId`. Standard SQS does not — don't assume.
- Better than ordering: make the handler **version-aware** or **commutative** so out-of-order delivery is safe. Carry a monotonic version (`updated_at`, `seq`) on the message; the consumer applies a state change only if its version is newer than what's stored. Two messages arriving out of order produce the same final state.
- Global ordering = one consumer = a bottleneck. If you genuinely need it, name it explicitly and accept the throughput ceiling.

**Example (TypeScript, version-aware consumer):**
```ts
// Out-of-order safe: stale messages are dropped, latest wins.
async function applyUserUpdate(msg: UserUpdatedMessage) {
  // Conditional update on version: only apply if msg is newer than what we have.
  const result = await db.query(
    `UPDATE users SET name=$1, version=$2
     WHERE id=$3 AND version < $2`,
    [msg.name, msg.version, msg.userId],
  )
  // rowCount === 0 means a newer update already landed; this one is stale. Drop it.
  if (result.rowCount === 0) return
}
```

---

### 7. Don't dual-write to DB and queue — use an outbox

**Rule:** Never write to a database and publish to a broker as two separate steps. They cannot be made atomic across systems, and the gap is where data gets corrupted in production.

**Why:** The pattern "save the row, then publish the event" looks fine until the process crashes between the two — or the DB commits but the broker is down, or the broker accepts but the DB rolls back. Each of those leaves an inconsistency that no amount of retries fixes (the in-memory message is gone, or the DB never persisted, or duplicate events get published on retry). This is the single most common source of "we lost the event" bugs. The fix is well-known and not optional for systems where the events matter.

**How to apply:**
- **Transactional outbox.** Write the outgoing message into an `outbox` table *in the same DB transaction* as the state change. A separate relay process polls the outbox (or uses CDC on it) and publishes to the broker. The relay marks rows as published. Producers never talk to the broker directly.
- **CDC (Change Data Capture).** Read the DB's write-ahead log (Postgres logical replication, MySQL binlog, MongoDB oplog) with a tool like Debezium, and turn DB changes into events automatically. Higher ops cost than outbox; lower application-code cost.
- The relay/CDC is at-least-once → publish duplicates are possible → consumers must be idempotent (principle 3). The two principles compose: outbox eliminates *lost* events, idempotency eliminates *duplicate* effects.
- See [[outbox]] for the table schema, relay design, and CDC trade-offs.

**Example (TypeScript, outbox write inside the same transaction):**
```ts
// Bad — dual write. Crash between the two = lost event, no way to recover.
async function placeOrder(cmd: PlaceOrderCommand) {
  const order = Order.create(cmd)
  await db.query('INSERT INTO orders ...', [order])
  await broker.publish('order.placed', order) // ← if this fails, the event is gone
}

// Good — single DB transaction; relay publishes from the outbox later.
async function placeOrder(cmd: PlaceOrderCommand) {
  const order = Order.create(cmd)
  await db.transaction(async (tx) => {
    await tx.query('INSERT INTO orders ...', [order])
    await tx.query(
      `INSERT INTO outbox (id, topic, payload, created_at)
       VALUES ($1, $2, $3, now())`,
      [order.eventId, 'order.placed', JSON.stringify(order)],
    )
  })
  // A separate relay process reads outbox rows and publishes them. The use case
  // doesn't know the broker exists.
}
```

---

## Pre-flight checklist

Before introducing or modifying a queue-based path, run through these:

1. **Purpose:** can I name in one sentence what this queue is for? Does the broker shape match (task / pub-sub / log / buffer)?
2. **Delivery:** what semantics does the broker provide, and have I read the docs for the failure modes? (Default assumption: at-least-once.)
3. **Idempotency:** is the handler idempotent under duplicate delivery? Natural, conditional, or keyed?
4. **Ack:** does the ack happen *after* the durable side effect? Is the visibility timeout longer than the work time (or am I heartbeating)?
5. **Retries + DLQ:** are max attempts, backoff (with jitter), and a DLQ configured? Do permanent errors short-circuit to the DLQ?
6. **Ordering:** does the handler assume order? If so, is it routed by partition key, or is it version-aware/commutative?
7. **Dual-write:** is anything writing to both a DB and a broker in two steps? If so, replace with an outbox.

If any answer is "I don't know," stop and find out before shipping.

## Backpressure, observability, and message hygiene

Three operational concerns that don't need their own principle but cannot be skipped.

- **Bound your queues.** Unbounded queues turn into latency black holes — work piles up faster than it drains, oldest messages get older, no signal until something else breaks. Set a max depth or a max age and apply backpressure (reject, drop oldest, slow the producer) when crossed.
- **Watch the right metrics.** The four signals that catch almost everything: queue depth, consumer lag (or age of oldest unacked message), error rate, DLQ depth. Alert on each. A queue without these dashboards is operating blind.
- **Keep messages small and schema-stable.** Big payloads (>256 KB-ish) inflate broker cost and slow throughput — use the claim-check pattern (store the blob in object storage, send the URL). Treat the message schema as a public API: add fields with defaults, never rename or remove without a deprecation window, version explicitly when you have to break.

See [[operating]] for thresholds, dashboards, and the claim-check pattern.

## When to skip this skill

- Using a `List`, `Deque`, `Queue`, or `chan` purely as an in-process data structure, with no concurrency, persistence, or cross-process concerns. (That's a data-structure choice — covered by [[programming-fundamentals]].)
- Throwaway scripts or one-shot data fixes where loss is acceptable and there's no production system on the other end.
- Synchronous request/response paths that don't involve a queue at all.

For everything else — any async boundary, any background job, any event publication, any message broker — these fundamentals apply, even on the "small" feature, even on the "internal" event.

## Reference files

Deeper guides for individual principles. Read the one that matches the work in front of you; you don't need to read them all upfront.

- `references/broker-selection.md` — decision matrix across in-memory, Redis, SQS, RabbitMQ, Kafka, NATS, Pub/Sub, and DB-backed job tables. Use when picking a broker (principle 1).
- `references/idempotency.md` — natural idempotency, conditional writes, dedup tables, idempotency keys for external APIs. Use when designing a consumer (principle 3).
- `references/outbox.md` — outbox table schema, relay design, CDC trade-offs, common pitfalls. Use when a use case writes to both a DB and a broker (principle 7).
- `references/operating.md` — DLQ runbooks, alerting thresholds, backpressure strategies, the claim-check pattern, schema evolution rules. Use when wiring up retries, dashboards, or message shapes (principle 5, plus backpressure/observability).

## How to use this skill in a conversation

This skill is always-on for queue-shaped work (per the project rule at `.claude/rules/queue-fundamentals.md`). Don't ask the user to opt in. If the task is in "When to skip", say so in one sentence and proceed without it.

When the skill applies:
- **Designing a new async path** — name the purpose first (principle 1), then the delivery semantics (2), then walk the rest of the checklist before writing code.
- **Reviewing existing queue code** — go through the 7 principles as a checklist. Be explicit about which ones the code currently fails. Cite the principle number when you flag an issue.
- **Debugging a queue incident** — start at the symptom and work back through the principles. "Double-charged" → principle 3 (idempotency). "Lost event" → principle 7 (dual-write) or principle 4 (ack order). "Stuck processing" → principle 5 (poison message, no cap or DLQ). "Out-of-order state" → principle 6.
- When you make a non-obvious call (turning off auto-commit, introducing an outbox, choosing FIFO over Standard, capping retries at a specific number), say *why* in one sentence and cite the principle. Don't emit YAML or code silently.
