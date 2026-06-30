---
name: programming-fundamentals
description: Apply the high-leverage fundamentals that govern any non-trivial piece of code — data modeling, illegal-state elimination, function design, pure core / effectful shell, error handling as values, complexity awareness, and naming. Use this skill BEFORE writing or modifying a function, module, script, or data structure of any meaningful size, in any language. Triggers on requests like "implement", "write a function", "add a feature", "refactor", "design a module", "model this data", "fix this bug", "improve this code", "review this code", "what's wrong with this", or whenever code is about to be produced or changed. Use it even when the user does not name a principle — the trigger is non-trivial code work. The skill provides a 7-principle pre-flight checklist with language-agnostic examples, plus reference files covering naming, error handling, complexity, and testing. Skip only for one-line shell commands, throwaway one-off scripts, or pure config edits with no logic.
---

# Programming Fundamentals

## Why this exists

Most code defects, hard-to-debug systems, and painful rewrites trace back to the same handful of missed fundamentals. Architecture — one service's layering via [[hexagonal-backend]], the system-level shape via [[architecture-fundamentals]] — and refactoring (via /simplify) sit on top of these. If the fundamentals are shaky, no architecture saves you and no refactor sticks.

This skill is a **pre-flight**: read it before you write the code, not after. The principles are language-agnostic — they apply equally to a 30-line Python data script and a 30-file Go service.

## The 7 principles

Each principle has a one-line rule, a *why*, and a worked example. Apply them in roughly this order — the early ones unblock the later ones.

---

### 1. Model the data first

**Rule:** Decide the shape of the data before you decide the operations. Operations follow from data; if the data is wrong, the operations will be ugly forever.

**Why:** Most "complex logic" is actually a symptom of an awkward data shape. A `Map<UserId, List<Order>>` makes "find orders by user" trivial. A flat `List<Order>` makes it an N-pass scan and invites bugs every time someone forgets to filter.

**How to apply:**
- Before writing the function, sketch the input type and the output type. If you can't write them down, you don't yet understand the problem.
- Prefer the most constrained shape that holds the data you actually have. Don't use a `List` if there are no duplicates — use a `Set`. Don't use a `Map<String, Any>` if the keys are known — use a struct/record.
- Group fields that travel together into one type. If three parameters always appear together, they're really one thing.

**Example:**
```ts
// Awkward — caller juggles three parallel arrays, easy to misalign
function chargeAll(userIds: string[], amounts: number[], currencies: string[]) { ... }

// Better — the shape mirrors the domain; misalignment is impossible
type Charge = { userId: UserId; amount: Money }
function chargeAll(charges: Charge[]) { ... }
```

---

### 2. Make illegal states unrepresentable

**Rule:** Use types, enums, sum types, smart constructors, and structure to ensure that wrong states can't even be written down.

**Why:** Every runtime check for "did the caller pass nonsense?" is a check you'll forget to write somewhere, or that you'll write five different ways across the codebase. Push the check into the type system or the constructor, and the compiler/runtime enforces it once, forever.

**How to apply:**
- Replace `String` with a typed wrapper when the string has rules (`Email`, `UserId`, `IsoCountryCode`). The constructor validates once; everyone downstream trusts it.
- Replace `boolean` flags that combine with other flags. If you have `isLoading`, `isError`, `data`, and only certain combinations are valid, model it as a sum type: `Loading | Error(e) | Ready(data)`.
- Replace nullable fields whose nullability is conditional. Instead of `User { email?: string; emailVerifiedAt?: Date }` with the rule "verified implies email is set", model two states: `Unverified(email?) | Verified(email, at)`.

**Example:**
```ts
// Bad — four boolean combinations, only some legal
type RequestState = { isLoading: boolean; error: Error | null; data: T | null }

// Good — three legal states, nothing else compiles
type RequestState<T> =
  | { tag: 'loading' }
  | { tag: 'error'; error: Error }
  | { tag: 'ready'; data: T }
```

---

### 3. Functions do one thing, named for what they do

**Rule:** A function should have one clear job and a name that states it. If you can't name it crisply without "and", it's doing too much.

**Why:** A function name is a contract with the reader: "you don't have to read my body to know what I do." When the name lies or hedges, every caller has to read the body, which defeats the whole point of having a function.

**How to apply:**
- Read your function's name and ask: does the body do exactly that, nothing more, nothing less? If it also writes to the database, also sends an email, and also returns a thing, the name is hiding two of those.
- Functions that return a value should be named for what they return (`activeUsers()`, `totalCents()`). Functions that act should be named for the action (`sendInvoice()`, `markPaid()`).
- A function that mixes a query with a command (returns data AND mutates state) is almost always two functions trying to share a body. Split them.
- See [[naming]] for the deeper guide.

**Example:**
```py
# Bad — name says "get", body also writes
def get_user_and_log_access(id):
    user = db.fetch(id)
    audit_log.append({"accessed": id, "at": now()})
    return user

# Good — two functions, two names, one job each
def find_user(id): return db.fetch(id)
def record_access(id): audit_log.append({"accessed": id, "at": now()})
```

---

### 4. Pure core, effectful shell

**Rule:** Push side effects (I/O, time, randomness, network, DB) to the edges. Keep the logic in the middle pure.

**Why:** Pure functions are testable, composable, and reasonable. The moment a function reaches out to a clock, a network, or a database, you can no longer reason about it locally — you need the whole world. Concentrate that pain at the boundary and the inner 80% of your code stays cheap to test and change.

