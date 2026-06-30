# Broker selection

Companion to principle 1 of [[queue-fundamentals]]. Use this when you have to choose *what* sits between the producer and the consumer. The goal is to match the shape of your problem to the shape of the broker — not to default to whatever the team used last time.

## Five shapes a queue can take

Most queue debates collapse once you can name which of these you actually need.

1. **In-process buffer** — same process, decouple two pieces of code or smooth a burst. Lost on restart. Examples: Go `chan`, Node `EventEmitter` + array, Python `asyncio.Queue`.
2. **Task / work queue** — point-to-point: one message, one worker out of a pool processes it. Built-in retries, backoff, scheduled jobs. Examples: SQS, BullMQ (Redis), Sidekiq (Redis), Celery (Redis/RabbitMQ), `pg-boss` (Postgres).
3. **Pub/sub** — fan-out: one message, every subscriber gets a copy. Each subscriber tracks its own progress. Examples: SNS+SQS fanout, RabbitMQ fanout exchange, Google Pub/Sub, NATS, Redis Pub/Sub (non-durable).
4. **Event log** — append-only, replayable history. Partitioned. Multiple independent consumer groups, each with its own offset. Examples: Kafka, AWS Kinesis, Redpanda, Pulsar, Redis Streams.
5. **DB-backed job table** — a table with `status`, `attempts`, `run_after`. A worker polls and locks rows. Examples: `pg-boss`, Oban (Elixir), Que (Ruby), hand-rolled. Underrated for many use cases.

If you can't name which of the five your problem is, you're not ready to pick a broker.

## Decision matrix

The questions, in order:

| Question | If yes, lean toward |
|---|---|
| Same process, loss acceptable on restart? | In-process channel |
| Already have Postgres, jobs < ~100/s, no fan-out? | DB-backed job table |
| Point-to-point work, want managed infra? | SQS / BullMQ / Sidekiq |
| Multiple independent consumers, each needs every message? | Pub/sub (SNS+SQS, Pub/Sub, RabbitMQ fanout) |
| Need to **replay** history, audit, or add new consumers retroactively? | Event log (Kafka, Kinesis, Redis Streams) |
| Need strict per-entity ordering at high throughput? | Log with partition key, or SQS FIFO |
| Need scheduled / delayed jobs (`run in 2h`)? | Task queue with delay support (BullMQ, Sidekiq, SQS DelaySeconds, DB-backed) |
| Need exactly-once semantics tightly coupled to a DB write? | Outbox + your existing broker — see [[outbox]] |

## Per-broker cheat sheet

The actual behaviors that bite. Verify against current docs before shipping — defaults change.

### In-process channel (Go chan, Node, Python asyncio)
- **Durability:** none. Restart = lost.
- **Delivery:** at-most-once.
- **Ordering:** preserved per channel.
- **DLQ:** none — handle errors at the consumer.
- **Use when:** you want to decouple two pieces of code in the same process, and you'd accept the loss if the process died. Almost always paired with a graceful-shutdown drain.

### SQS Standard
- **Durability:** durable; replicated.
- **Delivery:** at-least-once.
- **Ordering:** none. Out-of-order is normal, not an edge case.
- **DLQ:** first-class. Configure a redrive policy with max receives.
- **Visibility timeout:** explicit; extend with `ChangeMessageVisibility`.
- **Use when:** point-to-point work, AWS-native, fine with no global ordering.

### SQS FIFO
- **Durability:** durable.
- **Delivery:** exactly-once delivery within a 5-minute deduplication window, per `MessageGroupId`.
- **Ordering:** preserved within a `MessageGroupId`. Across groups: parallel and unordered.
- **DLQ:** first-class.
- **Caveat:** throughput is much lower than Standard; the 5-minute dedup window is **not** a substitute for an idempotent consumer.

### RabbitMQ
- **Durability:** depends on queue + message flags. Confirm `durable: true` queues + `persistent` messages + `publisher confirms`.
- **Delivery:** at-least-once with consumer acks (always set `manual_ack`).
- **Ordering:** per-queue, single consumer. Multiple consumers on the same queue → no ordering.
- **DLQ:** dead-letter exchange (`x-dead-letter-exchange`) — configure per queue.
- **Caveats:** auto-ack is a footgun for anything that matters. Mirrored queues and quorum queues have very different durability guarantees — pick consciously.

