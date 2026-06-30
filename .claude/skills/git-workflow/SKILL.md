---
name: git-workflow
description: Apply the high-leverage fundamentals that govern any non-trivial git work — branching from a fresh base, committing small atomic units, writing commit messages future-you can read, rewriting local history but never shared history, integrating often to avoid rot, treating PRs as the unit of review (not the diff), and recovering with the reflog before reaching for anything destructive. Use this skill BEFORE you create a branch, stage a commit, write a commit message, rebase, merge, force-push, open or update a PR, or reach for `git reset --hard` / `git push --force` / `git checkout --` to "clean things up." Triggers on requests like "commit this", "open a PR", "rebase onto main", "squash these commits", "I messed up the history", "this branch is behind", "amend the last commit", "force push", "I lost my changes", "split this into commits", "review-ready", "clean up this branch", "merge vs rebase", "what's a good commit message for X", "fix the merge conflict", or any moment you're about to write to `.git`. Use it even when the user doesn't say "git" — the trigger is any branch/commit/PR-shaped move. The skill provides a 7-principle pre-flight checklist with worked examples, plus reference files on commit messages, branching and rebasing, pull requests, and recovery. Skip only for one-off `git status` / `git log` / `git diff` reads where nothing is being written.
---

# Git Workflow

## Why this exists

Git is the joint between your work and everyone else's. Almost every "lost my changes," "force-pushed over a teammate," "the branch went sideways," or "we can't tell what this PR does" story traces back to the same handful of skipped fundamentals — a branch that grew five purposes, a commit message that said "fixes" and nothing else, a rebase of a published branch, a `git reset --hard` reached for before `git reflog`. The mechanics differ across teams (GitHub vs GitLab, rebase-only vs merge-commits, trunk-based vs gitflow); the contract you have to think about does not.

The principles below are workflow-, platform-, and tool-agnostic. They apply equally to a solo project pushing to `main` and a 200-engineer monorepo with a queued merge bot. Catch them before the write and the cost is seconds; catch them in a postmortem and the cost is a teammate's afternoon, a lost commit, or a PR nobody can actually review.

## The 7 principles

Each principle has a one-line rule, a *why*, and a worked example. Apply them roughly in order — the early ones (where you started, what you committed) constrain the later ones (how you rebase, how you ship).

---

### 1. Branch from a fresh base; one branch, one purpose

