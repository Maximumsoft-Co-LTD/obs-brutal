---
name: ddd-strategic
description: Apply strategic Domain-Driven Design before deciding where a business model lives and what language it speaks. Use for subdomain classification, bounded contexts, ubiquitous language, context mapping, event-storming/domain-storytelling, aggregate sizing, domain vs integration events, or any feature that spans contexts with real business rules. This skill decides semantic/model boundaries; [[architecture-fundamentals]] decides runtime/component boundaries, communication, resilience, and contracts after those meanings are clear. Skip generic CRUD, throwaway prototypes, single-context work, and pure code-level changes.
---

# DDD Strategic

## Why this exists

Most "we need to split this service" or "this codebase has become unintelligible" stories are not architecture problems and not code-quality problems — they are model problems. A `Customer` class with thirty fields, half nullable, with comments like "only set during checkout, ignore in CRM context" is a model trying to serve two contexts at once. A team that calls the same thing `record`, `row`, `entity`, `account`, and `user` across four files is leaking the translation tax forever. A team that builds a custom CRM, custom auth, and custom invoicing in-house is spending core-domain engineering effort on subdomains that should have been bought. A team that ships microservices drawn around technologies (`api-service`, `worker-service`, `db-service`) gets every downside of microservices and none of the boundary benefit.

Each of these is the same missed fundamental: **the strategic half of DDD** — deciding where the model lives, what language it speaks per place, how it relates to other places, and which parts deserve full domain investment versus a CRUD adapter or an off-the-shelf tool.

The **tactical** half (aggregates, entities, value objects, repositories, domain services, factories) is mostly covered by the construction-fundamentals skills already in this library — see "Skills this sits next to" below. This skill deliberately does *not* repeat that material. The one tactical piece kept here is **aggregate sizing**, because Vernon's four rules are sharper than the generic single-owner-data guidance and they sit between strategic design and code in a way nothing else in the library covers.

## Skills this sits next to

- [[architecture-fundamentals]] — system-level boundaries, ownership, communication, resilience. This skill decides *what the boundaries mean* (a linguistic + model boundary, possibly inside one deployable); architecture-fundamentals decides *how components on either side of a boundary talk and survive failure*. The two compose: this skill finds the boundary, that skill operates it.
- [[hexagonal-backend]] — *within* one bounded context, how the code is layered (domain/application/infrastructure). Aggregates and value objects live in the domain layer; repositories are ports. Once a bounded context exists, hexagonal owns its internals.
- [[programming-fundamentals]] — the code inside an aggregate or value object: illegal-state elimination, pure core, error handling. A well-designed aggregate is the architectural shape; programming-fundamentals is what makes its code resilient.
- [[database-fundamentals]] — one bounded context's persistence: schema, transactions, indexes. Aggregate-as-transactional-boundary is the bridge: this skill says "what's in the transaction"; database-fundamentals says "how the transaction behaves."
- [[queue-fundamentals]] — the mechanics of *one* event channel. Internal domain events stay inside a context (and may not need a broker at all); integration events cross contexts and need outbox, idempotency, DLQ — that's queue-fundamentals' territory. This skill draws the line between the two.
- [[debug-fundamentals]] — the recovery sibling when a bug crosses contexts. "Customer email shows wrong in CRM after Billing updates it" is a context-boundary bug; debug-fundamentals finds the cause, this skill names the structural fix (usually: missing ACL, or two contexts secretly sharing a model).

## The 6 principles

Each principle has a one-line rule, a *why*, and a worked example. Run them in roughly this order — early ones unblock later ones.

---

### 1. Classify the subdomain before applying DDD

**Rule:** Before designing aggregates, drawing context maps, or running an event-storming session, name the subdomain and classify it: **core** (your business differentiator — full custom design), **supporting** (necessary but not differentiating — build it pragmatically), or **generic** (commodity — buy, integrate, or use off-the-shelf). The level of DDD investment follows the classification; applying full DDD to a generic subdomain is over-engineering, and applying CRUD-by-default to a core subdomain is throwing away your moat.

**Why:** The single most common DDD failure is not "we did it wrong" — it's "we did it everywhere." Teams apply aggregates, repositories, value objects, and three-layer hexagonal to a 5-table admin tool and pay full tactical-DDD tax for zero domain leverage. The same teams then under-invest in the one subdomain that actually differentiates their product — the pricing engine, the matching algorithm, the risk model — and ship it as anemic CRUD because "we have a deadline." Classification flips both errors at once: it tells you where to spend, and where to *stop* spending. Supporting and generic subdomains want *less* DDD, not more.

