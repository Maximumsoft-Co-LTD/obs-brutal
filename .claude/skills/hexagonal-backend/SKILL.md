---
name: hexagonal-backend
description: Apply hexagonal architecture (ports & adapters) when designing or implementing backend code. Use this skill BEFORE writing or restructuring any backend that has business logic — services, APIs, repositories, use cases, domain models, persistence, message handling. Triggers on user requests involving "backend", "API", "service", "repository", "use case", "domain", "controller", "handler", "DAO", "persistence", "business logic", or when starting/refactoring a server-side codebase. Use it even when the user does not explicitly say "hexagonal" — the trigger is backend work with real domain logic. The skill defines the 3-layer structure (domain / application / infrastructure), port and adapter conventions, dependency direction rules, testing strategy, and common pitfalls, with language-agnostic examples plus TypeScript and Go snippets. Skip only for throwaway scripts or trivial CRUD with no real domain logic.
---

# Hexagonal Backend Architecture (Ports & Adapters)

## Why this exists

The user's standing preference: every backend with real domain logic should be built using hexagonal architecture. The goal is **resilience to requirement changes** — swapping a database, web framework, message broker, or external API should touch only adapters, never the core business logic.

If a future requirement asks "can we move from Postgres to DynamoDB?" or "can we expose this over gRPC instead of REST?", a hexagonal codebase answers in days. A coupled codebase answers in months.

## The 3 layers

### Domain (core)
- Entities, value objects, domain services, business rules
- **Zero external dependencies** — no ORM imports, no HTTP libs, no framework types, no `fetch`, no `db.query`
- Pure functions and plain types only
- Must compile and test in isolation

### Application (use cases)
- Orchestrates domain logic to fulfill a single user-facing intent (`PlaceOrder`, `CancelSubscription`, `RefundPayment`)
- Defines **ports** — interfaces describing what it needs from the outside world (`OrderRepository`, `PaymentGateway`, `EmailSender`)
- Depends on: domain only. Never imports infrastructure.

### Infrastructure (adapters)
Two flavors:
- **Driving adapters** (call into the application): HTTP controllers, gRPC handlers, CLI commands, message consumers, cron jobs
- **Driven adapters** (called by the application via ports): DB repositories, external API clients, message publishers, file storage, email senders

## The one rule you must not break

**Dependency direction: Infrastructure → Application → Domain**

Never the other way. Domain must compile without adapters. Application must compile without adapters. If a domain file imports anything from `adapters/` (or any concrete framework/library), the architecture is broken.

## Folder structure

### TypeScript / Node example
```
src/
  domain/
    order/
      order.ts                 # entity
      order-status.ts          # value object
      order-id.ts              # value object
      order-errors.ts          # domain errors
  application/
    ports/
      order-repository.ts      # driven port (interface)
      payment-gateway.ts       # driven port (interface)
      clock.ts                 # driven port — even time is a port
    use-cases/
      place-order.ts
      cancel-order.ts
  adapters/
    driving/                   # call INTO the application
      http/
        order-controller.ts          # HTTP driving adapter
    driven/                    # called BY the application through ports
      persistence/
        postgres-order-adapter.ts    # implements OrderRepository
      payment/
        stripe-payment-adapter.ts    # implements PaymentGateway
      clock/
        system-clock-adapter.ts      # implements Clock
  config/
    composition-root.ts        # wires concrete adapters into use cases
```

### Go example
```
internal/
  domain/
    order/
      order.go
      status.go
      errors.go
  app/
    ports/
      order_repository.go      # driven port (interface)
      payment_gateway.go       # driven port (interface)
    usecase/
      place_order.go
      cancel_order.go
  adapters/
    driving/                   # call INTO the application
      http/
        order_handler.go       # HTTP driving adapter
    driven/                    # called BY the application through ports
      postgres/
        order_adapter.go       # implements ports.OrderRepository
      stripe/
        payment_adapter.go     # implements ports.PaymentGateway
  config/
    wire.go                    # composition root
cmd/
  api/
    main.go                    # entrypoint — assembles everything
```