### Kafka
- **Durability:** durable. Configure `acks=all` and a replication factor of 3 for anything important.
- **Delivery:** at-least-once by default. The **idempotent producer is on by default since Kafka 3.0** (no duplicate writes from producer retries within a session). **End-to-end exactly-once** requires three things together: idempotent producer + transactional producer (`transactional.id`) + consumer with `isolation.level=read_committed`. This works for Kafka-to-Kafka topology only — the moment you write to an external DB or call HTTP, you're back to at-least-once and need the outbox (see [[outbox]]).
- **Cluster mode:** **KRaft (Kafka Raft) is the only supported mode from Kafka 4.0 (March 2025) onward.** ZooKeeper was removed entirely; ZK-backed clusters lost upstream support in late 2025. Any new Kafka cluster is KRaft. Existing ZK clusters must migrate.
- **Ordering:** per partition. Use a stable partition key to keep one entity's events in one partition.
- **DLQ:** **not built in.** You implement it: catch in the consumer, publish to a `*-dlq` topic, then commit.
- **Offsets:** turn off auto-commit for anything that matters. Commit only after the work is durable.
- **Use when:** you need a replayable log, multiple consumer groups, or high throughput with per-key ordering.

### Redis Streams
- **Durability:** depends on Redis persistence config (AOF/RDB). Treat as durable only if you've configured it that way and have replication.
- **Delivery:** at-least-once via consumer groups + `XACK`.
- **Ordering:** preserved per stream.
- **DLQ:** not built in; use the pending entries list (PEL) + `XCLAIM` for redrive, or your own DLQ stream.
- **Use when:** you're already running Redis, throughput needs are moderate, you don't need partitioning.

### Google Pub/Sub
- **Durability:** durable.
- **Delivery:** at-least-once. (There's an "exactly-once" mode — read the docs; it has limits.)
- **Ordering:** off by default; opt in with ordering keys, which constrains throughput.
- **DLQ:** first-class (dead letter topic, max delivery attempts).

### NATS / NATS JetStream
- **Plain NATS:** at-most-once, no persistence. Don't use for anything you care about losing.
- **JetStream:** durable, at-least-once, ack-based, with DLQ-like features. Use this if you're on NATS and need durability.

### BullMQ (Redis), Sidekiq (Redis), Celery
- Task queues with retries, scheduled jobs, and built-in worker pools. Built on Redis (mostly).
- **Delivery:** at-least-once.
- **Ordering:** not guaranteed across workers; jobs are independent.
- **DLQ:** "failed jobs" set; some support automatic DLQ-like behavior. Configure max attempts and inspection.
- **Use when:** you want batteries-included background jobs without standing up a separate broker.

### DB-backed job table (pg-boss, Oban, Que, hand-rolled)
- **Durability:** as durable as your DB.
- **Delivery:** at-least-once via row locking (`SELECT ... FOR UPDATE SKIP LOCKED`).
- **Ordering:** by `created_at` or `priority`; you control it in the query.
- **DLQ:** a `failed_jobs` table or `status='dead'` flag — you build it.
- **Transactional with DB writes:** yes, naturally. This is the killer feature — no outbox needed for use cases that only emit work to themselves, because the job insert is in the same DB transaction as the state change.
- **Limits:** polling adds latency (LISTEN/NOTIFY helps); throughput ceiling is your DB's write throughput; doesn't scale to fan-out across services.
- **Use when:** your team already runs Postgres well, throughput is moderate, and you want fewer moving parts.

## Common selection mistakes

- **Using Kafka as a task queue.** Kafka is a log; it doesn't have per-message ack/redelivery semantics. Treating it as a task queue means you reinvent retries, DLQ, and ack tracking by hand — and you'll do it worse than SQS/BullMQ already does.
- **Using SQS as an event log.** SQS doesn't replay. Once a message is acked, it's gone. If you ever want "replay the last 24 hours to a new consumer," you needed Kafka.
- **Using Redis Pub/Sub for anything durable.** Redis Pub/Sub is fire-and-forget — subscribers connected at publish time get the message; everyone else doesn't. Use Redis Streams instead if you need durability.
- **Standing up Kafka for 10 messages/minute.** Operational cost is wildly out of proportion. A DB-backed job table or SQS is almost always the right answer at that scale.
- **In-process channel for cross-service coordination.** If the producer and consumer are in different processes (or different machines), an in-memory queue is not a queue — it's a memory leak with extra steps.

## How to decide quickly

When in doubt, this is the order I'd try:

1. Can it be a **DB-backed job table**? If yes, default to that — fewer moving parts, transactional with your data.
2. Otherwise, is it **point-to-point work**? Use SQS / BullMQ / Sidekiq.
3. Otherwise, is it **fan-out to N independent consumers**? Use Pub/Sub (SNS+SQS, Google Pub/Sub) or RabbitMQ fanout.
4. Otherwise, do you need **replayable history or per-key ordering at scale**? Use Kafka.
5. Same process, loss-tolerant? In-process channel.

If you can't justify a more complex broker in one sentence, drop to the next-simpler option.