**How to apply:**
- **Core** is the subdomain that gives the business its reason to exist — what customers pay for, what competitors can't easily copy. Invest senior engineers, deep domain conversations, full tactical DDD if it earns its cost, event sourcing or CQRS if the read/write asymmetry justifies it. *Build* the core, never buy it.
- **Supporting** is necessary infrastructure that isn't the moat — internal admin tools, custom reporting, the workflow glue between core systems. Build pragmatically: a clean module, a rich enough model to enforce the rules, but no heroic abstraction effort. Often a well-modularised piece of a monolith rather than its own service.
- **Generic** is commodity — auth, billing infrastructure, full-text search, file storage, transactional email, payment processing for standard cases. *Buy* it (Auth0, Stripe, Algolia, SendGrid, S3) or use a well-maintained OSS package. The "we'll build our own" trap on generic subdomains is the single largest waste of engineering time on this list — every "but we have unique requirements" justification on a generic subdomain deserves three rounds of "are you sure?" before any code lands.
- The boundary between these is not fixed. A subdomain that's core today (the recommendation algorithm, when it was the differentiator) can become supporting tomorrow (when the differentiator has moved upstream); a generic subdomain (payments for a normal e-commerce site) becomes supporting when you actually have unique payment requirements (a multi-currency, multi-payout-channel marketplace). Re-classify periodically.
- See [[subdomain-classification]] for the test ("does the business lose its differentiation if we use a commodity version of this?"), the relationship to Wardley mapping, and the investment-allocation framing.

**Example:**
```
Wrong: small e-commerce startup builds custom auth, custom billing engine, custom search ranking,
       custom transactional email. All four are generic subdomains. The team spends 60% of its
       engineering quarters on commodity work that Auth0 + Stripe + Algolia + SendGrid would have
       handled in a week, and ships its actual differentiator — curated recommendations — as
       anemic CRUD with a single SQL query. The thing customers pay for gets the least attention.

Right: same team classifies up front. Auth/billing/search/email → generic, buy. Order management
       and inventory → supporting, build as clean modules in the monolith. Recommendations engine
       → core, full design effort, dedicated team, event-sourced for replayability and A/B
       experimentation. Engineering investment matches business value.
```

---

### 2. Discover boundaries from events, not entities

**Rule:** When you don't yet know what the bounded contexts are, find them by walking the domain's *events* — what happens, in what order, with what consequences — not by listing the nouns the team currently uses. Run an Event Storming or Domain Storytelling workshop with the domain experts present, surface the verbs, and the contexts will reveal themselves at the seams where the language and the actors change.