## Patterns

### Port definition (TypeScript)
```ts
// application/ports/order-repository.ts
import type { Order, OrderId } from '../../domain/order/order'

export interface OrderRepository {
  save(order: Order): Promise<void>
  findById(id: OrderId): Promise<Order | null>
  findByCustomer(customerId: CustomerId): Promise<Order[]>
}
```

### Port definition (Go)
```go
// internal/app/ports/order_repository.go
package ports

import "myapp/internal/domain/order"

type OrderRepository interface {
    Save(ctx context.Context, o *order.Order) error
    FindByID(ctx context.Context, id order.ID) (*order.Order, error)
}
```

### Adapter (TypeScript)
```ts
// adapters/driven/persistence/postgres-order-adapter.ts
import type { Pool } from 'pg'
import type { OrderRepository } from '../../../application/ports/order-repository'
import { Order } from '../../../domain/order/order'

export class PostgresOrderAdapter implements OrderRepository {
  constructor(private readonly db: Pool) {}

  async save(order: Order): Promise<void> {
    await this.db.query(
      'INSERT INTO orders (id, status, total) VALUES ($1, $2, $3) ' +
      'ON CONFLICT (id) DO UPDATE SET status = $2, total = $3',
      [order.id.value, order.status, order.total.amount],
    )
  }

  async findById(id: OrderId): Promise<Order | null> {
    const result = await this.db.query('SELECT * FROM orders WHERE id = $1', [id.value])
    if (result.rows.length === 0) return null
    return this.toDomain(result.rows[0])
  }

  private toDomain(row: OrderRow): Order {
    // Mapping lives in the adapter, not the domain. `rehydrate` is a static
    // factory that skips invariant checks — the row is already valid because
    // the domain wrote it in the first place.
    return Order.rehydrate({
      id: new OrderId(row.id),
      customerId: new CustomerId(row.customer_id),
      status: row.status as OrderStatus,
      total: new Money(row.total_amount, row.total_currency),
      placedAt: new Date(row.placed_at),
    })
  }
}
```

### Adapter (Go)
```go
// internal/adapters/driven/postgres/order_adapter.go
package postgres

import (
    "context"
    "database/sql"
    "errors"

    "myapp/internal/app/ports"
    "myapp/internal/domain/order"
)

type OrderAdapter struct {
    db *sql.DB
}

func NewOrderAdapter(db *sql.DB) *OrderAdapter {
    return &OrderAdapter{db: db}
}

// Compile-time check that we satisfy the port. If the port changes
// shape, this line breaks before runtime does.
var _ ports.OrderRepository = (*OrderAdapter)(nil)

func (r *OrderAdapter) Save(ctx context.Context, o *order.Order) error {
    _, err := r.db.ExecContext(ctx, `
        INSERT INTO orders (id, customer_id, status, total_amount, total_currency)
        VALUES ($1, $2, $3, $4, $5)
        ON CONFLICT (id) DO UPDATE
          SET status = EXCLUDED.status,
              total_amount = EXCLUDED.total_amount`,
        o.ID().String(), o.CustomerID().String(), o.Status(),
        o.Total().Amount(), o.Total().Currency(),
    )
    return err
}

func (r *OrderAdapter) FindByID(ctx context.Context, id order.ID) (*order.Order, error) {
    var row orderRow
    err := r.db.QueryRowContext(ctx,
        `SELECT id, customer_id, status, total_amount, total_currency
         FROM orders WHERE id = $1`, id.String(),
    ).Scan(&row.id, &row.customerID, &row.status, &row.amount, &row.currency)
    if errors.Is(err, sql.ErrNoRows) {
        return nil, nil
    }
    if err != nil {
        return nil, err
    }
    return r.toDomain(row), nil
}

func (r *OrderAdapter) toDomain(row orderRow) *order.Order {
    // Mapping stays in the adapter. `Rehydrate` is the package-level
    // factory that skips invariant checks — storage holds already-valid state.
    return order.Rehydrate(
        order.ID(row.id),
        order.CustomerID(row.customerID),
        order.Status(row.status),
        order.NewMoney(row.amount, row.currency),
    )
}

type orderRow struct {
    id, customerID, status, currency string
    amount                           int64
}
```

