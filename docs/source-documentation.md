# Source documentation audit

[Documentation map](index.md) · [Repository README](../README.md)
[Next: API reference](api.md)

## Audited scope

Baseline `b996ebf18469be88ed8cc19cea8055185b151595` has 32 tracked Go
files: twelve maintained implementation files and twenty test files across the
root library, `benchmarks/comparison` and `integration/resilience` modules.
The latter modules are non-releasable consumers of published v2.0.0, not optional
runtime packages or proof of the local root revision. The tracked Go inventory
has no build/platform variants, vendor files, generated source or embedded
executable templates. Four root fuzz targets, nine root benchmarks, one
comparison benchmark and three executable examples were inspected.

Reproduce the boundary with `git ls-files '*.go' 'go.mod' '**/go.mod'` and
`golib source-docs inventory --json`. The existing root package comment did not
explain evaluation ordering, borrowed collaborator lifetime or application
refusal versus fault refusal. The baseline had 115 objective member-comment
findings. No debt, deprecation or generation directives require resolution;
the compatibility lint directives around `net.Error.Temporary` remain intact.

| Source | Audited contract and documentation outcome |
| --- | --- |
| `types.go`, `fault.go`, `schedule.go` | Safe attribution identifiers, explicit nonzero enums, constructor-time versus New-time validation, retained injected errors, deterministic counter-based schedules and adapter-specific phase support. |
| `engine.go` | Copied configuration versus borrowed callbacks, rule-call eligibility, activation counts, truncated composition, lock-acquisition ordering, synchronous observation and saturation/reset boundaries. |
| `runtime.go` | Copied allowlist, sampled expiry, authorization and audit obligations, evaluation budget including no-match/replay, final selection versus terminal disable, and non-atomic runtime snapshots. |
| `execute.go` | Zero-result injected failures, already-canceled child contexts, no goroutine/recovery, and caller-owned operation effects/resources. |
| `io.go`, `http.go`, `net.go`, `filesystem.go`, `time.go` | Bounded replay, counted partial results, caller buffer ownership, underlying concurrency/lifecycle, contextless interfaces, cleanup of rejected acquired resources, and half-close fallback. |
| All tracked root tests | Declared configuration boundaries, copied schedules, seeded golden sequence, reset generations, callback panic containment, terminal disable/replay admission and private fixture ownership. |
| Nested test-only modules | Published consumer identity, finite in-process retry/breaker campaigns and comparison workloads with intentionally unequal setup/assertion costs. |

The package defines application policy rather than claiming packet, kernel,
cluster or protocol conformance. Existing [adapter](adapters.md),
[safety](safety.md), [operations](operations.md) and
[comparison](../benchmarks/comparison/README.md) guides retain their ownership.
Source comments explain the adjacent constraints without duplicating a control
plane or evidence framework.

## Verification boundary

This pass preserves executable tokens, literals, tests, example outputs,
lint directives, all module/workspace/dependency bytes and API signatures.
The configured `api/v2/baseline.txt` mode is `apidiff`, not `go-doc`; comments
cannot change that symbol snapshot and it is not regenerated as prose. The v1
baseline remains historical. Root `go doc -all .` provides the transient rendered
API view; root examples run with `GOWORK=off` to avoid substituting the local root
into published nested-module consumers. All tracked Go files, including both
nested harnesses, receive syntax/token and objective-source inspection.

No unchanged race, active fuzz, leak campaign, performance comparison, nested
integration execution or release rehearsal is selected for this prose-only pass.
The normal root example process retains its existing TestMain cleanup check.
Changed documentation receives local-link, secret and complete-diff inspection.
The coordinator owns remote-main/exact CI and shared-enforcement adoption;
local source proof does not certify those boundaries. No module release or
consumer repinning is required by this documentation change.

## Objective Enforcement

The qualified public Tools v2.4.0 binary and immutable reusable CI source
provide the strict tracked-source check through `make source-docs-check`,
the existing local check/CI routes and Required aggregation. Release and
selected security/assurance routes retain their existing controls. This
structural status supplements the rendered semantic audit; it does not
certify prose meaning, generator execution or runtime conformance.
