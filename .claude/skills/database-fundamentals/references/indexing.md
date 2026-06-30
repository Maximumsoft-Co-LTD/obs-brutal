# Indexing

An index is a separate data structure that lets the database find rows by some column(s) in `O(log n)` instead of scanning the table. The catch: every index pays write cost on every `INSERT`/`UPDATE`/`DELETE`, and takes disk and memory. The goal isn't "index everything," it's "index the queries that need it, nothing more."

## The two rules that catch most engineers

### 1. The leftmost prefix rule

A composite index on `(a, b, c)` can serve queries that filter on:
- `a`
- `a AND b`
- `a AND b AND c`

It **cannot** serve queries that filter on:
- `b` alone
- `c` alone
- `b AND c`

The columns are concatenated in order; the index is a tree sorted by `a`, then within each `a` value by `b`, then by `c`. You can use a tree if you know where to start at the top — which means you need the leftmost columns.

**Implication for ordering:** put the most-selective and most-frequently-filtered column first. A composite `(tenant_id, status)` for a multi-tenant app makes sense; `(status, tenant_id)` rarely does.

### 2. Range filters consume the rest

Once a composite index hits a range filter (`<`, `>`, `BETWEEN`, `LIKE 'foo%'`), it can't use columns to the *right* of that filter for further narrowing — only for sorting or as covering data.

```sql
-- Index: (user_id, placed_at, status)
SELECT * FROM orders
WHERE user_id = $1 AND placed_at > $2 AND status = 'paid';
-- Index narrows by user_id, then by placed_at range.
-- status filter cannot use the index — it's checked post-scan.

-- Better index for this query:
CREATE INDEX ON orders (user_id, status, placed_at);
-- Now equality on (user_id, status) narrows tightly,
-- then placed_at range scan within that.
```

Equality first, range last.

## Covering / index-only scans

If the index contains every column the query needs (both filter and `SELECT` list), the database can answer entirely from the index without touching the table. That's the cheapest possible read.

```sql
-- Query
SELECT id, total_cents FROM orders WHERE user_id = $1 AND status = 'paid';

-- Plain index — finds the rows but must visit the heap for total_cents
CREATE INDEX ON orders (user_id, status);

-- Covering index — adds total_cents as INCLUDE column (Postgres syntax)
CREATE INDEX ON orders (user_id, status) INCLUDE (total_cents, id);
-- Postgres can do an Index Only Scan: never touches the table.
```

Don't over-include — every column in the index inflates its size, which inflates cache pressure and write cost. Cover for queries that are both hot and read-heavy.

## Partial indexes

Index only the rows you actually query. Massive wins when one value dominates the table.

```sql
-- 99% of users are 'active'; you only ever query the other 1%
CREATE INDEX ON users (created_at) WHERE status = 'suspended';
-- Index is tiny, query is fast, the 99% pay nothing on write to this index.

-- Soft-delete pattern: index only live rows
CREATE INDEX ON orders (user_id, placed_at) WHERE deleted_at IS NULL;
```

Partial indexes are also how you make `UNIQUE` work conditionally: `CREATE UNIQUE INDEX ON users (email) WHERE deleted_at IS NULL` lets soft-deleted rows free up the email.

## Index types beyond B-tree

Default index is B-tree — good for equality, ranges, and ordering. Other index types exist for non-default access patterns:

- **Hash** — equality only, slightly faster than B-tree for that one use case. Postgres B-tree is usually fine; rarely worth choosing hash.
- **GIN** — for inverted lookups: full-text search, JSONB containment (`@>`), array membership. The right answer when you want "is X in this array/document?"
- **GiST / SP-GiST** — for geometric, range, and nearest-neighbor queries.
- **BRIN** — block range index. Tiny, useful when data is physically clustered by the indexed column (e.g., append-only logs ordered by `created_at`). Cheap to maintain, less precise than B-tree.

If you don't know which to pick, the answer is B-tree.

## Foreign keys need indexes

Postgres does **not** create an index on foreign-key columns automatically. (MySQL InnoDB does.) Without one, every delete or update on the parent table has to scan the child table to find dependents — turning O(log n) deletes into O(n) deletes, and holding row locks while it scans.

```sql
CREATE TABLE order_items (
  order_id  UUID REFERENCES orders(id),
  ...
);
-- Without this, DELETE FROM orders WHERE id = $1 scans order_items.
CREATE INDEX ON order_items (order_id);
```

## When NOT to add an index

- **Small tables** (under ~10k rows). The seq scan is faster than B-tree lookup once you account for the planner overhead, and the optimizer will ignore the index anyway.
- **Low-selectivity columns**, alone. An index on a boolean `is_active` (95% true) is mostly useless — the planner will seq scan because following the index costs more than the read. *Composite* or *partial* indexes on low-selectivity columns can still help.
- **Heavy-write, rarely-read columns.** Every index is a tax on writes. If a column is never in a `WHERE`, don't index it.
- **You haven't measured.** Don't speculatively index. Run the query, look at the plan, add the index only if the plan shows seq scan over a meaningful row count.

## How to find unused indexes (Postgres)

```sql
-- Indexes that have never been read since stats reset
SELECT
  schemaname || '.' || relname AS table,
  indexrelname AS index,
  pg_size_pretty(pg_relation_size(i.indexrelid)) AS size,
  idx_scan AS scans
FROM pg_stat_user_indexes i
JOIN pg_index USING (indexrelid)
WHERE idx_scan = 0 AND NOT indisunique
ORDER BY pg_relation_size(i.indexrelid) DESC;
```

If a non-unique index has zero scans since the last stats reset (default: server start), it's a candidate for dropping. Don't drop blindly — confirm by checking that no infrequent batch job uses it.

## Diagnostic flow when adding an index

1. Find the slow query. (Postgres `pg_stat_statements`, MySQL slow query log.)
2. Run `EXPLAIN ANALYZE` on it.
3. Identify the seq scan or the sort that's costing the most.
4. Propose an index: equality columns first, range/order columns next, included columns if covering helps.
5. Add it (`CREATE INDEX CONCURRENTLY` in Postgres on big tables).
6. Re-run `EXPLAIN ANALYZE`. Confirm the planner uses it and the actual time dropped.
7. Don't keep indexes that didn't help — they're write tax forever.