### Use case (TypeScript)
```ts
// application/use-cases/place-order.ts
import { Order } from '../../domain/order/order'
import type { OrderRepository } from '../ports/order-repository'
import type { PaymentGateway } from '../ports/payment-gateway'

export interface PlaceOrderCommand {
  customerId: string
  items: Array<{ sku: string; qty: number }>
  paymentMethodId: string
}

export class PlaceOrder {
  constructor(
    private readonly orders: OrderRepository,
    private readonly payments: PaymentGateway,
  ) {}

  async execute(cmd: PlaceOrderCommand): Promise<OrderId> {
    const order = Order.create(cmd)                 // domain logic
    await this.payments.charge(order.total, cmd.paymentMethodId)  // port
    order.markPaid()                                 // domain logic
    await this.orders.save(order)                    // port
    return order.id
  }
}
```

### Use case (Go)
```go
// internal/app/usecase/place_order.go
package usecase

import (
    "context"

    "myapp/internal/app/ports"
    "myapp/internal/domain/order"
)

type PlaceOrderCommand struct {
    CustomerID      string
    Items           []order.LineItem
    PaymentMethodID string
}

type PlaceOrder struct {
    orders   ports.OrderRepository
    payments ports.PaymentGateway
}

func NewPlaceOrder(o ports.OrderRepository, p ports.PaymentGateway) *PlaceOrder {
    return &PlaceOrder{orders: o, payments: p}
}

func (uc *PlaceOrder) Execute(ctx context.Context, cmd PlaceOrderCommand) (order.ID, error) {
    o, err := order.Create(cmd.CustomerID, cmd.Items)   // domain logic
    if err != nil {
        return "", err
    }
    if err := uc.payments.Charge(ctx, o.Total(), cmd.PaymentMethodID); err != nil {
        return "", err                                  // port
    }
    o.MarkPaid()                                        // domain logic
    if err := uc.orders.Save(ctx, o); err != nil {
        return "", err                                  // port
    }
    return o.ID(), nil
}
```

### Driving adapter (HTTP controller, TS)
```ts
// adapters/driving/http/order-controller.ts
export function orderRoutes(placeOrder: PlaceOrder) {
  return async (req: Request, res: Response) => {
    try {
      const id = await placeOrder.execute({
        customerId: req.body.customerId,
        items: req.body.items,
        paymentMethodId: req.body.paymentMethodId,
      })
      res.status(201).json({ orderId: id.value })
    } catch (e) {
      // translate domain errors → HTTP status. Translation lives here, not in use case.
      res.status(400).json({ error: e.message })
    }
  }
}
```

### Driving adapter (HTTP handler, Go)
```go
// internal/adapters/driving/http/order_handler.go
package http

import (
    "encoding/json"
    "errors"
    "net/http"

    "myapp/internal/app/usecase"
    "myapp/internal/domain/order"
)

type OrderHandler struct {
    placeOrder *usecase.PlaceOrder
}

func NewOrderHandler(p *usecase.PlaceOrder) *OrderHandler {
    return &OrderHandler{placeOrder: p}
}

func (h *OrderHandler) Create(w http.ResponseWriter, r *http.Request) {
    var body struct {
        CustomerID      string           `json:"customerId"`
        Items           []order.LineItem `json:"items"`
        PaymentMethodID string           `json:"paymentMethodId"`
    }
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
        http.Error(w, "bad request", http.StatusBadRequest)
        return
    }
    id, err := h.placeOrder.Execute(r.Context(), usecase.PlaceOrderCommand{
        CustomerID:      body.CustomerID,
        Items:           body.Items,
        PaymentMethodID: body.PaymentMethodID,
    })
    if err != nil {
        writeError(w, err) // domain/app error → HTTP status, mapping lives here
        return
    }
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    _ = json.NewEncoder(w).Encode(map[string]string{"orderId": id.String()})
}

func writeError(w http.ResponseWriter, err error) {
    switch {
    case errors.Is(err, order.ErrNotFound):
        http.Error(w, "not found", http.StatusNotFound)
    case errors.Is(err, order.ErrInsufficientFunds):
        http.Error(w, "insufficient funds", http.StatusUnprocessableEntity)
    default:
        http.Error(w, "internal error", http.StatusInternalServerError)
    }
}
```

