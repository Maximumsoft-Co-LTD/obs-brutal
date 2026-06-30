---
name: architecture-fundamentals
description: Apply system-level architecture fundamentals before designing how modules, services, processes, APIs, events, or teams relate at runtime. Use for service/module boundaries, ownership, sync vs async, resilience, consistency, observability, API/event contracts, scaling, extraction, strangler-fig work, or any cross-component call. If the open question is business meaning, language, bounded contexts, or subdomain investment, use [[ddd-strategic]] first; this skill then decides runtime/component boundaries, communication, failure modes, and contract evolution. Skip purely within-one-service code work, throwaway prototypes, and code-level refactors with no cross-boundary concern.
---

# Architecture Fundamentals

## Why this exists

Most "we need to rewrite this system" stories are not about bad code. They are about boundaries that were never chosen, ownership that was never named, sync calls that should have been async (or vice versa), no story for failure when a dependency blips, eventual consistency that nobody noticed until the data drifted, an outage that took three days to debug because nothing was instrumented, and a "small" API change that broke six downstream consumers because the contract was never thought of as a contract. Each of these is a missed fundamental, and each compounds — a system that gets one wrong is brittle; a system that gets several wrong becomes the thing the team is afraid to touch.

This skill is a **pre-flight** for any system-level architectural work: read it before you draw the first box on the diagram, not after the first outage. The principles are stack-agnostic — they apply equally to a monolith with internal module boundaries, a handful of services, or a sprawling distributed system. The mechanics (which RPC framework, which broker, which observability stack) differ; the decisions you have to make do not.

Skills this one sits next to:
- [[programming-fundamentals]] — the code *inside* any one component. Apply when writing functions.
- [[database-fundamentals]] — the schema, queries, and migrations of *one* data store. Apply when shaping tables.
- [[hexagonal-backend]] — *one service's* internal layering (domain/application/infrastructure). This skill defines how multiple such services relate; hexagonal defines how each one is built. The two compose.
- [[queue-fundamentals]] — the mechanics of *one* queue (delivery semantics, idempotency, ack, DLQ, outbox). This skill decides *whether* a queue belongs in a given interaction; queue-fundamentals decides how to operate it correctly.
- [[debug-fundamentals]] — the recovery sibling. When a system bug crosses boundaries, this skill named the fix layer; debug-fundamentals finds the cause.

## The 7 principles

Each principle has a one-line rule, a *why*, and a worked example. Apply them in roughly this order — the early ones unblock the later ones.

---

### 1. Draw the boundaries before the boxes

**Rule:** Decide what's *one thing* before deciding whether to put it in one module, one service, or one team. Boundaries follow the domain, not the org chart you happen to have today, and not the technology you happen to like.

**Why:** Almost every painful rewrite is a boundary problem. Two concepts welded into one component can't be deployed, scaled, evolved, or replaced independently — every change is a coordination problem. Two components that always change together cost double the operational tax and give you none of the upside of separation. Conway's Law tells you the system will mirror your communication structure; if you don't *choose* the boundaries deliberately, your org chart, your last hire, and your "we already have a service for that" reflex will choose them for you, and those choices last a decade.

**How to apply:**
- Name the **bounded contexts** — clusters of concepts where words have one definition. "Customer" in billing is not the same thing as "customer" in CRM; one model cannot serve both without becoming an unintelligible god-object. The boundary goes where the language changes.
- Within a bounded context, prefer the **smallest deployment unit that lets the team own a coherent piece of business value**. A new service is a permanent operational cost — separate deploy, separate monitoring, separate on-call story, a network call where a function call used to be. Pay that cost only when the boundary buys you genuine independence (independent scale, independent deploy cadence, independent failure domain) that a module inside a monolith couldn't. **The 2024 Fowler/Newman consensus is "monolith unless you have a really good reason"** — a well-modularised monolith with clear internal boundaries is the right starting shape for most systems, and extraction to services (via the strangler fig, see [[boundaries]]) happens when a specific piece earns its way out. Starting microservices-first is a known anti-pattern.
- **Boundaries hide implementation.** If exposing internals (DB tables, internal types, "just call our private endpoint") is the only way to make a feature work across the boundary, the boundary is in the wrong place. Move it before you cement it.
- Watch the "two services that always deploy together" smell. They're really one. Either merge them, or fix the contract so changes on one side don't ripple to the other.
- See [[boundaries]] for bounded contexts, Conway's Law, extracting a service from a monolith, and anti-corruption layers.

