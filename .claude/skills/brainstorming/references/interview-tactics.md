# Interview tactics

The interview is one `AskUserQuestion` batch (in `/dev`) or a short series of focused questions (outside `/dev`). This reference is the field guide for making it land.

## Pick the 3–4 most consequential UNSPECIFIED slots

Walk `.claude/agents/pm.md > Required slots` and `.workflow/_templates/spec.md`. For each slot, classify:

1. **Pinned by the intent** — the user already said it. Don't re-ask.
2. **Pinned by the repo** — `package.json`, `README.md`, recent commits, or the file the intent names answers it. Don't ask.
3. **Open** — neither the intent nor the repo answers it.

Of the open slots, pick the 3–4 with the highest **consequence × ambiguity**: the slots where a wrong guess would cascade through the spec, plan, and code. Skip slots that have safe defaults the gate can still flip (e.g., `Open PR on ship: yes` for feat — defaults work, confirm at gate).

### Consequence ranking (rule of thumb)

| Slot | Consequence if wrong | Ask priority |
|------|----------------------|--------------|
| Type (when ambiguous) | Whole workflow branches differently | Always ask first if ambiguous |
| Acceptance criteria | Defines what "done" means | Always ask if open |
| Scope (`Out / non-goals`) | Hidden subsystems will surface at planning | Ask if anything could expand |
| Constraints (tech stack for new code, integration points for existing) | Wrong stack = whole plan wrong | Always ask if neither intent nor repo answers |
| Reproduction (`Type=fix`) | Regression test depends on it | Always free-text, always ask |
| Timebox (`Type=spike`) | Spike runs forever otherwise | Always ask, default 1 day if user shrugs |
| NFR detection (perf / security / a11y) | A missing-but-needed NFR passes every consistency scan and only breaks in prod | Mandatory **binary** ask for feat/fix shipping runtime code; on `yes` it becomes an AC (verify = `measured:`), on `no` nothing |
| Error/boundary per consequential behavioural AC | A silently-guessed unhappy path is the #1 "runs but does the wrong thing" failure | Mandatory **detect** ask per consequential behavioural AC (NFR-class measured ACs exempt); `none — <default>` is a valid recorded answer, silence is not |
| Users / context | Shapes AC and approach | Ask if non-obvious |
| Ship as / Open PR | Has safe defaults | Skip if running short on slots |

## Question shape — multi-choice > open-ended > free-text

`AskUserQuestion` lets you set 2–4 options per question plus an "Other" escape hatch. Prefer multi-choice — the user picks in one click, you get structured input, and the options themselves educate the user about the trade-off space.

### Multi-choice template

```
Question: <one specific question, ends with "?">
Header: <≤12 char chip>
Options:
  - Label: <1–5 words, the recommended option first with "(Recommended)" suffix>
    Description: <1 sentence: what this means + main implication>
  - Label: <alternative>
    Description: <1 sentence: trade-off>
  - (optional 3rd / 4th)
```

The "Other" option is added automatically; do not list it yourself.

### When to use free-text (open-ended)

Free-text is right when the answer is genuinely descriptive:
- `Reproduction` for `Type=fix` — never multi-choice; the user needs to type steps.
- A goal restatement that the intent left vague — "what does 'done' look like for you in one sentence?"
- A constraint the user knows that the repo can't show you — e.g., a compliance deadline.

Don't reach for free-text just because picking options feels hard. Hard-to-design options usually means the question is too broad — split it.

## Framing multi-choice options

Two failure modes:

1. **Loaded options** — Option A is described as a paragraph of upside, Option B as a one-line "but harder." That's not exploration. Match the depth and tone.
2. **False-binary** — "REST vs GraphQL?" when the actual choice has 3+ shapes (REST, GraphQL, RPC, file-based export). Add the third option even if it loses; the user can see the space.

### Worked example — Type=feat, "export user data"