### Composition root
One place wires concrete adapters into use cases. Everywhere else, code receives ports through constructors.

```ts
// config/composition-root.ts
const db = new Pool({ /* ... */ })
const orderRepo = new PostgresOrderAdapter(db)
const payments = new StripePaymentAdapter(stripeClient)
const placeOrder = new PlaceOrder(orderRepo, payments)

app.post('/orders', orderRoutes(placeOrder))
```

```go
// cmd/api/main.go
package main

import (
    "database/sql"
    "log"
    "net/http"

    _ "github.com/lib/pq"
    "myapp/internal/adapters/driven/postgres"
    "myapp/internal/adapters/driven/stripe"
    apphttp "myapp/internal/adapters/driving/http"
    "myapp/internal/app/usecase"
)

func main() {
    db, err := sql.Open("postgres", "...")
    if err != nil {
        log.Fatal(err)
    }

    orderRepo := postgres.NewOrderAdapter(db)
    payments := stripe.NewPaymentAdapter( /* stripe client */ )
    placeOrder := usecase.NewPlaceOrder(orderRepo, payments)

    h := apphttp.NewOrderHandler(placeOrder)
    http.HandleFunc("/orders", h.Create)
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

## Transactions and atomicity

When a use case mutates multiple aggregates that must succeed or fail together, you cannot just call `repo1.save()` then `repo2.save()` — a crash between them leaves data inconsistent. But you also cannot pass a raw `db.Tx` or `mongoose.ClientSession` into the use case, because that re-couples application to infrastructure.

**Unit of Work pattern** — expose a transaction port; the use case hands it a function; the adapter runs the function inside a transaction. Repositories called inside the function pick up the same transaction (via context, request-scoped DI, or an explicit handle bound by the adapter).

```ts
// application/ports/unit-of-work.ts
export interface UnitOfWork {
  run<T>(work: () => Promise<T>): Promise<T>
}

// application/use-cases/transfer-funds.ts
export class TransferFunds {
  constructor(
    private readonly uow: UnitOfWork,
    private readonly accounts: AccountRepository,
  ) {}