**Why:** Entity-first design (we have `User`, `Order`, `Product`, `Invoice` — let's draw boxes around them) produces god-objects, because the same noun means different things to different parts of the business and a model that tries to satisfy all of them satisfies none. Event-first design (someone places an order → payment is captured → inventory is reserved → shipment is scheduled → invoice is issued → revenue is recognized) makes the seams visible: the team that talks about "placing an order" is not the team that talks about "recognizing revenue," and forcing them to share a model is forcing two teams to coordinate on every change forever. The workshops are not the only way to find boundaries, but they're the *fastest* way when the domain has real complexity and domain experts are available.

**How to apply:**
- **Don't skip the workshop just because it sounds heavy.** A 3-hour Event Storming session with five domain experts and four engineers can save quarters of refactoring. The artifact is concrete (a wall of stickies in event order with actors, policies, and read models), and it maps almost line-for-line into code: events become domain events, commands become use-case methods, policies become event handlers, read models become projections.
- **Three flavors of Event Storming:** *Big Picture* (whole business line, broad stroke, surface the timeline and the seams), *Process Modelling* (one specific business process end-to-end with commands, policies, and reactions), *Software Design* (zoom into one process and identify aggregates and bounded contexts). Pick the flavor that matches the question you're answering.
- **Domain Storytelling** is the lighter alternative — pictographic actor/work-object/activity diagrams, driven by a facilitator interviewing domain experts. Better for narrative cooperation flows; weaker for event-driven systems. Use it when the domain doesn't have a strong event/state-machine shape.
- **The boundary signals to watch for:**
  - The language changes ("we call it an `order` until it ships, then it's a `shipment`").
  - The actors change (sales people stop being involved; operations people take over).
  - The clock changes (one part runs in real-time, another in nightly batches; one is user-facing, another is back-office).
  - The hotspot pink stickies cluster ("we've never actually agreed how this works") — those are usually telling you a boundary is ambiguous, not that the team is confused.
- The output of the workshop is a *list of candidate bounded contexts* plus the events that cross them. Validate the candidate boundaries by checking principles 3–4 before cementing them in code.
- See [[event-storming]] for the workshop format, the sticky-color grammar, facilitation tips, Domain Storytelling as alternative, and the concrete artifact-to-code mapping path.

**Example:**
```
Wrong: team starts modelling by listing nouns: User, Order, Product, Invoice. Draws boxes
       around each, calls them services. Six months later, every feature touches `User` and
       `Order` because those nouns each mean four different things, and every change requires
       merging conflicting fields. The "model" is now the source of friction, not its solution.

Right: team starts with a 3-hour Event Storming. The timeline shows: cart-built → checkout-started
       → payment-captured → order-confirmed → inventory-reserved → shipment-scheduled →
       package-shipped → delivery-confirmed → invoice-issued → revenue-recognized. The natural
       seams appear: Checkout owns the cart-to-confirmed sequence; Fulfillment owns the
       reserved-through-delivered sequence; Finance owns the invoice-and-revenue sequence. Each
       has its own "order" concept linked by ID. No god-object.
```

---

### 3. One ubiquitous language per bounded context

**Rule:** Inside a bounded context, code, conversations, tickets, dashboards, database tables, and API contracts all use the *same words the domain experts use*, with the same meanings. Across contexts, the same word may mean a different thing, and that's correct — the boundary is exactly where the language changes. Translation, if needed, happens at the boundary (via an anticorruption layer or a published language), never in code that pretends one model serves both contexts.

**Why:** The translation tax — `record` in code, `row` in DB, `entity` in API, `account` in CRM, `user` in talk-track — is paid on every conversation, every code review, every onboarding, and every bug report, forever. Worse, when the language drifts, *bugs* drift: developers and domain experts agree they understood each other and then ship the wrong thing because "an active customer" meant two different things. Inside one context, ubiquitous language eliminates that drift cheaply. Across contexts, accepting that the same word means different things — and naming the translation explicitly — eliminates the god-object that comes from trying to share one model across two languages. The discipline is naming, but the payoff is correctness.

**How to apply:**
- **Build a glossary per bounded context.** Twenty terms, one sentence each, ratified by domain experts. Living document. Code must use these terms verbatim — no `customer` in code when the glossary says `subscriber`, no `customerId` when the glossary says `subscriberRef`.
- **Rename ruthlessly.** When the glossary says the word is `policy`, rename `record`, `entry`, `agreement`, and `contract` to `policy` across the codebase, the DB schema, the API, the dashboards, and the runbooks. The translation cost is paid once at rename time and saved forever after. (This deliberate, glossary-driven rename *is* the task — it is not the incidental adjacent-code rename that [[coding-discipline]]'s *surgical-changes* warns against; scope it as its own change so the diff stays reviewable.)
- **The same word in two contexts is two different concepts.** Don't fight this — name it. `IdentityCustomer` and `BillingCustomer` can both exist, with different fields, different invariants, and different lifecycles. They share an ID (often) but not a model. The translation between them is an explicit boundary concern (principle 4).
- **The smell that tells you the language is missing:** developers and domain experts have *parallel vocabularies*. Domain expert says "renewal grace period"; engineer says "the field on the subscription table that controls whether we still let them log in for 7 days." Both are correct; the gap is the missing ubiquitous language. Close it by adopting the expert's term in code, not the other way around.
- Ubiquitous language is the part of DDD that most pays off in *boring* domains, because the cost of drift is highest in domains where the rules are intricate and the experts are not engineers (insurance, healthcare, regulated finance, logistics). Don't dismiss it as ceremony.

**Example:**
```
Wrong: insurance system uses `Policy` in the underwriting context (a draft proposal, may not be
       bound, may be quoted but not active), `Policy` in the billing context (a recurring billable
       relationship with start/end dates and a payment schedule), and `Policy` in the claims context
       (a coverage agreement with limits, deductibles, and exclusions). All three are the same
       class with thirty fields, half nullable. Every feature must check "which kind of policy
       state is this?" Every bug is "we treated a quoted policy as a billable one."

Right: three bounded contexts, three named concepts: `Quote` (underwriting), `BillableSubscription`
       (billing), `CoverageAgreement` (claims). Each has its own fields, lifecycle, and language.
       The shared identifier is a `PolicyNumber`. The translation between them lives at explicit
       integration points. No god-object, no nullable-field maze, no "which kind of policy" check
       in business logic.
```

---

### 4. Name the relationship between contexts deliberately

**Rule:** For each pair of bounded contexts that must integrate, pick one of the seven context-mapping patterns and *name it* — in the context map, in the team's vocabulary, and (when relevant) in code (the ACL is a real module, the OHS is a real API). The pattern names what the contract is, who owns the translation, and what the failure modes are.

**Why:** Most cross-context pain is unnamed coupling. Two contexts agreed-by-accident to share a model and now neither can evolve. A downstream team builds against the upstream team's internal API and breaks every time the upstream team refactors. The shared kernel has no maintainer and has become the place where every team's least-favorite code lives. An external vendor's ugly schema has leaked into the heart of your domain because nobody put a translation layer at the edge. Each of these is a context-mapping pattern that was needed and skipped. The seven names exist so two teams can have a five-minute conversation about which one applies, instead of a six-month conversation about who broke the build.

**How to apply (the seven patterns, with when-to-use):**

| Pattern | When to use it | Cost / risk |
|---|---|---|
| **Shared Kernel** — two contexts jointly own a small shared model. | The shared concept is genuinely identical in both contexts and the two teams cooperate closely (often inside one larger team). Examples: a `Money` type, a shared `Address` value object, a small core protobuf used by both sides. | Brittle. Any change requires coordination. Use *sparingly* — most "shared" things turn out to differ subtly between contexts. |
| **Customer / Supplier** — downstream depends on upstream, but downstream has political pull on the upstream's roadmap. | Both teams are inside the same org and the customer's needs are negotiable into the supplier's priorities. The supplier *accepts responsibility* for serving the customer's needs. | Real cooperation cost; works only when both sides actually have the relationship the pattern names. |
| **Conformist** — downstream slavishly accepts whatever upstream publishes, with no translation. | Upstream won't accommodate the downstream (different org, vendor, regulator) AND the downstream model fits cleanly onto the upstream model. | Couples downstream forever to upstream's choices. Acceptable when the upstream model is genuinely fine, dangerous when it's not. |
| **Anticorruption Layer (ACL)** — a translation shell sitting at the boundary, converting the upstream's model into the downstream's language. | Upstream's model leaks concepts you don't want in your domain (vendor APIs, legacy systems, third-party SaaS with idiosyncratic conventions). Most common defensive pattern. | The translation is real code with real cost; pay it deliberately at the boundary, not implicitly throughout the codebase. |
| **Open Host Service (OHS)** — upstream publishes a stable, deliberately-designed protocol for *many* consumers. | The upstream context is consumed by enough downstreams (internal teams, partners, public API users) that pairwise integration would be unmaintainable. Pair with **Published Language** below. | The upstream pays the cost of a stable public surface forever. Worth it when the consumer count is high. |
| **Published Language** — a shared, deliberately-designed interchange schema neither side owns. | Industry-standard or org-standard interchange formats: HL7 in healthcare, FIX in finance, ISO 20022 in payments, internal `OrderCreated.v1` schemas registered in a schema registry. | Governance cost — somebody runs the schema registry / publishes the spec — but the cost replaces N pairwise translations. |
| **Separate Ways** — the explicit decision *not* to integrate; the two contexts coexist without exchanging data. | The integration genuinely isn't worth its cost — the two contexts are duplicates by accident of history, or the data they'd share isn't valuable enough to justify the coupling. | The cost of *not* using this pattern when it applies is dragging an integration nobody needed for years; the cost of using it when it doesn't apply is two systems that should have been one. |

- **A context map is a single diagram listing every pair of integrating bounded contexts and the pattern between them.** Keep it short, one A4 page. Update it when the relationship changes. The diagram itself is half the value — the conversation that produces it is the other half.
- **The most common patterns to reach for in practice:** ACL when integrating with anything outside your team's control; OHS + Published Language when you are the upstream for many consumers; Customer/Supplier when two friendly teams inside the same org integrate; Separate Ways when somebody proposes an integration that nobody actually needs.
- **The most common patterns to be suspicious of:** Shared Kernel (it ages badly) and Conformist (it's often a sign you skipped the ACL conversation).
- See [[bounded-contexts]] for the full pattern catalog with worked examples and the context-map drawing recipe.

**Example:**
```
Wrong: e-commerce platform integrates with a legacy ERP for inventory. The ERP's API returns
       records with cryptic field names (`STK_BAL`, `LOC_CD`, `WHSE_NUM`), embeds business
       logic in stored procedures, and treats null differently from zero. The platform team's
       Inventory context just maps these field names directly into its types. Six months in,
       half the platform's domain model speaks the legacy ERP's language; renaming the ERP
       fields would require touching forty files.

Right: same situation, but the platform team puts an ACL at the boundary. `LegacyErpInventory
       Adapter` consumes the ERP's responses and emits clean `InventoryLevel` values in the
       platform's ubiquitous language. The rest of the codebase sees only the clean types. When
       the ERP eventually changes (or is replaced), the change is contained in one module.
       Total cost: one extra translation layer; total saving: the rest of the system stays
       unpoisoned.
```

---

### 5. Size aggregates around invariants, not entities

**Rule:** An aggregate is a *transactional consistency boundary* — the smallest set of entities and value objects that must change atomically to keep a business invariant true. Design aggregates *small*, reference other aggregates *by identity* (not object reference), modify *one aggregate per transaction*, and let everything outside the boundary be *eventually consistent*. The test: "what business rule is invalidated if these two things are modified in separate transactions?" If you can't name one, they belong in different aggregates.

**Why:** The classic DDD-gone-wrong shape is an oversized aggregate — a `Project` that owns all `Releases` that own all `Sprints` that own all `BacklogItems` — and it fails for a reason every team eventually meets in production: transactional contention. Two users add an item to the same project, hit the same root, the optimistic lock collides, both retries pile up, the page hangs. The cure is to find the actual invariant ("the project must have a name and a code"; "the sprint must not exceed its capacity"; "the backlog item must belong to one sprint at a time") and put exactly the entities needed to enforce *that* invariant inside the boundary. Most invariants live inside a much smaller boundary than the first instinct suggests. Vernon's empirical observation: ~70% of well-designed aggregates are a single root entity holding only value-typed properties; ~30% have 2–3 entities total. Bigger than that is almost always a smell.

**How to apply (Vernon's four rules):**
1. **Model true invariants in consistency boundaries.** An aggregate's job is to enforce one or a few business rules atomically. Everything else lives outside.
2. **Design small aggregates.** Default to root + value-typed properties. Add entities only when an invariant *requires* them to be in the same transaction. Beware `0..*` collections inside an aggregate — they grow unbounded and force the whole graph to load on every operation.
3. **Reference other aggregates by identity.** Hold an `OrderId`, not a reference to the `Order` object. Cross-aggregate modification becomes impossible by construction, which is the property you want.
4. **Use eventual consistency outside the aggregate.** When two aggregates need to coordinate (a state change in one drives a state change in another), the path is: aggregate A commits → emits a domain event → handler updates aggregate B in a *separate* transaction. Combine with the **outbox** pattern (see [[queue-fundamentals]]) so the event is published exactly as often as the write commits.

- **The sizing test.** Before you put two entities in the same aggregate, name the invariant that would break if they were in separate transactions. If you can't, they belong in separate aggregates linked by ID. "It's convenient to load them together" is not an invariant; that's a query concern, solvable with a read model.
- **The collection trap.** "An aggregate that contains a list of items it owns" is the most common path to oversized aggregates. If the list is bounded and small (a `Cart` with at most ~50 line items), it's fine inside. If the list is unbounded (`Project.backlogItems`, `Customer.orders`), the items are their own aggregates referenced by the parent's ID.
- **When the rule pushes back:** sometimes the business *does* require strong consistency across what looks like two aggregates. That's a signal they're actually one aggregate. Merge them — but verify the invariant is real by asking the domain expert "what breaks if these are inconsistent for 200ms?"
- **Aggregates and the rest of the library:** the aggregate is the shape; the *code inside* it is governed by [[programming-fundamentals]] (illegal states unrepresentable, invariants enforced in constructors, no setters that bypass rules). The aggregate is *persisted via* [[hexagonal-backend]]'s repository ports. The transaction it owns is governed by [[database-fundamentals]]. The events it emits cross to [[queue-fundamentals]] when they leave the context.
- See [[aggregate-design]] for the four rules with worked examples, the sizing test, the Vernon "split the Scrum aggregate" walkthrough, and the relationship between aggregates and bounded contexts.

**Example:**
```
Wrong: a `Project` aggregate owns `Release` objects which own `Sprint` objects which own
       `BacklogItem` objects. Adding a backlog item loads the entire project graph, modifies the
       root, and saves. Two users adding items concurrently fight for the root's optimistic lock.
       Under load, throughput collapses. Splitting later is hard because every operation reaches
       through the root.

Right: four aggregates — `Project` (id, name, code, owner), `Release` (id, project_id, name,
       window), `Sprint` (id, release_id, capacity, start, end), `BacklogItem` (id, sprint_id,
       title, story_points, state). Each aggregate holds *its own* invariants:
       Sprint enforces "sum of story_points ≤ capacity"; BacklogItem enforces "state transitions
       follow the rule (open → in_progress → done, no skip)". Cross-aggregate moves (assign item
       to sprint, close a sprint) happen via domain events handled in separate transactions.
       Throughput scales with the number of aggregates touched, not with the size of the project.
```

---

### 6. Separate internal domain events from cross-context integration events

**Rule:** Domain events that live *inside* one bounded context can be rich, evolve freely, and use the context's full ubiquitous language. Events that cross *into* another bounded context (or out to external consumers) are a *contract* — narrower, versioned, deliberately stable, often a different shape from the internal event. Don't publish your internal event directly as an integration event; that couples your model evolution to every downstream consumer forever.

**Why:** Internal domain events and integration events look similar from a distance — both are "a thing happened, with some payload" — but they have completely different lifecycle properties. Internal events are *implementation detail*: when you rename a field, restructure an aggregate, or split a context, you change the internal events too, in lockstep, in one repo. Integration events are *contract*: every change has to respect every downstream consumer, deploy on a different schedule, and survive consumer versions that have not yet adopted the change. Conflating them is one of the top three DDD failure modes in modern practice — every internal refactor breaks downstream consumers, the upstream team is now responsible for downstream rollouts, and "small" changes ship over quarters instead of afternoons.

**How to apply:**
- **Two distinct event types in the codebase.** `OrderConfirmed` (internal domain event, rich, may change with the model) and `OrderConfirmedV1` (integration event, narrow contract, versioned, lives in a schema registry). The integration event is *produced from* the internal event at the boundary, never *equated* to it.
- **Integration events are smaller, not bigger.** Strip internal-only fields, denormalise the parts downstream consumers actually need, version explicitly. Tolerant readers (consumers ignore unknown fields, producers don't depend on consumers having every field) make backwards-compatible evolution safe.
- **Outbox pattern for the publish.** Internal events are emitted by the aggregate inside the same DB transaction as the state change; an outbox table captures them; a relay publishes integration events (the translated, contract-shaped version) to the broker. This is [[queue-fundamentals]]' territory operationally — this skill's contribution is *which event is which* and *where the translation happens*.
- **Naming convention.** Domain events use the ubiquitous language verbatim (`PolicyBound`, `ClaimSubmitted`, `RiskRecalculated`). Integration events use the same verb but with a version suffix and (often) a shorter payload (`PolicyBoundV1`, `ClaimSubmittedV1`). The version is part of the event name, not a header — making it visible in code prevents accidental breakage.
- **Schema registries are the right tool here.** Avro / Protobuf / JSON Schema with a registry encodes compatibility rules (backward, forward, full) and rejects breaking changes in CI before they ship. See [[queue-fundamentals]] for the operational mechanics.
- **The cross-context boundary is the same line as the language boundary (principle 3).** An integration event that crosses contexts is *also* the place where the ubiquitous language changes. The translation happens at the same boundary as the schema flattening.

**Example:**
```
Wrong: Billing publishes its internal `InvoiceCreated` event directly to the broker. The event
       carries internal fields — `pricing_engine_version`, `applied_promotion_tree`,
       `tax_jurisdiction_resolution_trace` — that exist only to help Billing debug itself. Three
       downstream consumers code against these fields. Six months later, Billing rewrites its
       pricing engine; the internal event shape changes; all three consumers break overnight,
       even though the *business* event (an invoice was created) didn't change at all.

Right: Billing emits an internal `InvoiceCreated` domain event into its own aggregate and outbox.
       A relay reads the outbox, translates each internal event into `InvoiceCreatedV1` — a small,
       stable contract: invoice id, customer ref, amount, currency, line items, due date — and
       publishes that to the broker. When the pricing engine rewrites internally, the internal
       event changes freely; `InvoiceCreatedV1` stays the same; downstream consumers see no
       change. Internal evolution and external contract are decoupled.
```

---

## Pre-flight checklist

Before doing strategic DDD work (designing or naming a context, integrating with another team, sizing an aggregate, running a discovery workshop, naming an event contract), run through these:

1. **Subdomain classification:** is the subdomain I'm working on **core**, **supporting**, or **generic**? Have I checked that I'm not applying full DDD to a generic subdomain (over-engineering) or shipping a core subdomain as anemic CRUD (throwing away the moat)?
2. **Discovery:** if the boundaries aren't already known and the domain has real complexity, have I run (or planned to run) an Event Storming or Domain Storytelling session with the actual domain experts? Or am I drawing boxes around nouns I made up myself?
3. **Ubiquitous language:** for each bounded context I'm designing in, can I produce a 20-term glossary that the domain experts would ratify? Does the code I'm about to write use those exact terms? If a word means different things in two contexts, have I named the two concepts distinctly?
4. **Context relationships:** for each pair of contexts that must integrate, have I named which of the seven patterns applies (shared kernel, customer/supplier, conformist, ACL, OHS, published language, separate ways)? Is the context map drawn and shared with the teams involved?
5. **Aggregate sizing:** for each aggregate I'm designing, can I name the *specific business invariant* that requires its entities to change atomically? If I can't, can I split it? Am I referencing other aggregates by ID, not object reference?
6. **Domain vs integration events:** for each event I'm emitting, is it *internal* (rich, evolves with the model) or *integration* (contract, versioned, deliberately narrow)? Where is the translation? Where is the outbox?

If any answer is "I don't know," stop and find out before cementing the design in code.

## Failure-mode diagnostics

When a DDD-shaped codebase has gone wrong, the failure usually has one of these names. Match the symptom to the structural fix.

| Symptom | Failure | Fix |
|---|---|---|
| Entities are bags of getters/setters; all logic lives in `*Service` classes; multiple services duplicate the same state-change logic. | **Anemic domain model.** | Move invariants into entity methods (`order.cancel()` not `orderService.cancel(order)`); private setters; constructors validate; see [[programming-fundamentals]] for illegal-state elimination. |
| Transactional contention under load; long-running locks; large object graphs loaded for small operations. | **Aggregate too big.** | Find the true invariants (principle 5); split along them; reference by ID; integrate with domain events. |
| Repeated cross-aggregate "transactions" via try/catch + compensating writes; business rules silently violated under concurrency. | **Aggregate too small.** | Merge — the missing invariant *is* the boundary. |
| Internal domain events published directly as integration events; downstream breaks every time you rename a field internally. | **Leaky bounded context.** | Separate domain and integration events (principle 6); introduce ACL or Published Language at the boundary. |
| Developers use technical terms ("record", "row", "DTO") in standups; domain experts use different words than the code; bugs trace back to "we thought we agreed." | **Missing ubiquitous language.** | Glossary per context (principle 3); rename in code; ban the translation tax. |
| Team applies aggregates, repositories, and value objects to a 5-table admin tool; pays full DDD tax for zero leverage. | **Strategic skipped.** | Classify the subdomain (principle 1); if generic or trivial supporting, drop most of the tactical DDD and use boring CRUD with rich-enough types. |
| Internal field rename ripples to forty files because every layer speaks the upstream vendor's vocabulary. | **Missing ACL.** | Insert a translation layer at the boundary (principle 4); rename inside the ACL; the rest of the codebase speaks your context's language. |
| Two teams co-own a "shared" library that nobody actually maintains; both teams' least-favorite code lives there. | **Shared Kernel rot.** | Either give the kernel an owner (one team accepts maintenance responsibility) or split it (each team takes the part that's actually theirs); shared-kernel-by-default is the trap. |

These diagnostics are most useful in code review and architectural reviews — a missing-language smell catches the issue at write time, an aggregate-too-big smell catches it at design time, before the production incident.

## Below the principles: things that matter but aren't full principles

A few operational concerns that don't earn their own principle but cannot be skipped on real strategic-DDD work.

- **Domain experts are the bottleneck, not the engineers.** Strategic DDD's primary lever is the developer/domain-expert conversation — Event Storming, glossary ratification, invariant clarification. If you don't have access to domain experts (or domain expertise is locked in one person's head and they have no time), most of the strategic-DDD payoff is unavailable to you. Tactical patterns can still help, but you're in DDD Lite territory (~30% of the value, per Three Dots Labs).
- **Context boundaries can be logical, not physical.** Two bounded contexts can live inside one deployable (a well-modularised monolith with explicit module boundaries) just as easily as in two services. The boundary is about *model and language*; the deployment shape is a separate decision (covered in [[architecture-fundamentals]]). Starting with logical boundaries inside a monolith is the safest default; extract to services when a specific context has earned its way out (strangler fig).
- **CQRS and Event Sourcing are amplifiers, not requirements.** They pair naturally with DDD's aggregate-as-write-model — aggregates emit events; reads are projections; commands go through aggregate command handlers — but they have their own cost and their own failure modes. Reach for them only when the read/write asymmetry, audit-trail requirements, or replayability concerns genuinely justify them. Plain CRUD-shaped persistence with rich aggregates is a fine starting shape for most contexts.
- **Conway's Law is a design input.** One bounded context per stream-aligned team is the modern default ([[architecture-fundamentals]] covers this from the system side). If the team shape doesn't fit the context shape, either reorganize the teams or accept that the context will fight you.
- **ORMs and aggregates have an awkward relationship.** Lazy loading, cascade rules, identity tracking — ORMs are built for the entity-graph view, and aggregates push back on parts of that. The compromises (eager-load the whole aggregate, suppress lazy loading inside the boundary, use repositories that hide the ORM specifics) are workable but not free. Worth noting in advance.

## When to skip this skill

- Generic CRUD with no real domain logic (a 5-table admin tool, an internal config UI, a thin reporting view) — applying aggregates, repositories, and value objects to it is the canonical over-engineering case. Use boring CRUD with rich-enough types ([[programming-fundamentals]]) and call it done.
- Throwaway prototypes, internal scripts, one-shot data fixes, proofs-of-concept meant to be deleted before they touch production. Skip the strategic pre-flight; just ship.
- Single-context features with no cross-boundary concerns — the work happens entirely inside one context whose boundaries are already known. Run [[programming-fundamentals]] + [[hexagonal-backend]] + [[database-fundamentals]]; this skill has nothing to add until the work crosses a boundary or touches a model.
- Pure code-level work (renaming a function, adding a parameter, fixing a bug whose cause is on the screen). [[debug-fundamentals]] or [[programming-fundamentals]] own that.
- Pure architecture-level operational decisions (timeouts, retries, instrumentation, service mesh) — that's [[architecture-fundamentals]]' territory. Strategic DDD informs *what* sits behind each boundary; architecture-fundamentals operates it.
- Domains where you have no access to domain experts and no domain documentation. The discovery workshops require domain experts. Without them, fall back to whatever model you can reconstruct from the existing code and tickets, accept you're in DDD Lite, and apply [[hexagonal-backend]] + [[programming-fundamentals]] inside whatever boundaries already exist.

For everything else — any system where the business rules are intricate enough that two people would naturally talk about them in different vocabularies, where the same noun means different things in different parts of the company, where integration with other teams or vendors is a recurring source of friction — these strategic fundamentals apply. They apply on the small feature too, especially the one where "we'll figure out the model later."

## Reference files

Deeper guides for individual principles. Read the one that matches the work in front of you; you don't need to read them all upfront.

- `references/subdomain-classification.md` — core/supporting/generic with the differentiation test, the relationship to Wardley mapping, the build-vs-buy decision, when subdomains drift between classifications, and worked examples. Use for principle 1.
- `references/bounded-contexts.md` — bounded contexts as linguistic + model boundaries, the glossary-per-context discipline, the seven context-mapping patterns with concrete when-to-use, the context-map drawing recipe, anti-corruption layers in depth, and boundary smells. Use for principles 3 and 4.
- `references/event-storming.md` — Big Picture / Process Modelling / Software Design workshop formats, the sticky-color grammar, facilitation tips, Domain Storytelling as the lighter alternative, and the concrete artifact-to-code mapping path. Use for principle 2.
- `references/aggregate-design.md` — Vernon's four rules with worked examples, the sizing test, the Scrum-aggregate split walkthrough, the relationship between aggregates and bounded contexts, and ORM/persistence concerns. Use for principle 5.

## How to use this skill in a conversation

This skill is always-on for strategic-DDD work (per the project rule at `.claude/rules/ddd-strategic.md`). Don't ask the user to opt in. If the task is in "When to skip," say so in one sentence and proceed without it. When working alongside the construction-fundamentals skills, run order is: this skill *first* (decide where the model lives and what language it speaks), then [[programming-fundamentals]] / [[database-fundamentals]] / [[hexagonal-backend]] / [[architecture-fundamentals]] / [[queue-fundamentals]] to build what this skill placed.
