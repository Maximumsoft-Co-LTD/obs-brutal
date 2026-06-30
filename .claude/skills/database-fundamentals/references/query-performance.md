# Query Performance

Most "the database is slow" complaints are actually "this specific query is slow," and most slow queries are slow for one of a small handful of reasons. The path is always the same: get the plan, identify the bottleneck, fix it, confirm with the plan.

## Reading EXPLAIN ANALYZE

`EXPLAIN ANALYZE` (Postgres / MySQL 8) runs the query and reports the actual plan with actual row counts and times. Plain `EXPLAIN` only shows the planner's estimates — which are often wrong, which is why you got there.

A Postgres plan reads bottom-up. The leaf nodes (the bottom) fetch data; each parent processes its children's output; the top node returns to the client.

```
Limit  (cost=0..1.50 rows=20 width=...) (actual time=0.05..0.06 rows=20 loops=1)
  ->  Index Scan using orders_user_status_placed_idx on orders
        (cost=0..1.45 rows=24 width=...) (actual time=0.04..0.06 rows=20 loops=1)
        Index Cond: (user_id = $1 AND status = 'paid')
```

Things to look at:
- **Operator type** — is it `Seq Scan`, `Index Scan`, `Index Only Scan`, `Bitmap Heap Scan`, `Nested Loop`, `Hash Join`, `Sort`?
- **`actual rows` vs `rows`** — if estimated is 10 and actual is 10,000, the planner has bad stats and is making bad choices. Run `ANALYZE <table>`.
- **`loops`** — for a nested loop, the inner side's `actual time` is per-iteration; total cost is `time × loops`.
- **`Rows Removed by Filter`** — rows the scan read and threw away. A million rows removed by filter means you scanned a million rows; the index didn't narrow them.

## The operators, ranked from cheap to expensive

- **Index Only Scan** — query is satisfied entirely from the index, no table access. The cheapest.
- **Index Scan** — index narrows the rows, then heap fetch retrieves them.
- **Bitmap Heap Scan** — multiple indexes combined, or one index that returns many rows; reads the heap in physical order. Cheaper than `Index Scan` when fetching many rows.
- **Seq Scan** — reads every row of the table. Correct for small tables or when no index matches the filter. **Problematic on large tables.**
- **Nested Loop join** — fine when the outer side is small.
- **Hash Join** — usually fine, sometimes the right answer for big joins.
- **Merge Join** — both sides pre-sorted; appears when joining on indexed keys.

If you see `Seq Scan` on a million-row table with a selective `WHERE`, you're missing an index or the planner can't use the one you have (often a type mismatch — `WHERE user_id = '5'` against a `BIGINT` column will bypass the index).

## The N+1 antipattern

The single most common performance bug in any system that uses an ORM.

**The pattern:**
```py
users = User.objects.all()                  # 1 query
for user in users:
    print(user.profile.full_name)           # N queries — one per user
```

Each `user.profile` access fires a separate query. Hidden behind clean-looking attribute access.

**The fix:**
```py
# Django
users = User.objects.select_related('profile').all()
# 1 query, joined

# SQLAlchemy
users = session.query(User).options(joinedload(User.profile)).all()

# Prisma
const users = await prisma.user.findMany({ include: { profile: true } })

# Rails
users = User.includes(:profile).all
```

**The detection:**
- Most ORMs have a query logger or middleware that counts queries per request. Wire it up and watch for endpoints that fire dozens or hundreds.
- In Postgres, `pg_stat_statements` ranks queries by total time and call count. An N+1 always shows up as a tiny query that fired hundreds of thousands of times.

**The harder variant — N+1 across batches:**
```ts
async function loadOrdersWithCustomers(orderIds: string[]) {
  const orders = await db.orders.findMany({ where: { id: { in: orderIds } } })  // 1 query
  for (const o of orders) {
    o.customer = await db.customers.findUnique({ where: { id: o.customerId } }) // N queries
  }
}
```

Fix with a single batched query and an in-memory join:
```ts
const orders = await db.orders.findMany({ where: { id: { in: orderIds } } })
const customerIds = [...new Set(orders.map(o => o.customerId))]
const customers = await db.customers.findMany({ where: { id: { in: customerIds } } })
const byId = new Map(customers.map(c => [c.id, c]))
for (const o of orders) o.customer = byId.get(o.customerId)
```

