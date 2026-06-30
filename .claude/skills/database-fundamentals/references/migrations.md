# Migrations

A migration runs against a live production database, while users are hitting the system, while the old version of the application is still pointing at the same tables. Every migration must be safe under those conditions. Most production outages caused by a database change are caused by ignoring this.

The mental model: a migration is a **public API change**. The database schema is the contract between the database and every application instance reading and writing it. Breaking the contract — even briefly — breaks all the running instances.

## The cardinal rule: never break the running app

At deploy time, you have a mix of old and new application instances. Both must work against the schema as it exists *now*. That means:

- Adding a column is safe (old app ignores it).
- Adding a table is safe.
- Adding an index is safe (data is unchanged), but **locking-aware**: a naive `CREATE INDEX` locks the table on Postgres until done.
- Renaming a column is **never** a single step. Old app reads the old name, errors immediately on rename.
- Dropping a column is **never** a single step. Old app might still write to it.
- Changing a column type is risky. Old app may write the old type.
- Adding `NOT NULL` to an existing column requires every row to have a value first.
- Adding a `CHECK` constraint requires every existing row to satisfy it.

## The expand → backfill → contract pattern

For any change that isn't strictly additive, split it across at least three deploys:

1. **Expand** — make the schema compatible with both old and new code. Add the new shape alongside the old.
2. **Backfill** — populate the new shape for existing rows. Make the app write to both shapes.
3. **Switch** — deploy code that reads from the new shape.
4. **Contract** — once no code references the old shape, drop it.

Each step is independently deployable and rollback-safe.

### Example: renaming `full_name` to `display_name`

```sql
-- Migration 1 (expand): both columns exist
ALTER TABLE users ADD COLUMN display_name TEXT;
```
```python
# Code deploy 1: write to BOTH columns, read from full_name
user.full_name = name
user.display_name = name
```
```sql
-- Migration 2 (backfill): populate display_name for old rows
UPDATE users SET display_name = full_name WHERE display_name IS NULL;
```
```python
# Code deploy 2: read from display_name (and still write both)
return user.display_name
```
```python
# Code deploy 3: stop writing full_name
user.display_name = name
```
```sql
-- Migration 3 (contract): drop the old column
ALTER TABLE users DROP COLUMN full_name;
```

Four deploys for a column rename. That sounds like a lot until you watch a naïve `RENAME` take production down for the 60 seconds it takes to roll out new app instances.

### Example: adding a `NOT NULL` column

```sql
-- Step 1 (expand): nullable with default
ALTER TABLE orders ADD COLUMN currency CHAR(3);
```
On Postgres 11+, adding a column with a default that's *constant* doesn't rewrite the table. With a default that's a function (`now()`), it does — slow on big tables.

```sql
-- Step 2 (backfill): set values for existing rows
UPDATE orders SET currency = 'USD' WHERE currency IS NULL;
-- For large tables, do this in batches to avoid bloating one transaction:
-- UPDATE ... WHERE id IN (SELECT id FROM orders WHERE currency IS NULL LIMIT 10000);
```

```sql
-- Step 3 (constrain): enforce NOT NULL after every row has a value
-- On large tables, prefer the two-step approach to avoid a long table lock:
ALTER TABLE orders ADD CONSTRAINT orders_currency_nn CHECK (currency IS NOT NULL) NOT VALID;
ALTER TABLE orders VALIDATE CONSTRAINT orders_currency_nn;
-- Then optionally swap to a true NOT NULL during a maintenance window:
ALTER TABLE orders ALTER COLUMN currency SET NOT NULL;
ALTER TABLE orders DROP CONSTRAINT orders_currency_nn;
```

The `NOT VALID` trick lets you add the constraint without scanning every row (it applies only to new rows); the `VALIDATE` step then checks existing rows under a weaker lock than a full `SET NOT NULL`.

### Example: changing a column type

The safe path: add a new column of the new type, backfill, switch reads, drop old. Same expand-contract sequence. Trying to `ALTER COLUMN TYPE` in place on a large table either takes a long exclusive lock or fails (depending on the engine and type compatibility).

## Locking-aware DDL

Different DDL statements take different locks. Knowing which lock matters because some locks block reads, some block writes, and some block both — and the duration depends on table size.

**Postgres** (most common gotchas):
- `CREATE INDEX` — blocks writes for the duration. **On any non-trivial table, use `CREATE INDEX CONCURRENTLY`.** Slower (two passes) but doesn't block writes.
- `ALTER TABLE ADD COLUMN ... DEFAULT <constant>` — instantaneous since PG 11 (metadata-only).
- `ALTER TABLE ADD COLUMN ... DEFAULT <expression>` — rewrites the table. Slow on big tables, holds `ACCESS EXCLUSIVE`.
- `ALTER TABLE ALTER COLUMN SET NOT NULL` — scans the table to verify, holds `ACCESS EXCLUSIVE`. Use the `CHECK NOT VALID` / `VALIDATE` trick above for big tables.
- `ALTER TABLE ADD FOREIGN KEY` — also scans to verify; same trick (`NOT VALID` then `VALIDATE`).
- `DROP INDEX` — quick, takes `ACCESS EXCLUSIVE` briefly. Use `DROP INDEX CONCURRENTLY` for safety.