**Example:**
```
Wrong: split "user profile" and "user authentication" into two services because they "feel different".
       Every signup touches both. Every login changes both. They redeploy in lockstep forever.
       You now have two services' worth of operational cost for one cohesive concept.

Right: one Identity service owns the user concept end-to-end. Months later, when notifications grow
       into a multi-channel product with its own backlog and on-call rotation, *that's* the natural
       seam — pull it out then, when the boundary has earned its cost.
```

---

### 2. One owner per piece of data

**Rule:** Every piece of state has exactly one component that owns its write path. Everyone else reads through that owner — via API, an event stream, or a read-only projection — and never writes directly to the owner's store.

**Why:** When two components can write to the same table, the same key, or the same topic, you have two truths and no way to choose between them. The bugs look like "the data is right in service A but the dashboard shows it wrong," "the nightly export disagrees with the live API," "we changed it in one place but the other place never noticed." The root cause is always the same: ownership was never named, so every change becomes a coordination problem and every inconsistency becomes a four-hour debugging session. With a single owner, you have one place to enforce invariants, run migrations, and add new fields. Without one, every feature drags two teams behind it.

**How to apply:**
- Pick the component whose **business rules govern the data** — that's the owner. Billing owns invoices, even though Support reads them. Inventory owns stock, even though Orders reads it. The owner is whoever decides whether a write is valid.
- Other components that need the data have three legitimate options, in order of preference:
  1. **Read through the owner's API.** Simple, always consistent at read time, costs a network hop.
  2. **Subscribe to the owner's event stream** and build a local projection. Fast reads, eventually consistent, requires the outbox pattern (see [[queue-fundamentals]]).
  3. **Shared read-only view or read replica.** Last resort — couples the consumer to the owner's storage schema. Only when API and events are genuinely infeasible.
- **No shared writable store.** "We'll just have both services write to the same table to keep things simple" is the single most regretted decision in service-oriented systems. The "simplicity" lasts six weeks; the cleanup lasts six quarters.
- If two components genuinely co-own a concept, you have two concepts hiding inside one name. Disambiguate them: the `User` in Identity is a different concept from the `User` in CRM, even if they share an ID. Each owns its own version.

**Example:**
```
Wrong: Orders service and Inventory service both write to `products` table.
       Inventory updates stock; Orders updates display name on a sale. Schema migrations need
       both teams to coordinate. Bugs ping-pong between them: "who set the stock to -1?"

Right: Catalog service owns `products`. Inventory writes to its own `stock_levels` table keyed by
       product ID. Orders writes nothing to products; it reads display name through Catalog's API
       (or a cached projection). One owner per row, one place to enforce invariants.
```

---

### 3. Choose sync or async for each interaction, deliberately

**Rule:** For every cross-boundary call, name whether the caller must wait for the callee to finish before continuing (sync request/response) or whether the caller can move on while the work happens later (async event/job). The right answer follows from whether the caller actually needs the result *now*, not from which technology stack is in fashion.

**Why:** Sync calls couple components in time — when the callee is slow, the caller is slow; when the callee is down, the caller is down. Long chains of sync calls become outage multipliers ("service A waits on B waits on C waits on D, which has a 99.5% SLO, so the chain delivers 98%"). Async decouples in time but introduces eventual consistency, duplicate-delivery, and ordering concerns. Picking the wrong one is a permanent tax: a sync call where async would do creates a cascade-failure path and user-visible latency; an async call where sync would do creates a "I clicked save and the page says nothing changed... but it does later, sometimes?" experience that no amount of polish can hide.

