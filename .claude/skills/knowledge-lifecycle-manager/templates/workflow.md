---
source_commit: <sha>
generated_at: <YYYY-MM-DD>
generator: knowledge-lifecycle-manager
confidence: HIGH | MEDIUM | LOW
status: CURRENT | STALE | REVIEW | DEPRECATED
---

# Workflow: <name>

<!-- MANAGED_START -->

## Purpose
<What this flow accomplishes, in one or two sentences. Business outcome, not mechanics.>

## Trigger
<What starts it: HTTP route, message/event, cron, scheduler. Name the entry symbol.>

## Steps
1. <step — name the symbol that performs it, e.g. `controller.HandleSlipVerify`>
2. <step>
3. <step>

## Dependencies
- **Modules:** <internal packages this flow touches>
- **Services:** <internal services called>
- **External integrations:** <third-party / network calls>
- **Events / queues:** <published or consumed; "none" if synchronous>
- **Data stores / collections:** <DB collections, caches read/written>

## Failure Scenarios
| Failure | Detection | Behavior | Evidence |
|---------|-----------|----------|----------|
| <what fails> | <log tag / status / signal> | <fallback / error returned> | <symbol or test> |

## Evidence
```
Workflow:     <self, or upstream workflow in maintenance mode>
Code Symbol:  <pkg.Func that anchors this flow>
Test:         <test name, or MISSING>
Incident:     <related incident id, if any>
Confidence:   HIGH | MEDIUM | LOW
```

<!-- MANAGED_END -->
