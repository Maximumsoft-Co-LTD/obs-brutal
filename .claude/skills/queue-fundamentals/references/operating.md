# Operating queues in production

Companion to principle 5 of [[queue-fundamentals]], plus the "backpressure, observability, and message hygiene" section. Use this when wiring up retries, dashboards, DLQ handling, or shaping the messages themselves. Most queue incidents are operational, not logical — these are the controls that turn a queue from a liability into a load-bearing piece of infrastructure.

## Retries: the three numbers and one destination

Every consumer needs all four configured before the first message flows:

1. **Max attempts** — how many tries before giving up. Typical range:
   - **Idempotent, cheap operations:** 5–10.
   - **Operations with side effects to external systems (charges, emails):** 3–5. Each retry is a real-world call that costs money or annoys someone.
   - **Cheap, fast retries with a tight downstream SLO:** can go higher; just make sure they don't pile up.
2. **Backoff strategy** — exponential is the default: `delay = base * 2^attempt`. Cap the maximum delay (e.g. 5–15 minutes) so the queue doesn't go effectively dormant after a few failures.
3. **Jitter** — randomize the backoff (`delay * (1 ± 0.2)`) so retries from many consumers don't synchronize. Without jitter, a downstream blip becomes a thundering herd of synchronized retries that DDoS your own services.
4. **DLQ target** — once max attempts is hit, the message goes here. Required, not optional.

**Worked example:**
```
attempt 1: try immediately
attempt 2: wait  1s ± 20%
attempt 3: wait  2s ± 20%
attempt 4: wait  4s ± 20%
attempt 5: wait  8s ± 20%
            └→ if still failing, route to DLQ
```

### Retryable vs permanent

Not every error deserves a retry. Distinguishing matters:

- **Retryable:** transient downstream failures, timeouts, 5xx, network errors, rate limits, lock contention. Retrying might succeed.
- **Permanent:** schema validation failures, 4xx responses, missing required fields, references to deleted entities, type mismatches. Retrying will fail in exactly the same way and waste your attempt budget while delaying inspection.

Permanent errors should DLQ on the **first** attempt. Otherwise a bad payload chews through 5 retries with backoff (potentially many minutes) before showing up where a human can look at it.

**Pattern:**
```ts
async function handle(msg: Message) {
  try {
    await process(msg)
  } catch (e) {
    if (isPermanent(e)) {
      // Short-circuit: go straight to DLQ. No retry will help.
      await broker.deadLetter(msg, { reason: e.message })
      return
    }
    throw e // let the broker retry with backoff
  }
}

function isPermanent(e: unknown): boolean {
  return (
    e instanceof SchemaValidationError ||
    e instanceof EntityNotFoundError ||
    (e instanceof HttpError && e.status >= 400 && e.status < 500 && e.status !== 408 && e.status !== 429)
  )
}
```

## The DLQ is a human inbox

A DLQ that nobody watches is the same as no DLQ. Treat it like an inbox: alert on it, triage it, drain it.

**Alerting thresholds (starting points; tune to your traffic):**
- **DLQ depth > 0 for 5 minutes:** page or notify. Even one message means something is broken and needs eyes.
- **Oldest message age > 30 minutes:** page. Indicates the redrive policy is working but nobody's looking.
- **DLQ growth rate > 0:** if you have many DLQ'd messages, a slope alert is more useful than a count threshold. Catches outages early.

**Triage runbook:**
1. **Look at a sample message.** Topic, headers, payload. Most DLQs have a one-shape problem ("all the bad messages are from publisher X after the schema change").
2. **Classify:** bug in the consumer, bad data from the producer, downstream-permanent-fail, or stale reference (entity deleted).
3. **Fix the cause.** Don't just redrive — redriving without a fix sends them back through the same broken path.
4. **Redrive in batches.** Most broker UIs / CLIs support "move N messages from DLQ back to main." Move in small batches, watch them succeed, then move more.
5. **Document the incident** if it's not trivial. The pattern of DLQ causes over time tells you what to invest in (validation, schemas, dead-link detection).

**What never to do:**
- Re-enqueue silently on a timer (some tools offer this). It hides recurring bugs.
- Delete DLQ messages without inspection. You're throwing away the only signal you have.
- Treat DLQ as overflow. The DLQ is the place messages go to **fail loudly**, not to wait their turn.

## Backpressure: bound your queues

Unbounded queues turn into latency black holes — work piles up faster than it drains, oldest message age grows, and the system appears to be working (queue accepting!) right up until everything downstream times out.

**The four backpressure strategies, ordered by aggressiveness:**

