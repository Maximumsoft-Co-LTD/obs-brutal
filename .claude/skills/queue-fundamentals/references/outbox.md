# The outbox pattern

Companion to principle 7 of [[queue-fundamentals]]. Use this whenever a use case has to (a) change state in a database and (b) emit an event to a broker, and both must "happen together." Which is almost any meaningful event-driven backend.

## The problem in one diagram

```
Use case:
  1. INSERT order row in DB     ← commits independently
  2. broker.publish('order.placed', order)   ← commits independently

Failure modes:
  - Crash between 1 and 2:        DB has the order; broker has no event.   → lost event
  - Step 2 fails (broker down):   DB has the order; broker has no event.   → lost event
  - Step 2 succeeds, step 1 rolls back (rare but real with retries):       → ghost event
  - Retrying the whole thing:                                              → duplicate event
```

There is no ordering of these two steps that avoids the problem on its own. Two-phase commit across "your Postgres" and "your broker" isn't a real option in practice. The fix is to make both writes part of **one** transaction by writing to one system, and have a separate process bridge to the other.

## The fix: an outbox table

Add an `outbox` table to the **same** database as your domain data. When the use case wants to publish an event, it writes a row into `outbox` inside the same transaction as the state change. A separate relay process reads `outbox` rows and publishes them to the broker.

```
Use case:
  BEGIN TX
    INSERT INTO orders ...
    INSERT INTO outbox (topic, payload) VALUES ('order.placed', '{...}')
  COMMIT
                                ↓
                          (separate process)
                                ↓
  Relay:
    SELECT * FROM outbox WHERE published_at IS NULL
    publish to broker
    UPDATE outbox SET published_at = now() WHERE id = ?
```

The use case sees one DB transaction. The broker sees the event eventually. There is no window where state changed but the event was lost.

## Schema

```sql
CREATE TABLE outbox (
    id          BIGSERIAL PRIMARY KEY,
    topic       TEXT NOT NULL,
    payload     JSONB NOT NULL,
    headers     JSONB,                              -- message ID, trace ID, partition key
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ                        -- NULL = pending; set when relay succeeds
);

-- Index for the relay's poll query. Partial index keeps it tiny.
CREATE INDEX outbox_pending_idx
    ON outbox (created_at)
    WHERE published_at IS NULL;
```

