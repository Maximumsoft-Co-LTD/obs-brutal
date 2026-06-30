# Verification Checks

Verification is **executable**, not subjective. Each check is a convention-agnostic **invariant** that you
**compile against the Convention Profile** (`convention-profile.md`) discovered for THIS repo — never against
the skill's own defaults.

> Why this matters: the first version hardcoded `docs/index/`, YAML front-matter, and `[[ ]]` links. On a
> mature repo that documents itself differently, every such check went **N/A** and the skill's headline
> rigor silently evaporated. A check is only meaningful if it tests the convention the repo actually uses.

Output per check: `PASS` / `FAIL` / `N/A` with offending paths as evidence. `N/A` is reserved for a concern
the repo genuinely doesn't have (e.g. no incidents yet) — **not** for "the repo uses a different format than
the skill." A different format means *recompile the check*, not skip it.

---

## How to compile a check

1. Read `profile.<concern>` from the Convention Profile.
2. Pick the matching command form below.
3. If `profile.<concern> == none` AND this is bootstrap, use the skill-default form (YAML / `docs/index/` / etc.).

---

## CHECK-001 — Index entries resolve
**Invariant:** every doc named in the navigation layer exists on disk.
Compile against `profile.index`:
- `readme` → collect links from `docs/README.md` (+ any `*-map.md`); verify each path exists.
- `index-dir` → collect from `docs/index/*`; verify each path exists.
```bash
# readme form (hash-central): grep links out of the index doc(s), check each
grep -rhoE '\]\(([A-Za-z0-9._/-]+\.md)\)' docs/README.md docs/index/ 2>/dev/null \
  | sed -E 's/.*\(([^)]+)\)/\1/' | sort -u | while read -r p; do
    f="docs/$p"; [ -f "$p" ] || [ -f "$f" ] || echo "MISSING TARGET: $p"; done
```

## CHECK-002 — Workflow references resolve
**Invariant:** every workflow reference in rules/specs/index points to a real workflow doc.
Compile against `profile.crossref`: `wikilink` → match `[[name]]`; `path-link` → match `](…workflow…md)`.
PASS when no unresolved reference. Use `profile.naming` to map a referenced name to its filename
(e.g. `workflow-NN-<name>.md`).

## CHECK-003 — Spec references resolve
**Invariant:** every spec cites ≥1 workflow or business rule, and those targets exist. A spec with zero
backing → `REQUIRES_HUMAN_REVIEW`, not PASS.
```bash
for f in specs/**/*.md specs/*.md; do [ -f "$f" ] || continue
  grep -qiE 'workflow|business rule|\[\[|\]\(' "$f" || echo "SPEC WITHOUT EVIDENCE: $f"; done
```

## CHECK-004 — Business-rule references resolve
**Invariant:** every rule cites ≥1 evidence item (workflow/test/incident/code symbol) and named targets exist.
```bash
for f in docs/business-rules/*.md; do [ -f "$f" ] || continue
  grep -qiE 'evidence|test|workflow|incident|symbol|BR-' "$f" || echo "RULE WITHOUT EVIDENCE: $f"; done
```

## CHECK-005 — No orphan documents
**Invariant:** every managed doc is reachable from the navigation layer (`profile.index`).
```bash
comm -23 \
  <(find docs specs -name '*.md' ! -path 'docs/index/*' ! -name README.md | sort -u) \
  <(grep -rhoE '\]\(([A-Za-z0-9._/-]+\.md)\)' docs/README.md docs/index/ 2>/dev/null | sed -E 's/.*\(([^)]+)\)/\1/;s#^#docs/#' | sort -u) \
  | sed 's/^/ORPHAN DOC: /'   # tune path prefix to the repo's link style
```

## CHECK-006 — No broken references
**Invariant:** no path-link / wikilink / relative link points at a non-existent target. Compile the matcher
against `profile.crossref`.
```bash
grep -rhoE '\]\(([A-Za-z0-9._/-]+\.md)\)' docs specs CLAUDE.md 2>/dev/null \
  | sed -E 's/.*\(([^)]+)\)/\1/' | sort -u | while read -r rel; do
    [ -f "$rel" ] || [ -f "docs/$rel" ] || echo "BROKEN LINK: $rel"; done
```

## CHECK-007 — Traceability present  *(profile.freshness)*
**Invariant:** every managed doc declares the commit/version it was verified against.
| profile.freshness | compile to |
|---|---|
| `stamp` | doc contains the stamp line (e.g. `verified against <sha>`) |
| `frontmatter` | doc has YAML `source_commit:` |
| `none` (bootstrap) | introduce + require the skill's YAML front-matter |
```bash
# stamp form (hash-central): every managed doc carries a freshness stamp
for f in $(find docs -name '*.md' ! -name README.md); do
  grep -qE 'verified against' "$f" || echo "NO FRESHNESS STAMP: $f"; done
```

## CHECK-008 — Human-authored content untouched  *(profile.protected)*
**Invariant:** no managed write crossed into human-authored content.
| profile.protected | compile to |
|---|---|
| `markers` | `HUMAN_AUTHORED_START`/`END` counts balanced + block bytes unchanged vs prev commit |
| `prose` | no edit inside a paragraph carrying a do-not-touch note (e.g. "Do NOT collapse…"); verify by `git diff` review, not grep |
| `none` | N/A |
For `prose`, this check is a **diff review**, not fully automatable — diff the human-authored regions against
the previous commit and assert they are byte-identical.

## CHECK-009 — Status values valid  *(profile.status)*
**Invariant:** any staleness marker uses an allowed value. Compile to the repo's vocabulary; for the skill
default that is `CURRENT|STALE|REVIEW|DEPRECATED` + `confidence: HIGH|MEDIUM|LOW`. For a `stamp` repo with no
explicit status field, this is `N/A` (legitimately — staleness is implied by the stamp date vs HEAD).

## CHECK-010 — Freshness anchor resolves  *(profile.freshness.field)*
**Invariant:** the commit each managed doc claims to be verified against is a real commit.
```bash
# stamp form: extract the <sha> from each stamp, confirm it resolves
grep -rhoE 'verified against `?[0-9a-f]{7,40}`?' docs CLAUDE.md 2>/dev/null \
  | grep -oE '[0-9a-f]{7,40}' | sort -u | while read -r sha; do
    git cat-file -e "$sha^{commit}" 2>/dev/null || echo "BAD freshness sha: $sha"; done
```
(`frontmatter` form: same, reading `source_commit:`.)

---

## Reporting

| Check | Compiled against | Result | Evidence |
|-------|------------------|--------|----------|
| CHECK-007 | freshness=stamp | PASS | all docs carry `verified against` |
| CHECK-009 | status=none | N/A | repo implies staleness by stamp date |
| … | | | |

Completion criterion: **all checks PASS or legitimately N/A**. A check that is N/A *only because the repo
uses a different format than the skill* is a **compile failure of this checklist**, not a pass — fix the
compilation (read the profile) and re-run.