**How to apply:**
- A function that takes inputs and returns outputs, with no hidden reads or writes, is pure. Aim for this whenever the logic doesn't *need* I/O.
- Pass time, randomness, and other "ambient" effects as arguments (or via injected ports — see [[hexagonal-backend]]). `priceFor(order, now)` is testable; `priceFor(order)` that secretly calls `Date.now()` is not.
- Sequence: load the data → compute → write the result. Don't interleave reads and computation, or your pure logic becomes infected with retries and timeouts.

**Example:**
```ts
// Bad — logic and I/O tangled; can't test the math without a DB
async function applyDiscount(orderId: string) {
  const order = await db.findOrder(orderId)
  const discount = order.total > 100 ? order.total * 0.1 : 0
  await db.update(orderId, { discount })
}

// Good — pure rule, separate I/O
function discountFor(total: Money): Money {
  return total.cents > 10_000 ? total.scale(0.1) : Money.zero(total.currency)
}
// shell calls: load → discountFor → save
```

---

### 5. Errors are values; handle them where you have context

**Rule:** Treat errors as first-class data. Never silently swallow. Handle at the layer that has enough information to do something useful — otherwise propagate.

**Why:** A `try { ... } catch (e) {}` is a time bomb: the program continues in a state it doesn't understand, and you debug the *consequences* hours later. The right question is always "who has enough context to decide what to do here?" — and that's rarely the function that first noticed.

**How to apply:**
- At the function that *can* recover (retry, fall back to a default, ask the user), handle it. Everywhere else, propagate.
- Distinguish *expected* failures (validation, not found) from *bugs* (null pointer, invariant violation). Expected failures belong in the return type (`Result`, `Either`, tagged union, error return). Bugs should crash loudly.
- At system boundaries (HTTP handlers, CLI entry points, message consumers), catch everything, log with context, and translate to the boundary's error model.
- See [[error-handling]] for the deeper guide.

**Example:**
```go
// Bad — swallows the error, keeps going on garbage
data, _ := json.Unmarshal(input)
process(data)

// Good — propagate to the caller who knows what to do
data, err := json.Unmarshal(input)
if err != nil {
    return fmt.Errorf("parse input: %w", err)
}
return process(data)
```

---

### 6. Mind complexity — know your Big O

**Rule:** Know the time and space complexity of the data structures and loops you use. Watch for accidentally quadratic patterns.

**Why:** Most performance disasters are not exotic — they're an `O(n)` lookup nested inside an `O(n)` loop, run on data that grew from 10 rows in dev to 1M rows in prod. You don't need to micro-optimize; you just need to not write `O(n²)` when `O(n)` is one `Map` away.

**How to apply:**
- For every loop, ask: what's the size of what I'm iterating, and what's the cost of one iteration? Multiply.
- A `list.includes(x)` or `array.find(...)` inside a loop over the same list is `O(n²)`. Build a `Set` or `Map` once, look up in `O(1)`.
- Know your data structures' costs: array append is amortized `O(1)`, array prepend is `O(n)`; hash map lookup is `O(1)` average, tree map is `O(log n)`; sorting is `O(n log n)`.
- Don't optimize until you've measured — but don't choose `O(n²)` when `O(n)` is the same code length.
- See [[complexity]] for the deeper guide.

**Example:**
```ts
// Bad — O(orders × customers)
const enriched = orders.map(o => ({
  ...o,
  customerName: customers.find(c => c.id === o.customerId)?.name,
}))

// Good — O(orders + customers)
const byId = new Map(customers.map(c => [c.id, c]))
const enriched = orders.map(o => ({
  ...o,
  customerName: byId.get(o.customerId)?.name,
}))
```

---

### 7. Read before you write

**Rule:** Read the surrounding code, the error message, and the actual failing function before writing or guessing.

**Why:** Most "bug fixes" that don't work are fixes for a problem the engineer imagined, not the one that's actually happening. Most "new code" that doesn't fit was written without looking at how the rest of the codebase solves the same shape of problem. Reading is cheap; rewriting is expensive.

**How to apply:**
- When debugging: read the full error message, including the stack trace. Most bugs name themselves in the first line.
- Before adding a new utility, grep for existing ones doing the same thing. Adopt the project's conventions even if they aren't your favorite — consistency is a feature.
- Before changing a function, read its callers. The function's contract is what callers depend on, not what its docstring claims.

---

## Pre-flight checklist

Before writing or substantially changing code, run through these in your head:

1. **Data:** what are the input and output types? Are they as constrained as the domain allows?
2. **Illegal states:** can a caller pass something that compiles but is semantically wrong? Can the type system stop them?
3. **One thing:** does each new function have a name you can say without "and"?
4. **Pure where possible:** are the I/O calls pushed to the edge, with pure logic in the middle?
5. **Errors:** does every failure path either recover here, propagate with context, or crash loudly? No silent swallows.
6. **Complexity:** any nested loops over the same collection? Any `O(n)` lookup inside an `O(n)` loop?
7. **Read first:** have I read the existing code, the error message, and the callers?

If any answer is "I don't know," stop and find out before writing.

## When to skip this skill

- One-line shell commands or trivial REPL exploration.
- Pure config edits with no logic (env vars, package versions, formatter rules).
- Throwaway scripts you will delete in the next hour.

For anything else — yes, even the "small" feature, even the "quick" fix — these fundamentals apply.

## Reference files

Deeper guides for individual principles. Read the one that matches the work in front of you; you don't need to read them all upfront.

- `references/naming.md` — variables, functions, types, files, commit messages.
- `references/error-handling.md` — Result/Either patterns, exceptions vs return values, boundary handling.
- `references/complexity.md` — Big O cheat sheet, common accidentally-quadratic patterns, profiling first principles.
- `references/testing.md` — what to test, what not to test, fast tests vs slow tests, test-behavior-not-implementation.