**How to apply:**
- **Sync for:** queries the user is actively waiting on, operations where the caller needs the result to make the *next* decision, simple reads, anywhere the caller has nothing to do until the answer arrives.
- **Async for:** side effects the caller doesn't need to wait on (sending email, indexing, analytics), fan-out to many independent consumers, smoothing bursts, decoupling deploy/scale/failure between components, anything where "I'll do this in the background and tell you when it's done" is a fine experience.
- **The wait-and-poll pattern is async with extra friction.** If the caller has to poll a status endpoint, you needed async events with a status update or a webhook — not a sync call dressed up.
- **Chain depth matters.** A sync call that calls a sync call that calls a sync call is fragile and slow. Where possible, **fan-out in parallel and gather** rather than chain serially. Where chains are required, set per-hop timeouts that fit inside the overall budget (see principle 4).
- **Async ≠ free.** Async means at-least-once delivery, idempotency requirements, an outbox to avoid lost events, and operating a broker. The cost is real; pay it where the decoupling is worth it.
- See [[communication]] for the decision matrix, sync transport choices (REST, gRPC, GraphQL), and the relationship to [[queue-fundamentals]] for async mechanics.

**Example:**
```
"Charge the customer's card during checkout"
  Sync: the user is staring at a spinner. The next screen depends on whether the charge succeeded.
        Set an aggressive timeout, fail loudly with a useful error if the payment provider is down.

"Send the welcome email after signup"
  Async: the user doesn't need the email sent before the response returns. Publish a
        UserSignedUp event; an emailer consumes it, retries on failure, DLQs on permanent failure.
        Signup never breaks because SMTP is having a moment.

"Recompute the search index after a product update"
  Async: search staleness for a few seconds is fine. Blocking the product-update API on indexing
        is not — and it couples product changes to the health of the search cluster.
```

---

### 4. Every cross-process call can fail — design for it

**Rule:** Set explicit timeouts, bounded retries with backoff and jitter, circuit breakers around dependencies, and bulkheads between failure domains. No infinite waits. No retry storms. No code path where one slow dependency takes the whole system down.

**Why:** Inside a single process, calls return. Across processes, calls hang, retry forever, succeed-without-responding, time-out-but-the-write-happened, and cascade. The default behavior of most HTTP clients, ORMs, and SDKs is genuinely dangerous: thirty-second-to-infinite default timeouts, no retry policy, no circuit breaking, no notion of a budget. The outages this causes follow the same script every time — a downstream blips, the upstream's connection pool fills with hung waits, the upstream itself becomes unresponsive, services depending on the upstream start timing out, a one-component blip turns into a system-wide outage. Designing for failure costs hours up front; not designing for it costs days of recovery and customer trust.

