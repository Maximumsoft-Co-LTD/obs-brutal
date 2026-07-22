# Security review: <title>

**Plan**: [./plan.md](./plan.md)
**Reviewed**: YYYY-MM-DD
**Verdict**: pass | fix-required
**Trigger**: <bucket(s) the diff touched — auth | crypto | sql | html | path | exec | deserialise | secrets | network>

## Threat model *(required)*

One paragraph: what an attacker would try given this diff. Trust boundaries crossed. Who can reach the new code.

## Checklist *(required)*

Walk ONLY the buckets your `Trigger` names; mark ✓ / ✗ / N/A with a one-line note.

- [ ] **Input** — validated at the boundary; no string-concat into SQL / shell / HTML / paths; safe parser for untrusted input (no `pickle.loads`, `yaml.load`, `eval`)
- [ ] **Authn / authz** — explicit authz check per new endpoint; session/token httpOnly + secure + SameSite; no admin path skips the authz layer
- [ ] **Secrets / crypto** — no hard-coded secrets / keys / test creds; no custom crypto; CSPRNG for security randomness
- [ ] **Output** — untrusted text escaped on the way out (HTML / JSON / logs); redirect targets allowlisted; errors don't leak traces / paths
- [ ] **Infra-adjacent** — path joins via `path.join` / `filepath.Clean` rejecting `..`; new outbound call has a timeout + allowlist; new exec doesn't shell out with user input

## Findings *(required)*

### Blocking (severity = high)

- `path:line` — <issue> → <fix>

### Non-blocking (severity = medium / low)

- `path:line` — <issue> → <fix or accepted risk>

## Sign-off *(required)*

pass | fix-required (counts against the review cycle budget)

---

**Fanout-only section** — add when surface-axis fanout ran:

- **Per-repo security** — one `### Repo: <path>` per tripping repo; Trigger / Verdict / Findings stay global

Shape → **orchestrator/references/fanout-dispatch.md > Lead — Mode C**.