1. **Reject new producers** (return an error / 429 / 503). Best when callers can retry, like external API clients you control. Honest about overload.
2. **Slow producers** (apply rate limits at the producer side). Best when producers are internal services you can ask to behave.
3. **Shed load** (drop or sample messages). Best for low-value telemetry where some loss is preferable to lag.
4. **Drop oldest** (keep the newest N messages, evict the rest). Best when stale messages are useless — fresh metrics, live state syncs.

Whichever strategy you pick, **pick consciously**. The default of "accept everything and pray" is the worst option.

**Where to set the bound:**
- Per-queue max depth (broker-supported on most queues).
- Per-queue max age (oldest message > N seconds → reject incoming).
- Producer-side rate limit (token bucket on the publish call).
- Concurrency limit at the consumer (only N workers; if more work piles up, queue depth grows and signals upstream).

## Observability: the four signals

You need dashboards for these four numbers on every queue. Without them, you're flying blind.

1. **Queue depth** — number of messages waiting. Trending up = consumers can't keep up.
2. **Consumer lag** — for log-based brokers (Kafka, Kinesis): how far behind the head each consumer group is. Equivalent to depth for log brokers.
3. **Age of oldest unacked message** — the most honest signal. A queue with depth 1000 and oldest-age 30s is healthier than depth 100 and oldest-age 10 minutes.
4. **Error rate / DLQ growth** — separate from "did we fail" is "are we failing more than baseline." Use this to spot regressions.

**Secondary signals worth tracking:**
- Processing latency (p50/p95/p99 per consumer).
- Throughput (messages/sec into the queue and out).
- Retry rate (messages on attempt > 1 / total).
- Visibility-extension count (consumers heartbeating because work is slow).

**Alerting starter pack:**
- `queue_depth > N` sustained for 5 min → warning, 15 min → page.
- `oldest_unacked_age > 5 min` → warning. → 30 min → page.
- `consumer_lag > threshold` (Kafka) → page.
- `dlq_depth > 0 sustained` → page.
- `error_rate > 3x baseline` for 5 min → page.

Tune N and timeouts to your traffic, but configure all five from day one.

## Message hygiene

The shape of your messages affects every consumer for the lifetime of the topic. A few rules pay back forever.

### Keep messages small

- **Under ~256 KB** is a safe heuristic; some brokers have hard limits below 1 MB.
- Big payloads inflate broker storage cost, slow throughput, and hurt cache hit rates inside the broker.
- For anything that wants to carry a big blob (image, file, large JSON tree), use the **claim-check pattern**: store the blob in object storage (S3, GCS), send only the URL and a small metadata header in the message. Consumers fetch on demand.

### Treat the message schema as a public API

You don't own the consumers, even if they're inside your team. The moment a message has more than one consumer (or might in the future), the schema is a contract.

- **Add fields with safe defaults.** New consumers benefit; old consumers ignore unknowns.
- **Never rename or remove fields silently.** Add the new field, dual-write for a deprecation window, then remove the old one.
- **Version explicitly** when you have to break — `topic.v2` or a `schemaVersion` field. Don't reuse the topic name with breaking changes.
- **Use a schema registry** (Avro/Protobuf + Confluent Schema Registry, or JSON Schema in a shared package) once you have more than a handful of topics. It catches breaking changes at publish time, not at 2 AM.

### Required headers

Every message should carry, at minimum:

- **Message ID** — stable across redeliveries, used by consumers for idempotency. See [[idempotency]].
- **Trace ID** — for distributed tracing across producer → broker → consumer.
- **Schema version** — even if implicit in the topic name, store it on the message for safety.
- **Created-at timestamp** — for measuring end-to-end latency and detecting old messages.

These are cheap to add up front and painful to retrofit.

## Putting it together: a launch checklist

Before a new queue path goes to production, verify all of these:

- [ ] Max attempts, backoff, jitter, and DLQ configured.
- [ ] Permanent errors short-circuit to DLQ on first attempt.
- [ ] Dashboards exist for depth, oldest-age, consumer lag, DLQ depth, error rate.
- [ ] Alerts wired for sustained depth, oldest-age, and DLQ > 0.
- [ ] Bound on queue depth or producer rate; explicit backpressure strategy chosen.
- [ ] Visibility timeout > p99 work time, or consumer heartbeats to extend.
- [ ] Message payloads under broker limit; claim-check for anything blob-shaped.
- [ ] Required headers (message ID, trace ID, schema version, created-at) populated.
- [ ] DLQ triage runbook exists and is linked from the alert.

If any of these is "no" or "I don't know," fix it before traffic.