**How to apply:**
- **Timeouts everywhere.** Every outbound call (HTTP, DB, broker, gRPC, external API) gets an explicit timeout, shorter than the upstream's own timeout, with budget left over for retries. Read the SDK docs — defaults are often "infinite" or "thirty seconds" and neither is what you want.
- **Retries: bounded, with exponential backoff and jitter.** Only retry **idempotent** operations or operations behind an idempotency key. Don't retry permanent errors (most 4xx that aren't 408/429) — those just delay the inevitable while burning attempts.
- **Retry budget.** A single user request might be allowed N retries across all hops; a service might cap its overall retry rate at X% of its request rate. Without a budget, retries amplify a downstream blip into a thundering herd that prevents recovery.
- **Circuit breakers** around dependencies that can become flaky. After N failures in a window, fail-fast for a cool-down period instead of stacking hung calls; periodically half-open to test recovery. (Resilience4j on JVM, Polly on .NET, opossum on Node, gobreaker on Go, pybreaker/tenacity on Python. Netflix Hystrix is end-of-life — don't pick it for new work.)
- **Bulkheads** isolate failure: separate thread pools, connection pools, and goroutine groups per dependency. One slow dependency cannot exhaust the resources another dependency needs.
- **Graceful degradation.** When a non-critical dependency is down, return a degraded but useful response (cached data, defaults, "recommendations unavailable") rather than a 500. Decide what "critical" means *before* the incident.
- See [[resilience]] for the patterns, the numbers, library choices, and worked examples.

**Example (TypeScript sketch):**
```ts
// Bad — default-infinite timeout, no retries, no breaker, no fallback.
// One slow downstream call brings down the whole request path.
const profile = await fetch(`${userService}/profile/${id}`).then(r => r.json())

// Good — bounded, retried with backoff, circuit-broken, with a graceful fallback.
const profile = await breaker.call(
  () => withTimeout(2000, () =>
    httpClient.get(`/profile/${id}`, { retries: 2, backoff: { initial: 100, max: 500, jitter: 0.2 } })),
).catch(() => CACHED_OR_DEFAULT_PROFILE) // degraded path — feature still works, just less personalized
```

---

### 5. Default to eventual consistency; treat strong consistency as opt-in

**Rule:** Across components, assume changes propagate eventually. Reach for strong cross-component consistency only when the business *genuinely* requires it — and accept the cost (latency, availability trade-offs, lock contention, operational complexity) deliberately, not by default.

**Why:** Most "the data needs to be consistent" requirements relax the moment someone asks "for how long, and what does the user actually see?" A 200-millisecond propagation lag between the order service and the dashboard is invisible to users; engineering a distributed transaction to eliminate it costs you availability, latency, and complexity that lasts forever. On the other side, casually accepting eventual consistency where the business needs strong (charging the right amount, not overselling the last unit, identity claims) leads to "but the system said it was fine" incidents that the data team spends days reconciling. The mistake is not picking one — the mistake is not naming, per interaction, which one you need and why.

**How to apply:**
- For each cross-boundary write, ask: **how stale can a reader of this data be without the business breaking?** Seconds? Minutes? Never?
- "Never" is rare. When it's real (charging the right amount, not overselling 1-of-1 inventory, identity that must not lie), **keep the consistent operation inside a single transactional boundary** — one component, one database, one transaction. Do not reach across components to enforce strong consistency.
- Most cross-component patterns are: write the source-of-truth, publish an event, downstream catches up. Combine with the **outbox** pattern (see [[queue-fundamentals]]) to make sure the event is published exactly as often as the write commits.
- For multi-step workflows that must succeed-or-undo across components, **start with sagas and compensating actions** — they keep components loosely coupled and available when one piece is down. Reach for **two-phase commit / XA** only inside a narrow, homogeneous boundary where you genuinely cannot tolerate a compensating window (a ledger entry whose components share one DBMS, an audit write that must not be observable mid-commit). The 2024 literature has moved from "sagas always, never 2PC" to a hybrid stance: sagas for user-facing flows, 2PC retained where the participants and the failure mode are tightly controlled. Pick deliberately; don't reach for 2PC because it sounds stronger.
- **Surface the staleness to readers when it matters.** `updated_at` timestamps, "last refreshed N seconds ago," lag metrics. Hidden staleness is worse than visible staleness.

**Example:**
```
Wrong: "The dashboard must show the right account balance instantly." → engineer a synchronous,
       cross-service transaction. Slow, brittle, breaks when billing is down — the dashboard now
       can't even render a stale value because the design depended on freshness.

Right: Billing owns the balance and is the consistency boundary for money. Dashboard subscribes
       to balance-changed events and caches its own projection. Visible: "Updated 2s ago." If lag
       exceeds N seconds, page on-call. Strong consistency lives inside Billing where it must;
       everything outside is eventually consistent and the system stays available when one piece
       is down.

Wrong: "Eventually consistent inventory is fine for our flash sale" → oversold seats. Refund
       and apology email to every customer who silently lost the race.

Right: Inventory is the consistency boundary for stock. Decrement-and-reserve happens inside one
       transaction in one component (with conditional UPDATE WHERE qty > 0). Other components
       consume reservation events to update their views. Strong where strong is required;
       eventual everywhere else.
```

---

### 6. Build observability in from day one

**Rule:** Every request path produces structured logs, useful metrics, and a trace that crosses every component boundary. Define what "healthy" means as SLIs and SLOs *before* you ship — not after the first outage.

**Why:** You cannot operate what you cannot see. In a single process, a stack trace tells you what happened; in a distributed or multi-component system, "what happened" is a relationship across N components and you cannot reconstruct it without correlation IDs, propagated trace context, and metrics over time. Teams that bolt observability on later — after the first outage — spend months instrumenting while the system is actively on fire and trust is being burned. Teams that build it in from day one debug incidents in minutes and ship more reliably. The cost difference is measured in person-quarters, not afternoons.

**How to apply:**
- **Three pillars, all on, structured.**
  - **Logs:** structured (JSON or equivalent), with `trace_id` and `span_id` on every line. No bare `console.log("got here")`.
  - **Metrics:** **RED** for request paths (Rate, Errors, Duration) and **USE** for resources (Utilization, Saturation, Errors). Use histograms or summaries for latency — averages hide the tail where users actually suffer.
  - **Traces:** OpenTelemetry (OTel) is the default — it's the CNCF standard, vendor-neutral, and as of 2025 logs joined traces and metrics as stable signals over OTLP. Trace context propagates across every component hop — HTTP headers, message metadata, DB context — so one user request produces one trace, end-to-end. (Pick a vendor-specific SDK only if you have a hard reason; OTel is portable across Datadog, Honeycomb, Tempo, Jaeger, New Relic, X-Ray.)
- **Define SLIs and SLOs.** SLI = what you measure (e.g., "checkout latency"). SLO = the target ("p99 < 800ms, 99.9% of the time over 30 days"). The gap between SLO and reality is the **error budget** — the budget you spend on feature velocity vs. reliability work.
- **Track DORA delivery metrics alongside SLOs.** Deployment frequency, lead time for changes, change failure rate, and failed-deployment recovery time (the 2023 rename of MTTR) are the standard delivery-performance vocabulary — they describe how *fast and safely* you ship, while SLOs describe how *well the running system performs*. Both are first-class.
- **Correlate everything.** A trace ID generated at the edge flows through every hop, sync or async, and into every log line. One ID, all components, one incident timeline. Without this, debugging a multi-hop request is archaeology.
- **Alert on symptoms, not causes.** Page on "users are seeing errors" or "p99 latency exceeded SLO," not on "CPU is over 80%." The latter generates noise; the former generates signal. Reserve paging for symptoms a human must address right now; route everything else to tickets.
- See [[observability]] for instrumentation patterns, the three pillars in detail, SLI/SLO definition, error budgets, and OpenTelemetry idioms.

**Example:**
```ts
// Bad — unstructured, uncorrelated, no measurable signal.
console.log("processing order", orderId)
try {
  await chargeCard(orderId)
} catch (e) {
  console.log("error", e)
}

// Good — structured, correlated, measurable. RED in three lines, trace propagation, error class.
log.info({ event: "order.charge.start", order_id: orderId, trace_id: ctx.traceId })
const stop = metrics.histogram("order.charge.duration_ms", { route: "checkout" }).start()
try {
  await chargeCard(orderId, { ctx })           // ctx propagates trace context to the downstream
  metrics.counter("order.charge", { result: "success" }).inc()
} catch (e) {
  metrics.counter("order.charge", { result: "failure", reason: classify(e) }).inc()
  log.error({ event: "order.charge.failed", order_id: orderId, trace_id: ctx.traceId, err: serialize(e) })
  throw e
} finally {
  stop()
}
```

---

### 7. Evolve contracts; don't break them

**Rule:** Treat every public API, event schema, and shared message format as a contract. Add fields and behaviors backwards-compatibly. Remove only after an honest deprecation cycle, long enough for every consumer to migrate.

**Why:** In any system with more than one component, you cannot atomically update producer and consumer — they deploy on different schedules, run different versions in flight, and may be owned by teams who learn about your change in a Slack message at 4pm on Friday. Breaking a contract turns "ship the feature" into "coordinate a multi-team rollout" — at best slow, at worst the source of half-deployed inconsistencies and outages where neither version is fully alive. Backwards-compatible evolution lets you ship continuously: the change is invisible to consumers who haven't adopted it, and explicit for those who have. The asymmetry between safe and breaking changes is enormous, and most teams discover this only after their first painful rollback.

**How to apply:**
- **Add, don't replace.** New optional fields with defaults are safe. New required fields are a break. Renaming a field is a break. Removing a field is a break. Changing the type or semantics of an existing field is a break.
- **Version explicitly when you genuinely must break.** `/v1/orders` and `/v2/orders` coexist; events carry a `schema_version`. Both versions run until consumers migrate, then v1 is removed — not before.
- **Tolerant reader.** Consumers ignore unknown fields rather than failing; producers don't depend on consumers having every field. Postel: be conservative in what you send, liberal in what you accept.
- **Deprecation is a long horizon.** Announce, instrument the deprecated path (count usage by consumer), notify consumers, wait until usage is zero (not "low," zero), *then* remove. "We told them six weeks ago" is not the same as "we know nobody's using it."
- **Idempotency keys are part of the public contract.** Any public mutation API accepts an `Idempotency-Key` header (or equivalent in events). It's not an optimization; it's the only way clients can safely retry without your help.
- **Schema registries help.** Avro, Protobuf, JSON Schema with a registry let you encode compatibility rules (backward, forward, full) so CI rejects breaking changes before they ship.
- See [[communication]] for versioning patterns across REST, gRPC, and event-driven systems, and the deprecation runbook.

**Example:**
```
Wrong: Rename `customer_email` to `email` in the user-created event. Half the consumers parse
       the old field name and break on the next producer deploy. Roll back, hold a meeting, ship
       the rename over an entire quarter with manual consumer coordination.

Right: Add `email` alongside `customer_email`. Both fields populated by the producer. Mark
       `customer_email` deprecated in the schema. Track consumption metrics until they hit zero.
       Then — and only then — remove the old field. Total cost: one extra line of code in the
       producer and a quarter of patient observation.
```

---

## Pre-flight checklist

Before designing or modifying anything that spans more than one component, run through these:

1. **Boundaries:** what bounded contexts are at play, and do my proposed components map onto them? Am I splitting things that change together, or merging things that don't?
2. **Ownership:** for every piece of data the design touches, can I name *exactly one* component as the owner? Does every other access go through the owner's API, events, or a derived projection — never a direct write?
3. **Sync vs async:** for every cross-component call, did I deliberately choose sync or async based on whether the caller needs the result now? Are sync chains short enough to fit in the latency budget?
4. **Failure:** does every outbound call have a timeout, a bounded retry policy, and a fallback or breaker for when the dependency is down? No "wait forever" paths?
5. **Consistency:** for each cross-boundary write, did I name the staleness budget? Is strong consistency confined to a single transactional boundary, with sagas (not 2PC) for multi-component workflows?
6. **Observability:** does each new path produce structured logs with a trace ID, RED metrics, and a span in the distributed trace? Is the SLI for this feature defined? Are alerts on symptoms, not causes?
7. **Contracts:** is every change to an API or event schema backwards-compatible? If I must break, did I version explicitly and instrument the deprecated path?

If any answer is "I don't know," stop and find out before shipping the design.

## Below the principles: things that matter but aren't full principles

A few operational concerns that don't earn their own principle but cannot be skipped on real systems.

- **Statelessness scales; statefulness needs a strategy.** A stateless component (no in-memory session, no local-disk state) scales horizontally with a load balancer and replaces with a kill-and-restart. State must live in a shared store (DB, cache, object storage) so any instance can serve any request. The exception is sticky-session and in-memory caches, which are real but require explicit thought about cache coherence and warm-up.
- **Cache deliberately, with an invalidation story.** Caches multiply throughput and hide problems in equal measure. Decide: TTL or invalidation? Read-through, write-through, or aside? Stampede protection? "We'll add a cache" without answering those is how stale-data incidents are born.
- **Security boundaries are architectural.** Every cross-component call crosses a trust boundary; auth and authz live at the boundary, not in shared helpers. Secrets live in a secret store, not in code or env files committed to source. Blast-radius matters: a compromised service should compromise the minimum, not everything.
- **Capacity planning is not optional.** Know the rough throughput each component must handle, the headroom you carry, and where the next bottleneck will appear. "We'll worry about scale later" works until the day it doesn't, and the day it doesn't is usually a launch day.
- **Operations: deploys, rollbacks, runbooks.** A system that can't be rolled back safely shouldn't be deployed at all. Every component has a deploy story, a rollback story, and an on-call runbook with the top five known failure modes and their first-response steps.

These compose with the seven principles — apply them when the corresponding concern shows up in the design.

## When to skip this skill

- Pure within-one-service code work where no boundary is crossed — that's [[hexagonal-backend]]'s territory, plus [[programming-fundamentals]] for the code itself.
- Throwaway prototypes, internal scripts, one-shot data fixes, and proofs-of-concept that will be deleted before they touch production. Skip the architecture pre-flight; just ship.
- Trivial CRUD or thin BFFs that only forward and reshape responses, with no real domain rules, no async paths, no second component to coordinate with.
- Operational tasks that touch the architecture as configuration (a new env var, a dependency version bump) but don't change the shape of the system.

For everything else — anything that crosses a component boundary, anywhere a new dependency is introduced, anytime a contract is touched, any failure-mode question — these fundamentals apply. They apply on the "small" feature too, and especially on the "we'll fix this later" feature.

## Reference files

Deeper guides for individual principles. Read the one that matches the work in front of you; you don't need to read them all upfront.

- `references/boundaries.md` — bounded contexts, module-vs-service decisions, Conway's Law, extracting a service from a monolith (strangler fig), anti-corruption layers, boundary smells. Use when designing the shape of the system (principle 1).
- `references/communication.md` — sync-vs-async decision matrix, REST/gRPC/GraphQL trade-offs, event-driven patterns, API versioning idioms, event schema evolution, idempotency keys as contract, deprecation lifecycle. Use when designing or changing how components talk (principles 3 and 7).
- `references/resilience.md` — timeout budgets, retry policies and budgets, exponential backoff and jitter, circuit breakers, bulkheads, graceful degradation, load shedding, liveness vs readiness probes, chaos testing. Use when designing failure modes (principle 4).
- `references/observability.md` — structured logging patterns, RED and USE methods, histograms and percentiles, SLI/SLO definition, error budgets, distributed tracing with OpenTelemetry, alerting on symptoms. Use when instrumenting a new component or feature (principle 6).

## How to use this skill in a conversation

This skill is always-on for system-level architectural work (per the project rule at `.claude/rules/architecture-fundamentals.md`). Don't ask the user to opt in. If the task is in "When to skip," say so in one sentence and proceed without it.

When the skill applies:

- **Designing a new system or feature that spans components** — walk the principles in order. Name the boundaries (1), name the owner of each piece of data (2), name the sync/async choice for each interaction (3), name the failure mode (4) and consistency story (5) for each. Then sketch observability (6) and contract shape (7). Show the user the structure before writing code.
- **Adding a new cross-component call** — work through principles 3, 4, 6, and 7 explicitly. Don't add a new dependency without timeouts, a fallback, a trace span, and a documented contract.
- **Reviewing an existing system** — use the principles as a checklist. Be explicit about which ones the system currently fails, and cite the principle number when you flag an issue ("this is a principle-2 violation: both Orders and Inventory write to `products`").
- **Splitting a monolith or merging services** — go to [[boundaries]] first. Don't propose a service split without a bounded-context argument and a deployment-independence argument.
- **Debugging a distributed incident** — start from the symptom and trace back through the principles. "We lost an event" → principle 7 (contract) or queue principle 7 (outbox). "Cascading outage" → principle 4 (resilience). "Stale data shown" → principle 5 (consistency). "Can't tell what happened" → principle 6 (observability).
- When you make a non-obvious call — introducing a saga instead of a 2PC, choosing async over sync, adding a circuit breaker, picking a versioning scheme — say *why* in one sentence and cite the principle. Don't emit architecture decisions silently.