For graph fetches across many call sites, use a **dataloader**: collect IDs during the request, fire one batch query per type at the end of the tick. Facebook's pattern, packaged as `dataloader` in Node and `aiodataloader` in Python.

## Pagination: keyset over offset

`OFFSET 10000 LIMIT 20` looks reasonable but scales `O(offset)` — the database has to fetch and discard the first 10,000 rows to give you rows 10001–10020. On page 500, this is a problem.

**Keyset pagination** uses the last seen value of the sort key as the cursor:

```sql
-- First page
SELECT * FROM orders WHERE user_id = $1 ORDER BY placed_at DESC, id DESC LIMIT 20;

-- Next page — cursor is the last row's (placed_at, id) from the previous page
SELECT * FROM orders
WHERE user_id = $1 AND (placed_at, id) < ($last_placed_at, $last_id)
ORDER BY placed_at DESC, id DESC
LIMIT 20;
```

Index on `(user_id, placed_at DESC, id DESC)` and every page costs the same: `O(log n)` lookup + 20 row reads. No matter what page you're on.

Tradeoff: no "jump to page 50" — only "next" and "previous." Almost always the right call for APIs and infinite-scroll UIs. Use offset only for admin grids where the page count is small and bounded.

## SELECT * and overfetching

`SELECT *` is fine in development. In production:

- It returns columns the caller doesn't need, wasting network and serialization time.
- It prevents `Index Only Scan` — the index doesn't cover the columns you didn't ask for.
- It silently breaks when a column is added or removed.
- It serializes large `TEXT` or `JSONB` payloads even when nobody reads them.

Project the columns you actually use. Keep the wide blob columns (`description`, `payload`, `metadata`) out of list endpoints; load them in the detail view.

## COUNT(*) on large tables

`SELECT COUNT(*) FROM orders` is `O(rows)` — there's no shortcut in MVCC databases. On a billion-row table, it takes seconds to minutes.

Options:
- **Don't count.** Most UIs that show "1,234,567 results" only need "lots" or "approximately X." `LIMIT 1000` and report "1000+".
- **Estimate from stats.** Postgres `SELECT reltuples FROM pg_class WHERE relname = 'orders'` is approximate but free.
- **Maintain a counter.** A separate `counters` table updated by trigger or by the application. Exact, fast read, slight write cost.
- **Use a filtered count with an index.** `COUNT(*) WHERE status = 'pending'` over an index on `status` is fast when 'pending' is a small slice.

## The slow-query diagnostic flow

When a query is slow:

1. **Get the plan.** `EXPLAIN ANALYZE` the exact query, with realistic parameters (not the one-row example that hits the index perfectly).
2. **Find the cost.** The biggest `actual time` cumulative is your bottleneck.
3. **Classify it.**
   - `Seq Scan` on big table + selective `WHERE` → missing or wrong index.
   - `Sort` after an `Index Scan` → index doesn't include the `ORDER BY` column.
   - `Nested Loop` with high `loops` × inner-side time → likely the N+1-as-a-join.
   - Big `Rows Removed by Filter` → index is the wrong shape; filter columns should be in it.
   - `actual rows` >> `rows` estimate → bad stats, run `ANALYZE`.
4. **Hypothesize.** Propose one change (add index, reshape query, switch to keyset).
5. **Apply and re-explain.** Confirm the planner uses your new index. Confirm `actual time` dropped.
6. **Watch in production.** Some queries that look fast on dev data crawl on real data with skew. Check `pg_stat_statements` after deploy.

## Pitfalls

- **Implicit type casts in WHERE.** `WHERE user_id = '123'` against a `BIGINT` column may bypass the index in some engines. Match types exactly.
- **`OR` across columns.** `WHERE a = $1 OR b = $2` typically can't use a single composite index. Either two separate indexes + bitmap-or, or a `UNION` of two queries.
- **Functions on indexed columns.** `WHERE lower(email) = $1` skips a plain `(email)` index. Use an expression index (`CREATE INDEX ON users (lower(email))`) or store the normalized form.
- **`NOT IN` with NULLs.** Returns no rows if any NULL is in the list. Use `NOT EXISTS` instead.
- **`LIKE '%foo'` (leading wildcard).** Can't use a B-tree index. Use full-text search or a reverse index.
- **`LIMIT` without `ORDER BY`.** The result is non-deterministic; the planner may choose a different plan run-to-run.
