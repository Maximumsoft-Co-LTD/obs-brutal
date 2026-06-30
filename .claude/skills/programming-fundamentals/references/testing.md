# Testing

A good test suite catches regressions and gives you the confidence to change things. A bad test suite breaks every time you rename a variable and tells you nothing about whether the system works. The difference is what you choose to test.

## The rule

**Test the behavior, not the implementation.**

- A behavior test asks: "given this input, does the system produce the right outcome?"
- An implementation test asks: "did this function call this other function in this order?"

Behavior tests survive refactors. Implementation tests punish you for refactoring.

## What to test

In rough priority order:

1. **Pure logic with branches.** Pricing rules, validation, state transitions, parsers, formatters. High value, fast tests, easy to write — they're the backbone.
2. **Contracts at boundaries.** What does the HTTP endpoint return for valid input? Invalid input? Auth failures? This is where regressions hurt users.
3. **Integrations across real components.** Does the use case + the real DB adapter produce the right rows? (Use a real test DB — see below.)
4. **End-to-end flows for critical paths.** Sign up, place order, refund. A handful of these, not hundreds.

## What not to test

- **Trivial getters / setters / DTOs.** No logic, no bug. Don't write `assertEquals(user.getName(), "Alice")`.
- **Framework code.** Don't test that Express routes URLs or that the ORM saves to the DB — that's the framework's job.
- **Implementation details.** "Did my function call `formatDate` exactly once?" If the behavior is right, you don't care.
- **Generated code or wrappers around well-tested libraries.** Test what *you* added on top.

## Fast tests vs slow tests

A good suite has both, in roughly the right ratio.

- **Unit tests (ms each)** — pure logic, no I/O. Run on every save. There should be hundreds to thousands.
- **Integration tests (10s–100s ms each)** — real DB, real filesystem, in-process. Run on every commit. Tens to hundreds.
- **E2E tests (seconds each)** — full stack, real network. Run on every PR. A handful.

If your "unit" tests take seconds each, they're not unit tests — they're hitting something they shouldn't. Find the I/O and stub it (or move it out — see "pure core, effectful shell" in the main skill).

## Mocking, carefully

Mocks let you test in isolation; they also let you write tests that pass while the real system is broken. Use sparingly.

- **OK to mock:** external APIs, slow services, things that cost money to call.
- **Risky to mock:** your own database. A mocked DB happily accepts queries that a real DB would reject. Prefer a real test DB (Docker, in-memory variant, transactional rollback).
- **Bad to mock:** the function under test. If you're mocking out half the logic to test the other half, the seam is in the wrong place.

If you find yourself writing 20 lines of mock setup for a 5-line test, the design is telling you the function has too many dependencies. Listen.

## Naming tests

A test name should describe the *behavior being asserted*, not the *function being called*.

```js
// Bad — tells you what's tested, not what's expected
test('calculateDiscount')

// Good — tells you the rule the test enforces
test('orders over $100 get a 10% discount')
test('orders under $100 get no discount')
test('discount applies before tax')
```

When a test fails, the test name should tell you what's broken without reading the test body.

## Arrange / act / assert

Keep tests in three visible sections. It makes them scannable.

```py
def test_orders_over_100_get_discount():
    # arrange
    order = Order(items=[Item(price_cents=15_000)])

    # act
    total = discounted_total(order)

    # assert
    assert total == 13_500
```

Don't interleave assertions and actions — one assertion section per test is the norm.

## Test data

- **Builders / factories beat fixtures.** A `make_order(total_cents=15_000)` helper lets each test express the *one thing* it cares about. Big shared fixtures couple tests to each other.
- **Realistic but minimal.** If the test cares about price, set price; don't set address. Future readers should see the relevant inputs at a glance.
- **No production data.** Even if it's "just a snapshot." Real user data in tests is a leak waiting to happen.

## Coverage

A coverage number is a *floor*, not a goal. 100% coverage of trivial code with 0% of the actual hard logic is worse than 70% coverage of the right things.

Use coverage to find **untested branches in important code**, not to chase a metric.

## When tests are painful to write

Painful tests are usually telling you something about the design, not the test.

- "I have to mock 12 things" → too many dependencies; consider extracting pure logic.
- "I can't tell what this is supposed to do" → the behavior isn't crisp; clarify the requirement before writing the test.
- "The test is longer than the code" → either the test is doing too much, or the code is hiding complexity behind a simple interface (which might be the right design — judgment call).

Listen to the test pain. The fix is often in the production code.
