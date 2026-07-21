# Compatibility policy

`obs-brutal` follows [Semantic Versioning 2.0.0](https://semver.org/spec/v2.0.0.html).
This document defines exactly what's covered by that contract so users
can pin `boeng` and expect it to behave.

## Public surface (covered by semver)

The full public API of v1.x is enumerated by
[`boeng/api_freeze_test.go`](./boeng/api_freeze_test.go). It compiles a
list of every exported identifier and the build fails if any of them
disappears or changes shape.

Concretely, semver applies to:

- Package `github.com/Maximumsoft-Co-LTD/obs-brutal/boeng` — every exported identifier (types,
  functions, methods, constants, variables).
- The five adapter sub-packages:
  - `github.com/Maximumsoft-Co-LTD/obs-brutal/boeng/gin`
  - `github.com/Maximumsoft-Co-LTD/obs-brutal/boeng/http`
  - `github.com/Maximumsoft-Co-LTD/obs-brutal/boeng/mongo`
  - `github.com/Maximumsoft-Co-LTD/obs-brutal/boeng/redis`
  - `github.com/Maximumsoft-Co-LTD/obs-brutal/boeng/rabbit`
- The JSON output schema produced by the default stdout sink
  (`datetime`, `level`, `msg`, `service`, `env`, `op`, `user_id`,
  `module`, `tenant_id`, `trace_id`, `span_id`, `request_id`,
  `duration_ms`, `error`, plus user fields).
- The names of auto-generated metrics: `<op>_total`,
  `<op>_duration_ms`, `<op>_error_total`, `<op>_panic_total`,
  `<event>_total`.

## NOT covered by semver

Everything under `internal/` is implementation detail and may change
in any release, including patch releases. This includes:

- `github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/...`
- `github.com/Maximumsoft-Co-LTD/obs-brutal/internal/adapter/...`
- `github.com/Maximumsoft-Co-LTD/obs-brutal/internal/logtrc/...`
- `github.com/Maximumsoft-Co-LTD/obs-brutal/internal/util/...`

Build-tagged test helpers (`*_test.go` and `export_test.go`) are also
exempt.

## What counts as a breaking change

Removing, renaming, or changing the signature of any covered symbol.
The full list:

- Removing or renaming any exported function, method, type, variable,
  or constant in the public surface above.
- Changing a function's signature (parameter or return types, parameter
  count, including widening from concrete to interface or vice versa).
- Changing an interface (adding a method, narrowing the signature).
- Changing a struct's exported field type or removing an exported field.
- Renaming a JSON output field key in the default schema.
- Renaming an auto-generated metric or removing one of the four
  per-op metrics.
- Changing the semantics of an existing call so that previously valid
  code now produces a different observable outcome (e.g. silently
  swallowing a panic that used to re-raise).

## What does NOT count as breaking

- Adding new exported identifiers, methods on existing types, or
  fields with safe zero values to `Config`.
- Adding new adapter subpackages.
- Adding new JSON fields in the output schema (consumers must accept
  unknown fields).
- Adding new auto-generated metrics, as long as the existing ones stay.
- Performance improvements that don't change the contract.
- Internal refactors not visible from the public surface.
- Documentation, README, CHANGELOG, examples, testdata, CI changes.

## Deprecation policy

When a public symbol must be retired:

1. Mark it with a `// Deprecated:` godoc comment in the same minor
   release the replacement ships. The comment must name the
   replacement.
2. Keep the deprecated symbol working for **at least two minor
   releases** (e.g. deprecated in v1.4 → earliest removal in v2.0).
3. Removal can only happen at the next major bump.

For minor breaking changes that are strictly necessary for safety
(security fix, severe correctness bug), a v2.x release will be cut
rather than slipping the change into v1.x silently.

## Version compatibility

- v0.x — pre-release, anything may change.
- v1.x — covered by the above policy.
- v2.0 — first allowed major break; documented one full release cycle
  in advance via the CHANGELOG.

## Supported Go versions

| Go release | Status |
| ---------- | ------ |
| 1.25       | Primary — CI runs against this |
| 1.24       | Best-effort — code targets 1.22+ features only |
| 1.23       | Best-effort |
| 1.22       | Minimum supported (matches the lowest version in `go.mod`) |
| ≤ 1.21     | Unsupported |

Bumping the minimum supported Go version is itself a breaking change
under this policy.

> Verified against `0d0a832` · 2026-07-22
