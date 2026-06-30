# Complexity

You don't need to micro-optimize. You need to not write `O(n²)` when `O(n)` is the same code length.

## Big O cheat sheet — what you actually need

| Operation                       | Array       | Hash map / set | Tree map / set | Linked list |
|---------------------------------|-------------|----------------|----------------|-------------|
| Lookup by key / contains         | O(n)        | O(1) avg       | O(log n)       | O(n)        |
| Insert at end / add              | O(1) amortized | O(1) avg    | O(log n)       | O(1)        |
| Insert at start / arbitrary      | O(n)        | O(1) avg       | O(log n)       | O(1) (with handle) |
| Iterate                          | O(n)        | O(n)           | O(n) (sorted)  | O(n)        |
| Sort                             | O(n log n)  | —              | already sorted | O(n log n)  |

The two facts that catch most engineers:
- **Array `.includes` / `.find` / `.indexOf` is O(n).** Inside a loop, that's `O(n²)`.
- **Array prepend** (e.g., `arr.unshift`, `[x, ...arr]`) is **O(n)**. Building a list by prepending in a loop is `O(n²)`.

## The accidentally quadratic patterns

These are the bugs you'll write if you're not watching for them.

### Lookup inside a loop over the same data
```js
// O(orders × customers)
orders.map(o => customers.find(c => c.id === o.customerId))

// O(orders + customers)
const byId = new Map(customers.map(c => [c.id, c]))
orders.map(o => byId.get(o.customerId))
```

### Building a string by concatenation
```py
# O(n²) — each `+=` copies the whole prefix
result = ""
for line in lines:
    result += line + "\n"

# O(n)
result = "\n".join(lines)
```

### Repeated `Array.from`/`.toArray()` inside a loop
Converting an iterable to an array is O(n). Doing it in a loop turns your O(n) loop into O(n²).

### Recursion with overlapping subproblems
Classic Fibonacci is exponential. If you find yourself recursing on the same inputs, memoize.

### Filter-then-filter chains on hot paths
Each `.filter` is a full pass. Three chained filters over a 1M-row dataset is 3M operations and 3 allocations. Fold them into one pass if it matters.

## Picking the right structure

Match the structure to the access pattern, not the other way around.

- **"I look up by id often"** → `Map<Id, T>`, not `T[]`.
- **"I need uniqueness"** → `Set<T>`, not `T[].includes(x) ? ... : push(x)`.
- **"I need it sorted with frequent inserts"** → tree map / sorted set, not "re-sort the array each time."
- **"I need first-in-first-out at scale"** → a real queue (deque), not `array.shift()` which is `O(n)`.

## When to actually optimize

> "Premature optimization is the root of all evil" — Knuth. But: don't intentionally choose the slower option when the faster one is the same code length. That's not optimization, that's not writing bad code.

1. **Profile first.** Don't guess where the hot spot is — measure. Most "obviously slow" code isn't on the hot path; most actually-slow code is somewhere boring.
2. **Algorithmic wins beat micro-optimizations.** Going from `O(n²)` to `O(n)` will outperform any amount of inner-loop tuning.
3. **Memory matters too.** Iterating over a large dataset in chunks instead of loading it all is often the difference between "works" and "OOM in prod."
4. **Cache invalidation is the other half.** A cache that's wrong is worse than no cache. Only cache when you can name the invalidation rule.

## A quick gut check

For every loop you write, answer:
1. What's the size of the thing I'm iterating? (10? 10K? 10M?)
2. What's the cost of one iteration? (Constant? Another loop? A DB call?)
3. Multiply. Is the answer OK at the largest realistic size?

If you can't answer 1, you don't know the shape of your input — go find out.