**Rule:** Start every branch from an up-to-date `main` (or the team's integration branch). Give the branch one job — one feature, one fix, one refactor — that you can describe in a single sentence.

**Why:** A stale base means you're already merging from day one — every conflict you hit later is the cost of skipping `git fetch` before you started. A branch with five purposes is five PRs trying to share a body: nobody can review it, you can't revert one bit without losing the others, and the eventual `git log --oneline` reads like a diary instead of a changelog. The convention `<type>/<short-slug>` (e.g., `feat/audit-log`, `fix/login-redirect`) is cheap and pays off the first time someone scans the branch list.

**How to apply:**
- Before `git checkout -b`, run `git fetch origin && git switch main && git pull --ff-only`. Branch *from there*. If `--ff-only` refuses, your local `main` has drifted — fix that first, don't paper over it.
- Name the branch after the change, not the author. `feat/audit-log`, `fix/login-redirect`, `refactor/auth-middleware` beat `flame/wip`.
- If, mid-branch, you discover an unrelated cleanup you want to make — stash it, branch separately, ship it on its own. Resist "while I'm in here." Every "while I'm in here" doubles the review cost.
- See `references/branching-and-rebasing.md` for branch naming, the case for trunk-based vs feature branches, and long-lived-branch hygiene.

**Example:**
```
Bad:  git switch -c my-branch         # from a week-old main
      → 30 commits later: feat work, a typo fix in an unrelated file, a config rename, 200 lines of formatter churn.
      → reviewer can't tell which diff is "the change." Bisect is meaningless. Revert is impossible.

Good: git fetch origin && git switch main && git pull --ff-only && git switch -c feat/audit-log
      → branch does one thing. The PR description fits in two lines. Revert is a single commit.
```

---

### 2. Commit small, atomic, and complete

**Rule:** Each commit should be one logical change that compiles, passes tests, and stands alone. If the message needs an "and," the commit is two commits.

**Why:** Atomic commits are what make `git bisect`, `git revert`, `git blame`, and code review actually work. A commit that mixes "fix the bug" with "rename three variables and reformat the file" is a debugging trap — the next engineer trying to bisect a regression lands on this commit, sees 400 lines of diff, and has no idea which line caused the test to fail. Granularity also forces honesty: if you can't describe the commit in one line without hedging, you don't actually know what you just did.

**How to apply:**
- Stage in pieces. `git add -p` (patch mode) lets you split a working tree into multiple commits even if you wrote them as one mess. Use it liberally.
- One commit = one reason to change. Refactors, behavior changes, formatting, and dependency bumps are *different* reasons. Don't braid them.
- Each commit should leave the tree in a state where tests pass. This is the contract `git bisect` depends on. Broken intermediate states make bisect useless and revert dangerous.
- The "fixup" workflow is your friend mid-branch: when you find a flaw in an earlier commit, `git commit --fixup=<sha>` and `git rebase -i --autosquash` *before* pushing. Don't pile "fix typo," "fix typo again," "really fix typo" into the history that ships.
- See `references/branching-and-rebasing.md` for `--fixup` / `--autosquash` and the patch-mode workflow.

**Example:**
```
Bad: 1 commit
     "fix audit log + cleanup + bump axios + reformat utils.ts"
     → 1400-line diff across 23 files. Two real bugs, one feature, one dep bump, one formatter pass, all in one node.

Good: 5 commits
     refactor(utils): pull formatter run through (no behavior change)
     chore(deps): bump axios 1.6.0 → 1.7.2
     feat(audit): record user_id on every audit row
     fix(audit): default actor to "system" when request has no user
     test(audit): cover system-actor branch
     → each commit reviewable on its own, revertible on its own, bisectable on its own.
```

---

### 3. Write commit messages future-you can read

**Rule:** The subject says *what* in ≤72 characters and the imperative mood. The body — when there is one — says *why*. Pick a convention (e.g., conventional commits) and hold the line.

**Why:** Commit messages are the only artifact that survives forever, side-by-side with the code they explain. `git blame` lands on a commit, not a PR; on a `git log` line, not a Slack thread. A message that says "updates" or "fixes" is a hostile act against every future engineer (including you) who has to figure out why a line is there. The *what* is in the diff; the message's job is the *why* — the constraint, the choice, the trade-off, the bug number, the alternative you rejected. A consistent format (conventional commits or similar) means tooling can parse the log into a changelog, a release type, or a triage signal for free.

**How to apply:**
- Subject line: imperative ("Add audit log column," not "Added" / "Adding"), ≤72 chars, no trailing period. If you can't fit it, the commit is probably two commits.
- Body: separated from subject by a blank line, wrapped at ~72 chars, focused on *why* and *what changed in shape*. Skip the body only when the subject is genuinely self-explanatory (`docs: fix typo in README`).
- Reference issues, RFCs, or the failing test by ID, not by paraphrase. `Closes #482` is useful; "fixes the bug we talked about Tuesday" rots in a week.
- Pick a convention and stick to it. This project uses **Conventional Commits** style — `<type>(<scope>): <subject>`. The CC v1.0.0 spec only *mandates* `feat` and `fix` (plus the `!` / `BREAKING CHANGE:` marker); everything else is a project choice. This project's type list is `feat | fix | refactor | chore | docs | spike | test | perf | build | ci` — a *superset* of the six `/dev` run/spec Types (`feat | fix | refactor | chore | docs | spike`) plus `test | perf | build | ci` for commits whose change isn't itself a `/dev` run Type. So every spec `Type:` maps to a commit `<type>`, but the commit log can also carry categories that never appear as a run Type. `spike` in particular is a project-local extension, not part of the CC spec. Breaking changes get a `!` after the type or a `BREAKING CHANGE:` footer.
- See `references/commit-messages.md` for the full conventional-commits cheat sheet, scope conventions, and breaking-change syntax.

**Example:**
```
Bad subject:
    "stuff"
    "WIP"
    "fix"
    "Updated some files based on PR comments and also fixed a thing"

Good subject + body:
    fix(auth): persist session before redirect to fix login loop

    When the OAuth callback set the session cookie and immediately
    redirected, Chrome (but not Firefox) dropped the cookie on the
    cross-origin redirect. Flush the session write before redirecting.

    Regression test: auth.spec.ts:redirect-loop.
    Closes #482.
```

---

### 4. Rewrite local history, never shared history

**Rule:** Before you push, your commits are yours — rebase, squash, reorder, reword freely. After you push to a branch others may have fetched, they're shared — additive commits only, or coordinate explicitly.

**Why:** A rebase rewrites SHAs. If a teammate has the old SHAs in their local clone and you force-push the new ones, their next `git pull` is a horror show — merge conflicts against ghost commits, lost work, "why is my branch 20 commits behind and 20 commits ahead." The golden rule isn't "never rebase"; it's "never rebase what others depend on." Local cleanup is *good* — it's how the commits you finally push tell a coherent story instead of stream-of-consciousness "fix typo / fix typo / really fix typo." The discipline is knowing the line.

**How to apply:**
- Locally, before any push: clean up freely with `git rebase -i`, `git commit --amend`, `git commit --fixup`, `git rebase --autosquash`. This is where messy work becomes a reviewable history.
- The moment you've pushed and someone else might be working off the branch (a teammate, a CI cache, a review tool), treat history as frozen. Add commits; don't rewrite past ones.
- If you *must* rewrite a pushed branch (e.g., to squash before merge), use `git push --force-with-lease`, not `--force`. `--force-with-lease` refuses to overwrite if someone else pushed in the meantime; `--force` happily clobbers their work.
- **Never** force-push to `main` (or any protected/long-lived integration branch). If a bad commit landed, *add a revert commit*. The history is the truth, and "the truth got worse for ten minutes" is preferable to "the truth got rewritten."
- Don't `--no-verify` to skip hooks, don't `-c commit.gpgsign=false` to bypass signing, unless the user has explicitly asked for it. If a hook fails, the hook is telling you something — read it, fix the cause.
- See `references/branching-and-rebasing.md` for the full rebase / squash / fixup mechanics and the force-with-lease pattern.

**Example:**
```
Local-only cleanup (good): rebased my 12-commit branch into 4 logical commits before pushing the first time. Force-push? Not needed —
                           branch has never been pushed.

Pushed branch, then squashed locally and force-pushed (acceptable, with care):
    git push --force-with-lease origin feat/audit-log
    → if a teammate had pushed a review-fix commit while I was squashing, this refuses the push and I notice.

Force-pushed over main (catastrophic):
    git push --force origin main
    → 2 days of commits from 4 engineers, gone. Reflog on origin doesn't help unless GitHub has it cached.
```

---

### 5. Integrate often; never let a branch rot

**Rule:** Sync long-lived branches with `main` on a cadence (daily for active branches, before any review request). Conflicts cost twice as much for every day you defer them.

**Why:** Integration debt is exponential, not linear. Two days behind `main` is a 10-minute rebase; two weeks behind is a half-day session of resolving conflicts in files you don't remember writing, against changes you don't remember reviewing, with semantic conflicts (the code merges cleanly but no longer behaves correctly) that only blow up at runtime. The cheap moment to handle a conflict is right after the change that caused it lands — that author's intent is still loaded in someone's head, and the conflict is small. A week later it's archaeology.

**How to apply:**
- `git fetch` daily on any active branch. `git status` will tell you how far behind you've drifted.
- Pick a default for the team — rebase onto `main` or merge `main` in — and apply it consistently. **Rebase** keeps a linear history (good for `bisect`, easier to read); **merge** preserves the literal sequence of events (good when commits have been reviewed and you want to keep them as-is). This project's `/dev` flow defaults to rebase-onto-main for feature branches; merges happen only at PR-merge time.
- Run the test suite after each integration. A clean merge is not a working merge — semantic conflicts compile and still break behavior.
- If your branch has been open >1 week without merging, that's a smell. Either ship a slice now or close it and rebuild. Long branches are where bugs hide and reviews die.
- If you maintain a stack of dependent branches (B branched off A branched off `main`), rebase from the bottom up — rebase A onto `main`, then B onto A. Going top-down creates phantom conflicts.

**Example:**
```
Bad: branch open 18 days, no fetch since day 2. Reviewer asks for changes; you rebase onto main and hit 14 conflicts across files you
     don't remember, three of them semantic. You "fix" them, push, and a test fails in CI that didn't fail locally — the merge resolved
     a syntactic conflict but missed a renamed argument three layers down.

Good: branch open 4 days. Each morning: git fetch origin && git rebase origin/main, run tests, push --force-with-lease. Conflicts that
      day are small and the author of the conflicting commit is still in slack. Reviewer rebase at PR time is a no-op.
```

---

### 6. PRs are the unit of review — small, scoped, with context

**Rule:** A PR is reviewed; a diff is not. Keep PRs small (target ≤400 lines of meaningful diff), single-purpose, and shipped with a description that says what changed, why, and how to verify.

**Why:** Code review is a human process, and humans have a fixed amount of attention. A 1500-line PR is not actually reviewed — it's skimmed and approved. Empirical studies converge on the same number: review quality falls off a cliff somewhere around 400 lines. The PR description is also the only place where the *why* lives in a form the next person to touch the code can find — the diff doesn't speak for itself, and the Slack thread you discussed it in is gone in a month. A good description is two minutes to write and saves the reviewer ten, and saves the next bisect-archaeologist an hour.

**How to apply:**
- One PR = one logical change. If you can't title it without "and," split it. Stacked PRs (a chain of small dependent PRs) are almost always better than one mega-PR.
- Description shape (and yes, every PR deserves one): **Summary** (2–4 bullets — what changed at the behavior level), **Why** (the motivation, the constraint, the linked issue), **Test plan** (how you verified it; a checklist the reviewer can re-run). UI changes get a before/after screenshot or GIF. API changes get an example request/response.
- Open as a **draft** while you're still iterating; flip to ready only when CI is green and you'd be comfortable with someone merging it as-is. Don't request review on a red PR — it teaches reviewers to ignore CI.
- Respond to review comments inline; resolve threads when you've addressed them, don't unilaterally mark "resolved" on threads the reviewer raised — let them close their own.
- Don't squash other people's commits out of a PR unless explicitly told to. Their authorship is a fact.
- See `references/pull-requests.md` for the full PR template, the stacked-PR pattern, draft/ready discipline, and review-comment etiquette.

**Example:**
```
Bad PR: "Audit log + minor refactors"
        2,400 lines, 38 files. No description. CI red on three checks. Reviewer left an "LGTM" 4 minutes after open. A regression
        ships. Nobody can identify which commit caused it.

Good PR: "feat(audit): record user_id and actor on audit rows"
         350 lines, 6 files. Description: 3 bullets on what, 2 sentences on why (compliance requirement, links the ticket),
         a 4-line test plan ("hit POST /orders as logged-in user, confirm audit row has user_id; same call with no session, confirm
         actor='system'; run integration suite; check migration on a copy of staging DB"). CI green. Reviewer reads it in 8 minutes,
         leaves three substantive comments, gets a real round of review.
```

---

### 7. Recover with the reflog before you destroy

**Rule:** Before any destructive command (`git reset --hard`, `git checkout --`, `git push --force`, `git clean -fd`, `git branch -D`), pause and ask: what would I run to get this back if I'm wrong? If the answer isn't "I have it in the reflog / a remote / a stash," don't run it.

**Why:** Git almost never actually loses your work — `git reflog` keeps a 90-day record of every commit your `HEAD` ever pointed at, even ones that look "gone" after a reset or a bad rebase. The work that gets lost is almost always lost *because someone tried to fix the problem with another destructive command*, after the first one. `git reset --hard` on a dirty tree before stashing is a classic. Force-pushing right after an "ugh, this rebase is broken" is another. The discipline is: **stop, breathe, list your options, pick the reversible one**. Reflog is the safety net under almost every git mistake — but only if you don't reach for `--hard` again first.

**How to apply:**
- `git reflog` is your friend. Run it before any "I think I lost something." The commit SHAs are still there for 90 days by default; you can `git switch -c rescue <sha>` to recover.
- Prefer reversible alternatives: `git stash` before `git reset --hard`. `git revert` (which adds a commit) before `git reset` (which rewrites history). `git push --force-with-lease` before `git push --force`. `git switch -c backup-branch` before any rebase you're nervous about.
- If you suspect you've lost work and your first instinct is "let me just clean things up and start over," **stop**. That's the move that makes it unrecoverable. Inspect first (`git status`, `git reflog`, `git fsck --lost-found`), recover second.
- For uncommitted work that vanished: `git fsck --lost-found` will surface dangling blobs from `git add` operations even if you never committed.
- Don't use `--no-verify`, `--no-gpg-sign`, or other hook-skipping flags to "just get past" a check. Those checks are usually telling you that a destructive thing is about to happen.
- See `references/recovery.md` for the full recovery playbook — recovering from bad rebase, lost stash, accidental `reset --hard`, force-pushed branch, deleted branch, detached HEAD work.

**Example:**
```
Bad sequence:
  $ git rebase main         # conflicts everywhere, panic
  $ git rebase --abort      # ok, back to normal
  $ git reset --hard HEAD~5 # wait, where did my last 5 commits go?
  $ git reset --hard origin/feat/audit  # ok now they're really gone
  → 2 hours of work, dropped. Reflog has them, but you don't think to look until tomorrow.

Good sequence:
  $ git rebase main                 # conflicts, panic
  $ git rebase --abort              # back to normal
  $ git switch -c rescue/audit      # make a backup branch first
  $ git reflog                      # see exactly where HEAD has been
  $ git rebase main                 # try again, calmly, knowing rescue/audit holds the SHAs
```

---

## Pre-flight checklist

Before any non-trivial git action — branch, commit, rebase, push, PR, recovery — run through these in your head:

1. **Base:** is my starting point an up-to-date `main`? Does this branch do exactly one thing?
2. **Atomicity:** can I describe this commit in one imperative sentence without "and"? Does the tree compile and pass tests at this commit?
3. **Message:** does my subject say *what* in the imperative? Does the body — when there is one — say *why*?
4. **History scope:** is this rewrite touching commits only I have, or commits others may have fetched? If shared, am I using `--force-with-lease` and have I told the team?
5. **Freshness:** how long since this branch saw `main`? More than a day on active work is debt.
6. **Reviewability:** is the PR small enough that a human will actually read it (~≤400 lines of meaningful diff)? Does the description carry the *why* and a test plan?
7. **Recoverability:** before this destructive command, what's my path back if I'm wrong? Have I checked `reflog`, made a backup branch, or used a reversible alternative?

If any answer is "I don't know," stop and find out before running the command. A 30-second check beats a 3-hour recovery.

## When to skip this skill

- One-off read-only operations (`git status`, `git log`, `git diff`, `git show`). Nothing is being written; nothing can be wrong.
- A `git pull` on a clean, up-to-date branch where you have no local commits to lose.
- Single-line trivial commits on a personal scratch repo with no collaborators. (Even here, principle 3 — the commit message — still pays for itself in a month.)

For anything else — yes, even "just a quick fixup," even "I'll squash it before pushing" — these fundamentals apply. The git mistakes that eat days are almost always the ones where the first thought was "this is too small to think about."

## Relation to other skills

Git workflow is the *delivery channel* for almost every other skill in this foundation. They compose, they don't compete:

- [[programming-fundamentals]] — the code inside a commit. The atomic-commit principle (#2 above) is the runtime cousin of "one function, one thing" — same discipline, different unit.
- [[database-fundamentals]] — migrations land via commits and PRs. The expand → backfill → contract pattern is a *sequence of commits*, and the PR description is where the deploy order lives.
- [[debug-fundamentals]] — `git bisect` is the canonical implementation of debug-fundamentals principle 4 (bisect the search space). It only works because of principles 1–2 here: atomic commits on a coherent branch history. Sloppy commits make bisect useless.
- [[hexagonal-backend]] — feature work that crosses layers tends to want one PR per layer in a stack, or one well-described PR with a section per layer. Either is fine; "one giant PR across layers" is the smell.
- [[queue-fundamentals]] — outbox migrations, consumer rewrites, broker changes all want their own atomic commits and an explicit rollout note in the PR.

Run order when multiple apply: use the *construction* skills (programming / database / hexagonal / queue / debug) to decide what to write; use this skill to decide *how to commit, branch, and ship* what you wrote. They're independent dimensions — get both right, not just one.

## Reference files

Deeper guides for individual areas. Read the one that matches what you're stuck on; you don't need to read them all upfront.

- `references/commit-messages.md` — conventional commits cheat sheet, type/scope/subject anatomy, body and footer conventions, breaking-change syntax, common anti-patterns.
- `references/branching-and-rebasing.md` — branch naming, trunk-based vs feature branches, interactive rebase, `--fixup` / `--autosquash`, `--force-with-lease`, stacked branches, rebase-vs-merge for integration.
- `references/pull-requests.md` — PR description template, the stacked-PR pattern, draft/ready discipline, review-comment etiquette, PR size targets, when to split.
- `references/recovery.md` — recovering from bad rebase, lost commits, deleted branches, accidental `reset --hard`, force-pushed work, detached HEAD edits, lost stashes; the `reflog` and `fsck --lost-found` playbook.