  async execute(cmd: TransferCommand): Promise<void> {
    await this.uow.run(async () => {
      const from = await this.accounts.findById(cmd.fromId)
      const to   = await this.accounts.findById(cmd.toId)
      from.debit(cmd.amount)        // domain logic
      to.credit(cmd.amount)         // domain logic
      await this.accounts.save(from)
      await this.accounts.save(to)
    })
  }
}
```

The use case knows transactions exist; it doesn't know Postgres, Mongo sessions, or savepoints. The adapter handles that.

**Crossing systems (DB + message broker)** — don't dual-write. Use a transactional outbox: persist the outgoing message into the same DB transaction as the state change, and let a separate relay process publish it. The outbox table is an adapter concern; the use case sees it only as `orders.save(order)` (the adapter writes both rows).

## Errors: domain → application → adapter

Errors flow outward in three flavors, and each layer translates the layer beneath it.

- **Domain errors** — broken business invariants, in business language: `InsufficientFundsError`, `OrderAlreadyShipped`, `EmailAlreadyTaken`. Live next to the entities that raise them. Carry no HTTP status, no SQL code.
- **Application errors** — broken use-case preconditions that aren't domain rules: `OrderNotFound`, `Unauthorized`, `IdempotencyConflict`. Live in the application layer.
- **Infrastructure errors** — leakage from external systems: `pg.UniqueViolation`, `stripe.CardDeclined`, `dial tcp: i/o timeout`. Adapters catch the ones with domain meaning and re-raise as domain/application errors (a unique constraint on `email` → `EmailAlreadyTaken`). Anything else propagates as a generic infra failure — don't pretend you know how to map every Postgres error code.

Driving adapters translate at the edge — domain/application errors become HTTP status codes, gRPC codes, or CLI exit codes.

```ts
// adapters/driving/http/error-handler.ts
export function toHttp(e: unknown): { status: number; body: object } {
  if (e instanceof InsufficientFundsError) return { status: 422, body: { code: 'INSUFFICIENT_FUNDS' } }
  if (e instanceof OrderNotFound)          return { status: 404, body: { code: 'NOT_FOUND' } }
  if (e instanceof Unauthorized)           return { status: 401, body: { code: 'UNAUTHORIZED' } }
  return { status: 500, body: { code: 'INTERNAL' } }
}
```

Keep this map in the driving adapter, not in the use case. The use case throws; HTTP decides the status.

## Queries that don't fit save/findById

The repository pattern is shaped for the *write* path — `save`, `findById`, `findByCustomer`. When the read path needs pagination, projections, joins across many tables, search, or aggregate stats, forcing it through a repository turns the repo into a god object and usually loads more domain state than the screen needs.

When that happens, introduce a separate **query port** that returns plain DTOs, not domain entities:

```ts
// application/ports/order-queries.ts
export interface OrderQueries {
  listByCustomer(customerId: CustomerId, page: Page): Promise<OrderSummaryDTO[]>
  searchAdminGrid(filter: AdminFilter): Promise<AdminOrderRow[]>
}
```

The adapter can issue whatever SQL, view, or denormalized read it needs without dragging the domain into it. The DTO shape lives in the application layer (it's part of the contract the use case exposes outward) but carries no behavior. This is the CQRS seam: writes go through repositories and the domain; reads go through query ports.

Don't reach for this on day one. Start with the repository. Add a query port the moment the repository grows read-only methods that the domain itself never uses.

## Testing strategy

| Layer | What to test | How |
|---|---|---|
| Domain | Business rules, invariants | Pure unit tests. No mocks. |
| Use case | Orchestration logic | Replace ports with **in-memory fakes** (preferred) or mocks |
| Adapter | Real integration | Integration tests against real DB / real HTTP / testcontainers |

**Rules:**
- Never mock the domain.
- Mock at port boundaries only — never inside the use case body.
- Prefer in-memory fake implementations over mock libraries — they're reusable across tests and force you to keep ports honest.
- Adapter tests don't mock anything inside the adapter — they test the real translation against a real dependency.

### Example: in-memory fake
```ts
// test/fakes/in-memory-order-repository.ts
export class InMemoryOrderRepository implements OrderRepository {
  private store = new Map<string, Order>()
  async save(o: Order) { this.store.set(o.id.value, o) }
  async findById(id: OrderId) { return this.store.get(id.value) ?? null }
}
```

## Common pitfalls (read this before writing code)

- ❌ **ORM model in domain** — importing `@prisma/client` or `gorm.Model` into a domain file. Leaks infrastructure into core. Use plain types in domain; map at the adapter.
- ❌ **Use case returning DB rows or framework types** — should return domain types or primitives. The driving adapter translates to HTTP/JSON.
- ❌ **Domain calling out** — `fetch()`, `db.query()`, `redis.get()` inside domain. Always invert via a port.
- ❌ **Adapter doing business logic** — adapters only translate (DB row ↔ entity, HTTP body ↔ command). Logic stays in use case or domain.
- ❌ **One mega-port** — `interface DataStore { saveOrder, saveUser, savePayment, ... }`. Keep ports narrow and use-case-focused (Interface Segregation Principle).
- ❌ **Hidden time/randomness in domain** — `new Date()` or `Math.random()` in domain. Inject a `Clock` port and a `Random` port. Makes tests deterministic.
- ❌ **Use case depending on framework** — no `express.Request` in use case signatures. Use case takes plain command objects.
- ❌ **Anemic domain** — entities with only getters/setters. Push behavior into the entity (`order.cancel()`, `subscription.renew()`).
- ❌ **Skipping the composition root** — instantiating adapters inside use cases (`new PostgresRepo()`). Always inject via constructor.

## Workflow when starting a new backend feature

1. **Name the use case.** One verb + noun. `PlaceOrder`, `RefundPayment`, `ResetPassword`.
2. **Sketch the domain.** What entities/value objects exist? What invariants must hold?
3. **List the ports the use case needs.** Each external dependency = one port. Keep them narrow.
4. **Write the use case** against ports. No real adapters yet.
5. **Write domain + use case tests** with in-memory fakes for ports.
6. **Implement adapters.** Real DB, real HTTP client, real broker.
7. **Wire the composition root.**
8. **Write adapter integration tests** against real dependencies.

## When NOT to apply strictly

- Throwaway scripts, one-shot migrations, prototypes meant to be deleted
- Trivial CRUD with no real business rules (a glorified spreadsheet)
- Very thin BFFs (backend-for-frontend) that only forward and reshape responses

For everything else — anything with business rules, multiple data sources, or any chance of changing requirements — apply by default.

**Ports are not the speculative abstraction [[coding-discipline]]'s *simplicity-first* warns against.** Simplicity-first says "no abstraction for a single call site"; a port is the *deliberate* seam that buys requirement-change resilience, so it earns its keep everywhere this skill applies. The skip-list above is exactly where simplicity-first wins instead — trivial CRUD and thin BFFs don't need the port.

## Relation to Vertical Slice Architecture

A frequent 2024-2025 critique of Clean / Hexagonal / Onion is that strict layering fragments a single feature across `domain/`, `application/`, and `infrastructure/` directories — a one-line behavior change touches files in three places, and reviewers have to reconstruct the slice in their head. **Vertical Slice Architecture (VSA)** organizes by feature instead: one folder per use case (`features/place-order/`) that *contains* the handler, the request/response types, the validation, and the persistence call, with hexagonal seams only where they earn their cost.

Treat VSA as **complementary, not competing**:

- The two share the same goals (low coupling, testable units, clear seams).
- VSA's per-feature folder is a *physical layout choice*; hexagonal's dependency-direction rule is a *logical invariant*. They compose: organize files by feature, but still keep domain types pure and inject external dependencies through ports inside the slice.
- For CRUD-heavy or read-mostly services, lean VSA — the ceremony of ports/adapters across many layers is overkill when the slice is one query and one mapping.
- For services with real domain invariants, multi-aggregate transactions, or many adapters per use case (HTTP + queue + DB + external API on one feature), the hexagonal-internal layering pays off; the VSA folder structure still works on top of it.

In short: the **logical layering rule from this skill** (domain has zero external dependencies; ports define the interface; adapters depend inward) is the load-bearing piece. Whether you express that as `domain/`, `application/`, `infrastructure/` top-level folders or as feature-scoped slices is a style choice that varies by team and codebase.

## How to use this skill in a conversation

This skill is always-on for backend work with real domain logic (per the project rule at `.claude/rules/hexagonal-backend.md`). Don't ask the user to opt in. If the task matches "When NOT to apply strictly", say so in one sentence and proceed without hexagonal.

When the skill applies:
- **Starting fresh** — propose the folder structure first, then sketch domain entities and ports before writing code.
- **Refactoring** — classify existing code into domain / application / infrastructure. Propose migration in slices, one use case at a time. If you find domain logic tangled in a controller or repository, name it explicitly before extracting.
- **Writing code** — follow the patterns above. When you make a non-obvious call (introducing a `Clock` port, splitting a repository, reaching for a unit-of-work, adding a query port), say *why* in one sentence. Cite specific pitfalls when relevant — don't just emit code silently.