Notes:
- `id` is monotonic — useful for preserving order during relay.
- `payload` is JSONB (or BYTEA if you're sending binary). Don't try to be clever and store columns per field; the schema would change every time an event shape changes.
- `headers` is where the message ID (used by consumers for idempotency — see [[idempotency]]) lives. Generate it at insert time, not at publish time, so retries publish the same ID.
- Don't delete published rows immediately. Keep them for a retention window (days) for auditability and replay; sweep older rows on a schedule.

## The relay

A small loop. Three flavors:

### 1. Polling relay (simplest)

```
loop:
  rows = SELECT id, topic, payload, headers FROM outbox
         WHERE published_at IS NULL
         ORDER BY id ASC
         LIMIT 100
         FOR UPDATE SKIP LOCKED;

  for row in rows:
    broker.publish(row.topic, row.payload, row.headers)
    UPDATE outbox SET published_at = now() WHERE id = row.id;

  COMMIT;
  if rows was empty: sleep(200ms)
```

Notes:
- `FOR UPDATE SKIP LOCKED` lets you scale the relay horizontally without two workers grabbing the same row.
- The relay is at-least-once: if it crashes after `broker.publish` but before `UPDATE outbox`, it'll publish again on next poll. Consumers must be idempotent. (Which they should be anyway — see principle 3.)
- Tune `LIMIT` and sleep interval for your throughput. Polling adds latency; expect a few hundred ms of lag between commit and publish.

### 2. LISTEN/NOTIFY relay (Postgres-specific, lower latency)

Use a trigger on `outbox` that fires `pg_notify('outbox', '')`. The relay process holds a `LISTEN outbox` connection and wakes up immediately on insert. Otherwise the loop is identical to polling.

Cuts publish latency from "polling interval" to "milliseconds" without much added complexity. Worth it for user-facing latency-sensitive events.

### 3. CDC relay (Debezium, etc.)

A change-data-capture tool reads the DB's write-ahead log directly and publishes outbox-row inserts to the broker. You don't write a relay process — the CDC tool is the relay.

Trade-offs:
- **Pros:** lowest latency; zero application code; works for any DB write, not just outbox-shaped ones.
- **Cons:** operational complexity (Debezium + Kafka Connect + schema registry); needs DB log retention configured; harder to test locally.

Use CDC when you have many services already on this pattern and the ops investment is amortized. For one or two services, polling or LISTEN/NOTIFY is plenty.

## Ordering

Two questions to answer up front:

1. **Per-entity order** (events for one order arrive in DB-insert order on the broker): trivially preserved if the relay reads rows `ORDER BY id` and publishes serially. With parallel relays, route by partition key — set a partition key header on the outbox row matching the entity ID, and let the broker handle within-partition ordering.

2. **Global order** across all entities: don't try. It forces single-threaded relay and limits throughput. Almost no consumer actually needs it.

## When to write to the outbox vs the broker

In the use case, **always** write to the outbox, never directly to the broker. The composition root wires the use case with an outbox-shaped port; the relay is its own deployable.

```ts
// application/ports/event-publisher.ts
export interface EventPublisher {
  publish(event: DomainEvent, tx: Transaction): Promise<void>
}

// adapters/driven/outbox/outbox-publisher.ts — implements EventPublisher
export class OutboxPublisher implements EventPublisher {
  async publish(event: DomainEvent, tx: Transaction): Promise<void> {
    await tx.query(
      `INSERT INTO outbox (topic, payload, headers)
       VALUES ($1, $2, $3)`,
      [event.topic, event.payload, { messageId: event.id }],
    )
  }
}

// Use case (composes with hexagonal):
async function placeOrder(cmd: PlaceOrderCommand): Promise<OrderId> {
  return await uow.run(async (tx) => {
    const order = Order.create(cmd)
    await orders.save(order, tx)
    await events.publish(OrderPlaced.from(order), tx)   // outbox insert
    return order.id
  })
}
```

The use case has no idea Kafka exists. That's the right level of decoupling — see [[hexagonal-backend]].

## Common outbox pitfalls

- **Forgetting the same-transaction rule.** If `outbox` insert is in a separate transaction from the domain write, you're back to dual-write. Use a Unit of Work or pass `tx` through explicitly.
- **No primary key index on `published_at IS NULL`.** A full-table scan kills the relay once outbox grows. Partial index is cheap and essential.
- **Deleting rows immediately on publish.** Now you can't audit what was sent, and a relay bug becomes unrecoverable. Set `published_at` and sweep on a schedule (1–30 days, depending on disk budget).
- **Generating message IDs at publish time.** A retry publishes a *new* ID, breaking consumer-side idempotency. Generate the ID at outbox-insert time and store it.
- **Relay that doesn't `SKIP LOCKED`.** Two relay workers grab the same row, both publish, you get duplicates that weren't strictly necessary. Use `FOR UPDATE SKIP LOCKED`.
- **Publishing the entire DB row as the event.** Events should be intentional API contracts, not "here's everything we changed." Pick the fields, version the shape.
- **Using outbox when a DB-backed job table would do.** If the consumer is in the *same* service and uses the *same* DB, you can skip the broker entirely — insert into a `jobs` table in the same transaction and have a worker poll. Outbox is for cross-system events; job tables are for in-system work.

## Choosing outbox vs CDC

| | Outbox (polling / LISTEN) | CDC (Debezium) |
|---|---|---|
| App-code cost | Low (one table + a relay) | None |
| Ops cost | Low | High (Connect, schema registry, log retention) |
| Latency | 100–500ms (polling) or ~ms (LISTEN/NOTIFY) | ~ms |
| Easy to test locally | Yes | No (needs the full stack) |
| Works for any DB write | Only what you `INSERT INTO outbox` | Any table — opt out per-table |
| Best for | 1–5 services emitting events | Many services or many tables emitting events |

Default to outbox + polling. Move to LISTEN/NOTIFY for latency-sensitive paths. Move to CDC only when the ops investment is justified.

## Putting it together

The outbox eliminates *lost* events. Idempotent consumers (see [[idempotency]]) eliminate *duplicate* effects. Together they give you what people usually mean by "exactly-once," using only at-least-once components. That's the working production answer to the dual-write problem.