✅ Good:
> Q: How should users receive the export?
> Header: Delivery
> Options:
> - Sync download (Recommended) — Browser downloads the CSV directly. Works for ≤ 100k rows / ≤ 30s; simplest path.
> - Async email link — Background job builds the file, emails a signed link. Best for large accounts; adds a queue.
> - Hybrid (sync up to threshold, async above) — Picks the right one per request. Most complex; only worth it if account sizes vary widely.

❌ Bad (loaded):
> Q: Should we use a queue?
> Options:
> - Yes (queue + email link, scalable, future-proof, recommended)
> - No (just download)

❌ Bad (false binary):
> Q: Sync or async?
> Options:
> - Sync
> - Async

## Bounded multi-round digging (when one batch is too shallow)

One `AskUserQuestion` batch is the default and is enough for narrow, concrete work. But the Mom Test is iterative — the most useful question ("tell me about the last time…") earns its value from the *follow-up* you ask after the answer lands, and you can't follow that thread inside a single batch. So a second batch is warranted when, after batch 1, **any** of these holds:

- `Type` is still genuinely ambiguous (the whole workflow branches on it).
- More than ~4 consequential slots are still open (you couldn't fit them in one batch).
- A batch-1 answer was vague, or arrived as "Other" free-text that *opened a new unknown* the options didn't cover.

Rules for the dig loop:

1. **Hard cap: 3 batches.** Past that, you're not converging — you're interviewing in circles.
2. **Each batch is narrower than the last.** Batch 2 digs into what batch 1 revealed; it does not pick fresh cold slots. If you find yourself opening *new* topics in batch 2, you mis-prioritised batch 1.
3. **An open picture after 3 batches is itself the finding.** Stop and write a `[NEEDS CLARIFICATION: <who> — <what>]` at the spot it matters. Guessing to "finish" the interview is the exact failure this loop exists to prevent.

The dig loop is the escape hatch for real ambiguity — the kind of open-ended product work this skill claims to own. It is *not* license to turn every interview into an interrogation; narrow concrete tasks still get one batch.

## Specification by Example — ground AC in concrete cases

Adapted from Specification by Example / BDD: the cheapest place to catch a mis-spec'd acceptance criterion is *before* it's written down, by forcing a real `input → expected output` pair. An AC stated only in the abstract ("user can export their data") hides the requirements that actually bite — size limits, formats, timeouts — until someone hits them in prod.

For every AC whose one-line behaviour isn't self-evident, capture one concrete example during the interview and carry it into the spec as an `e.g.:` sub-bullet:

| Abstract AC (hides requirements) | AC + example (surfaces them) |
|----------------------------------|------------------------------|
| "User can export their data" | "User exports their data" · *e.g.:* `account with 200k rows` → `CSV with columns A,B,C, downloaded in <30s` |
| "Search returns relevant results" | "Search returns matches ranked by recency" · *e.g.:* `query "invoice" with 3 matches from 2021/2023/2024` → `2024, 2023, 2021 order` |
| "Login rejects bad credentials" | "Login rejects bad credentials without leaking which field was wrong" · *e.g.:* `valid email + wrong password` → `same 401 + message as unknown email` |

The example is doing the work the pre-mortem's "mis-spec'd AC" scan does — but *up front*, where it's cheapest. When the user can't give a concrete example for an AC, that's a signal the AC isn't understood well enough to build yet → dig (above) or mark `[NEEDS CLARIFICATION]`. **Never invent the example values** — an invented example is a guessed requirement wearing a contract's clothes.

## Handling `revise` follow-ups (in `/dev`)

If the user picks `revise <notes>` at the gate (Phase 1 step 8), you don't necessarily re-run a full batch. Decide:

- **Notes affect requirements** (changed AC, added scope, changed users) → re-interview only the affected slots. A 1–2 question batch is fine.
- **Notes are spec-only** (rewording, clarifying an existing slot, fixing a contradiction) → don't re-interview. Edit `spec.md` inline — resolve any related `[NEEDS CLARIFICATION]` markers, or add a new one AT THE SPOT the ambiguity sits — and re-spawn `pm` for a spec patch.
- **Notes affect approach** (user wants Option B instead of A) → no interview; update the design, re-present, get the yes, re-spawn `pm`.

Don't treat `revise` as "start over." It's surgical.

## The `Type=fix` reproduction question

`Reproduction` is the most important free-text answer in `/dev`. The regression test depends on it.

Ask shape:
> Q: How do we make the bug appear? Walk us through the steps that trigger it, what you expected to happen, and what actually happens.

Three things the answer must contain:
1. **Steps** — concrete enough to encode in a test. "Click X, then Y, then Z" or "POST /api/foo with body {…}".
2. **Expected** — what the user thought would happen.
3. **Actual** — what does happen (with the error message / wrong value if there is one).

If the answer is missing one of these, ask one targeted follow-up. **Do not invent steps.** A spec with invented reproduction means the regression test is testing the wrong thing — and the bug will come back.

## Anti-patterns specific to the interview

- **Asking about a slot the intent already answered** — wastes a question. Re-read the intent before each question.
- **Five-option menus** — too many choices; the user defaults to the first or refuses to pick. Three with a lead is the format.
- **Combining two questions** — "What stack and what's the deploy target?" forces the user to answer both or one well. Split.
- **Free-text when multi-choice works** — "How should users authenticate?" with no options. Give them email/password vs OAuth vs SSO vs magic link and let them pick.
- **No "Recommended" tag** — without a lead the user is forced to evaluate cold. If you genuinely don't have a preference, say so in the question text and offer all options equally — but that's rare.
- **Designing the question batch without reading `FOLLOWUPS.md`** — the interview is the time to fold in carry-overs. If a follow-up could be in scope, ask "should we also handle <follow-up X> in this run?" as one of the batch slots.

## The Mom Test for spec interviews

Adapted from Rob Fitzpatrick's *The Mom Test* — three rules for getting useful information out of a customer interview, applied to the `/dev` spec interview. The book is about customer-discovery interviews for new products; the same failure modes show up when interviewing about an internal feature, a refactor scope, or a bug repro.

The book is named for the observation that even your mother — who has every incentive to be kind — can be asked questions that elicit useful information instead of polite encouragement. The trick is asking questions whose answers don't depend on the respondent liking the idea.

### The three rules

**Rule 1 — Talk about their life, not your idea.** Don't pitch the feature; ask about the problem they already have. The moment you describe the solution, you've stopped learning and started selling.

| Don't ask | Ask instead |
|-----------|-------------|
| "Would you use an export feature?" | "How do you currently get data out of the system when you need it?" |
| "Do you think a dashboard would help?" | "How do you keep tabs on the metrics that matter to you right now?" |
| "Want me to add a notification when X happens?" | "What do you do today to know when X has happened?" |

**Rule 2 — Ask about specifics in the past, not opinions about the future.** Past behaviour is observable; future opinions are imagination, and people are bad at predicting their own future behaviour. "Tell me about the last time…" is the single most useful question shape.

| Don't ask | Ask instead |
|-----------|-------------|
| "Would you pay for this?" | "Tell me about the last time you paid for something to solve this problem — what was it, how much, did it work?" |
| "How often do you think you'd export data?" | "When did you last export data? What was the format and what did you do with the file?" |
| "Would this be useful?" | "Walk me through the last time you needed this and didn't have it — what did you do instead?" |

**Rule 3 — Talk less, listen more.** Silence is a tool. When the user trails off, resist filling the gap — they almost always continue with the most honest sentence of the conversation. Don't paraphrase their answer back to them in a way that softens or sharpens it; that's a tell that you're leading.

In `/dev` this rule looks different than a live interview because the channel is `AskUserQuestion`, not voice. The equivalent of "listen more" is: don't write a 200-word framing before each question, don't pre-emptively answer the question in the option labels, and don't bundle a follow-up question into the description ("If you pick A, we'll also need to know B"). One question, lean framing, let the answer come.

### Three types of bad data (also from *The Mom Test*)

When a user answer feels great and you find yourself excited, check whether it's actually one of these three failure modes:

1. **Compliments** — "I love this idea," "this is exactly what we need," "you should definitely build this." Praise feels like validation; it isn't data. Redirect: "What was the last time you tried to solve this on your own, and how did it go?"
2. **Hypothetical fluff** — "I would probably use it," "I could see myself reaching for that," "in theory that would work for me." Future-tense, conditional, hand-wavy. Redirect: "Walk me through the last time the problem came up."
3. **Wishlists** — "you should also add Y," "while you're in there, can it do Z?" Wishlists are seductive because they sound like requirements; they're usually projections. Redirect: ignore for the current spec, log to `FOLLOWUPS.md` if they recur from multiple users with concrete examples.

Filter these *as you receive them* — every minute you spend designing AC against a hypothetical is a minute you haven't spent designing against a real workflow.

### When the user IS the user (you, your team, solo project)

The Mom Test is usually framed for talking to external customers. When the spec is for an internal feature where the user is the same person you're interviewing (and especially when "the user" is the engineer themselves), the same rules apply but the *tell* shifts:

- "Compliments" become **self-justification** — "this will make us so much faster." Redirect: which past task would have been faster, and by how much?
- "Hypothetical fluff" becomes **scope-creep enthusiasm** — "while we're refactoring, we should also..." Redirect: was this a real pain point on a real task last week, or a tidy-up wish?
- "Wishlists" become **gold-plating** — "let's make it generic so we can reuse it later." Redirect: name one concrete second use case that exists today. If you can't, defer.

Reading the rules this way prevents the spec from drifting from "what we actually need this run" to "what would feel satisfying to build."

## Pre-mortem at the gate

Adapted from Amazon's PR/FAQ practice: every internal FAQ ends with the question **"Top three reasons this product will not succeed."** It's a *pre-*mortem — you imagine the failure before it happens and write down what would have caused it, because that's the cheapest moment to design the failure out.

Applied to the `/dev` spec, the pre-mortem is the 5th self-review scan (principle 7 of `SKILL.md`). The shape:

> Imagine it's three weeks from now. This work shipped, and it was a disappointment. Name the top three reasons.

For each reason, classify the response and act:

| Reason category | What to do |
|-----------------|------------|
| **Mis-spec'd AC** — the AC was satisfied but the user wasn't | Rewrite the AC to be specific enough that satisfying it satisfies the user. |
| **Hidden dependency** — something we don't own had to deliver | Add a `Risks` bullet; surface as an `Open question` if it changes the plan. |
| **Scope mis-read** — the team will build something different from what was meant | Add a sentence to `Scope > In` or `Scope > Out` that makes the intended reading unambiguous. |
| **Approach risk** — option A was wrong for this context | Reconsider the approach options from principle 4 before locking the spec. |
| **Operational gap** — works in dev, breaks in prod | Promote to `Constraints` or add an observability AC. |

Three is the magic number — fewer and you're not stretching; more and you're padding. If you genuinely can't think of three, the design is either trivial (size = XS) or you haven't sat with it long enough; the right move on "I can't find three" is usually to sit with it five more minutes, not to skip the scan.

The pre-mortem is *not* a Risks dump — Risks list things that might go wrong. The pre-mortem asks the harder question: *which of these would I be most embarrassed about in three weeks?* That filter promotes the right items to action.

### Worked example — pre-mortem on "export user data" spec

Imagining the work shipped and disappointed:

1. **Mis-spec'd AC** — "user can export their data" passed because the endpoint returns *something*, but the CSV doesn't include the columns the user actually wanted. **Fix:** add a specific column list to the AC.
2. **Operational gap** — works on test data (1k rows), times out in prod (200k rows). **Fix:** add an AC that the export completes within 30s for the 95th-percentile account size; promote the timeout/streaming question from "implementation detail" to a `Constraint`.
3. **Scope mis-read** — the team built a one-shot download because the spec said "download," but the user actually expected a scheduled weekly email. **Fix:** add `Out (non-goals): scheduled / recurring exports — manual one-shot only this run.`

Three concrete risks, three concrete spec edits. Pre-mortem done.
