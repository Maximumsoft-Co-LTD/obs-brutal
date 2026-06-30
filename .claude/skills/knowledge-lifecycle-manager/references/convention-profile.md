# Convention Profile

The bridge between **"discover, don't impose"** (principle #3/#5) and **"verify everything"** (principle #7).
Without it those two pillars contradict: hardcoded checks only know the skill's *own* format, so on any repo
that already documents itself differently, verification silently degrades to N/A — i.e. on **every mature repo**.

Discovery produces **one** profile describing the conventions the repo ALREADY uses. Every later phase —
generation, verification, coverage — reads the profile instead of assuming the skill's defaults.

## Produce it during Discovery

Inspect existing docs (`docs/`, `CLAUDE.md`, `.workflow/`, READMEs) and record this in the run checkpoint:

```yaml
profile:
  freshness:  stamp | frontmatter | none     # how a doc declares the commit it was verified against
    field:    "<the literal form, e.g. '🕒 verified against <sha> · <date>'  OR  YAML source_commit>"
    applies_to: [<doc layers the freshness convention covers>]   # conventions can be PARTIAL — see below
  status:     stamp | frontmatter | none      # how staleness/REVIEW is marked
  anchors:    symbol | line | mixed           # how claims cite code
  index:      readme | index-dir | none       # where the navigation layer lives (docs/README.md vs docs/index/)
  incident:   workflow-run | inc-file | none  # .workflow/<run-id>/ vs incidents/INC-xxxx.md
  crossref:   path-link | wikilink | none     # ](path.md) vs [[name]]
  protected:  markers | prose | none          # HUMAN_AUTHORED markers vs human prose notes
  naming:     "<observed doc-naming pattern, e.g. workflow-NN-<name>.md>"
```

Rules:
- If a convention EXISTS, record it and **extend** it. The skill's defaults (YAML front-matter, `docs/index/`,
  `[[ ]]`, `INC-xxxx`) are used **only where the profile says `none`** — genuine greenfield.
- **Never two conventions for one concern.** If the repo stamps freshness, you stamp freshness — you do not
  add YAML alongside it. (This is exactly what made CHECK-007/009/010 go N/A in the hash-central run.)
- **A convention can be PARTIAL / per-layer.** Don't assume it covers the whole repo. hash-central stamps
  *source-derived* docs (foundation, workflows, business-rules, testing) but NOT *policy* docs (`docs/ai/`)
  or *third-party* docs (`docs/slip2go/`, which version against the external API, not our commit). Record the
  scope in `applies_to`; a compiled check (e.g. CHECK-007) then evaluates **only** docs inside that scope, so
  it stops false-flagging docs the convention was never meant to cover. A doc outside `applies_to` is `N/A`
  for that check **by design** — not a defect.

## The profile is the bootstrap/maintenance switch

- profile mostly **populated** → **MAINTENANCE** (a KB exists): audit → update → fill gaps. *Common case.*
- profile mostly **`none`** → **BOOTSTRAP** (no KB): create using skill defaults.

Mode also selects the evidence-model primary source (`evidence-model.md`) and the generation path.

## How the profile feeds verification

Verification checks are written as convention-agnostic **invariants**, then *compiled* against the profile.
Example — CHECK-007 (traceability):

| profile.freshness | CHECK-007 compiles to |
|-------------------|-----------------------|
| `frontmatter` | every managed doc has YAML `source_commit` |
| `stamp` | every managed doc has a `verified against <sha>` stamp line |
| `none` (bootstrap) | introduce + require the skill's YAML front-matter |

See `verification-checks.md` for all ten compiled checks.

## Worked example (hash-central, real run)

```yaml
profile:
  freshness:  stamp   field: "🕒 verified against <sha> · <date>"
    applies_to: [foundation, workflows, business-rules, testing, specs]   # NOT docs/ai (policy), NOT docs/slip2go (third-party)
  status:     stamp
  anchors:    symbol
  index:      readme          # docs/README.md, foundation-scoped (no cross-cutting index → that's a real gap)
  incident:   workflow-run    # .workflow/<run-id>/ + INDEX.md + FOLLOWUPS.md
  crossref:   path-link
  protected:  prose           # "Do NOT collapse… hard-won operational knowledge"
  naming:     "workflow-NN-<name>.md"
```

→ CHECK-007/009/010 compile against the **stamp**, not YAML — so they actually run instead of going N/A.
→ Generation extends `workflow-NN-*.md`; it does not impose `<name>.md`.
→ Incidents go under `.workflow/<run-id>/`; the skill does not create `incidents/INC-xxxx.md`.