**MySQL / InnoDB:**
- Most `ALTER TABLE` operations on a large table are problematic. Three online-schema-change tools are in active use: **`pt-online-schema-change`** (Percona, triggers-based — long-standing default), **`gh-ost`** (GitHub, binlog-based — lighter on the primary, originally built for GitHub's MySQL fleet), and **Spirit** (the modern successor by Morgan Tocker — binlog-based with better resumability and rewritten internals; intended to replace gh-ost for new work). All three build a shadow copy, sync writes from the source, and atomically swap. Pick the tool your team operates already; default to Spirit for greenfield.
- `ALGORITHM=INPLACE, LOCK=NONE` works for some changes (adding nullable columns, dropping indexes) but not all.

## Long-running backfills

A 50-million-row `UPDATE` in one transaction is a bad idea: it bloats the WAL, blocks `VACUUM`, holds locks, and if it fails you redo all of it. Batch instead.

```sql
-- Pseudo-pattern (Postgres). Loop in a script.
WITH batch AS (
  SELECT id FROM orders
  WHERE currency IS NULL
  LIMIT 10000
  FOR UPDATE SKIP LOCKED
)
UPDATE orders SET currency = 'USD' WHERE id IN (SELECT id FROM batch);
-- Sleep briefly between batches to let replicas catch up and let other writers in.
```

Why batches:
- Each batch commits quickly, releasing locks.
- Replica lag stays bounded.
- Failure costs you one batch, not the whole job.
- You can monitor progress (`COUNT(*) WHERE currency IS NULL`).

## Migration files

Most stacks have a migration framework (Flyway, Liquibase, Alembic, golang-migrate, Knex, Prisma Migrate, Rails ActiveRecord, Django). Use one — don't hand-run SQL files in production.

Conventions:
- Migrations are **numbered or timestamped**, applied in order, applied once.
- **Forward-only in production.** Don't rely on `down` migrations to fix bad deploys — they're for local development. Production fixes are new forward migrations.
- **One migration = one logical change.** Don't bundle five unrelated alters; if one fails, you have to undo by hand.
- **Migrations are code reviewed.** A schema change is at least as risky as a code change.
- **Schema and code go together but deploy separately.** The migration deploys first; the code that depends on it deploys second.

## Dual-write transitions (the "shadow write")

When the new shape is in a different system entirely (new table, new database, new service), the pattern is:

1. Write to both old and new. Read from old.
2. Backfill the new system with historical data.
3. Add a verification job: for each old write, confirm the new value matches.
4. Switch reads to new.
5. Stop writing to old.
6. Decommission old after a grace period.

Same idea — expand, backfill, switch, contract — at a larger scale.

## Pitfalls

- **Combining schema change and dependent code change in one deploy.** The instant the migration runs, the old code (still running on some instances) is broken. Always: schema first, then code.
- **Adding `NOT NULL` without a default to an existing big table.** Fails immediately if any row is null. Fails painfully and slowly even if it would succeed.
- **`CREATE INDEX` without `CONCURRENTLY` on a busy table.** Blocks writes until done. Can be minutes on a big table. Always `CONCURRENTLY` in production Postgres.
- **Dropping a column the app still references.** Old code on still-rolling-out instances will start erroring as soon as the migration runs. Stop reading and writing it in the code first.
- **Long transactions for big `UPDATE`s.** Bloat, lock contention, vacuum starvation, replication lag. Batch instead.
- **Relying on `down` migrations to recover production.** They almost never work as expected — by the time you'd need to roll back, new data has been written in the new shape. Roll forward.
- **Migrations that depend on app data shape.** A migration that reads JSON column contents to backfill is brittle — the JSON shape might have changed since the data was written. Prefer migrations that operate on plain SQL types.

## Checklist before merging a migration

1. Is this change strictly additive? (Add column, add table, add nullable column with default → safe.) If yes, you can deploy with confidence.
2. If not additive, am I doing expand → backfill → contract across multiple deploys?
3. For any DDL on a large table: do I know what lock it takes, and for how long? Should I use `CONCURRENTLY` or a `NOT VALID`/`VALIDATE` split?
4. For any data backfill: is it batched? Will it bloat replication or `VACUUM`?
5. Can the currently-deployed application code still work against the post-migration schema? (If not, the migration is breaking.)
6. Has someone other than me read this migration?
