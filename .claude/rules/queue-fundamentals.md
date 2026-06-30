# Rule: Queue fundamentals by default

For every task that introduces, modifies, or debugs a queue-based path — message brokers, event streams, background jobs, async workers, pub/sub topics, task queues — invoke the `queue-fundamentals` skill **before** designing or writing code.

This rule is the always-on pointer. The 7 principles (broker selection, delivery semantics, idempotent consumers, ack discipline, retries/DLQ, ordering, outbox), pre-flight checklist, operational concerns (backpressure, observability, message hygiene), and the deep-dive guides on broker selection, idempotency, the outbox pattern, and operating queues in production all live in the skill:

- `.claude/skills/queue-fundamentals/SKILL.md`

**Why:** Queues are the joints of a distributed system, and almost every "ghost in the machine" production bug — silent message loss, double-charges from redelivered jobs, ordering assumptions that break under load, dead-letter queues nobody set up so a single poison message wedges the pipeline, a DB write that "succeeded" with no matching event — traces back to a handful of missed queue fundamentals. The mechanics differ across brokers (SQS, Kafka, RabbitMQ, BullMQ, an in-memory channel); the contract you have to think about does not. Catching these at design time costs minutes; catching them in an incident costs days and customer trust.

**How to apply:** At the start of any queue-shaped task, load the `queue-fundamentals` skill and run the 7-principle pre-flight (purpose → delivery semantics → idempotency → ack discipline → retries/DLQ → ordering → dual-write). Apply the relevant reference file when the work is concentrated in one area (broker pick → `references/broker-selection.md`; consumer design → `references/idempotency.md`; DB + broker writes → `references/outbox.md`; retries/dashboards/message shape → `references/operating.md`). The skill itself lists when to skip (pure in-process data structures, throwaway scripts) — defer to it rather than re-deciding here.

**Relation to other skills:** Queue fundamentals compose with [[programming-fundamentals]] (the code inside a consumer must still respect data shape, pure core, error handling), [[database-fundamentals]] (the outbox table, dedup tables, and DB-backed job tables are schema decisions with indexes and transactions), and [[hexagonal-backend]] (producers and consumers are adapters; the use case sees ports; the outbox sits between domain writes and the broker). They are not competing. Run order when multiple apply: `ddd-strategic` → `programming-fundamentals` → `database-fundamentals` → `hexagonal-backend` → `architecture-fundamentals` → this skill. The queue contract is the outermost layer, sitting on top of code, schema, and architecture.

**Status:** Active. Applies to all queue-related work in this project and any project that adopts this foundation.
