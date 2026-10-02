# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [2.0.1] - 2026-10-02

### Changed

- Raise the module graph's indirect Testify minimum from v1.8.0 to
  v1.11.1. Consumers using Testify may select the newer version through
  Go module version selection; the fault-injection API is unchanged.

## [2.0.0] - 2026-09-27

### Security

- Coordinate terminal emergency disable with final budget admission and fault
  selection. Authorization or predicate work still pending when disable wins
  cannot select a fault; already-selected work may finish. Caller callbacks
  remain outside the coordination lock.

- Recheck runtime safety gates and consume an evaluation for every buffered
  duplicate read, including HTTP bodies, files, and connections. Rejected
  replays discard queued fault bytes; admitted replays retain their original
  decision attribution in the audit event.
  ([2eed2002eb](https://github.com/faustbrian/go-fault-injection/commit/2eed2002eb))

- Require `Runtime` for `Run` and every built-in adapter so fault application
  cannot bypass authorization, boundary allowlisting, expiry, evaluation
  budget, auditing, or terminal emergency disable. This is a breaking public
  API change in the published `/v2` module path. Callers must update their
  imports, construct a runtime, and
  replace every application API argument that previously passed an injector.
  ([50cc3bdd5e](https://github.com/faustbrian/go-fault-injection/commit/50cc3bdd5e))

### Changed

- Require Go 1.27.0 for the v2 module and its internal harnesses. The
  published v1.0.0 artifact retains its declared Go 1.26.6 minimum.
  ([1fb61c7478](https://github.com/faustbrian/go-fault-injection/commit/1fb61c7478),
  [43b1ef825f](https://github.com/faustbrian/go-fault-injection/commit/43b1ef825f),
  [9232ab09aa](https://github.com/faustbrian/go-fault-injection/commit/9232ab09aa))

- Upgrade the non-releasable resilience campaign to Retry v1.1.0 and its
  strict policy, execution, and known-outcome contract for deterministic
  in-process fault injection.
  ([a5d75c4909](https://github.com/faustbrian/go-fault-injection/commit/a5d75c4909))

- Adopt the checksum-verified `go-library-tools` v1.4.0 CLI and immutable W14
  reusable workflow, and reconcile nested Golib dependency checksums with
  their published v1.0.0 archives without changing selected versions, the
  fault-injection API, or runtime behavior.
  ([862f516e7b](https://github.com/faustbrian/go-fault-injection/commit/862f516e7b))

- Publish schema-v2 cohesion metadata for the public fault-injection module,
  including its ownership, construction, lifecycle, and companion boundaries,
  and enable the stable root v2 release.
  ([bb118b71ef](https://github.com/faustbrian/go-fault-injection/commit/bb118b71ef),
  [b66025a9ed](https://github.com/faustbrian/go-fault-injection/commit/b66025a9ed))
- Adopt checksum-verified go-library-tools v1.3.0 validation locally and pin
  CI to its immutable cohesion-enforcing reusable workflow.

- Adopt the released go-library-tools v1.0.13 and v1.0.14 contracts through a pinned
  reusable workflow and strict repository configuration while preserving
  the root API baseline and approved mutation evidence.
  ([87184ec497](https://github.com/faustbrian/go-fault-injection/commit/87184ec497),
  [c1ae027d6f](https://github.com/faustbrian/go-fault-injection/commit/c1ae027d6f))

- Replace duplicated local policy automation with the proportional-assurance
  repository policy without changing runtime behavior.
  ([86ccb23eca](https://github.com/faustbrian/go-fault-injection/commit/86ccb23eca))

### Documentation

- Clarify root and independently releasable nested-module tag conventions,
  route adoption discussions and vulnerability reports to their dedicated
  GitHub facilities, and correct the v1.0.0 date to the signed tag and
  published release chronology.
  ([87a7f3331a](https://github.com/faustbrian/go-fault-injection/commit/87a7f3331a))

- Add canonical v1 installation, stable Go support, lifecycle and ownership,
  project support, and security-reporting guidance.

- Link ecosystem and resilience-family guidance to the immutable v1.4.0
  documentation release.

- Document copied configuration data and the retained lifetime of borrowed
  injector and runtime collaborators.
- Link the package entry point to the versioned Golib ecosystem and resilience
  family guidance.

- Use task-oriented README headings instead of internal planning terminology.
  ([8459b9c741](https://github.com/faustbrian/go-fault-injection/commit/8459b9c741))

- Replace the archived monorepo link with package-owned documentation.

- Explain the v2 Runtime migration, published-version Go support, and
  maintainer-operated release sequence.
  ([82b20f3a1f](https://github.com/faustbrian/go-fault-injection/commit/82b20f3a1f))

## [1.0.0] - 2026-08-26

### Changed

- Upgrade the Prometheus comparison dependency to its current secure release.

- Exclude intentional nested modules from root local-proxy archives so local,
  bootstrap, CI, and public module checksums describe the same source
  boundary.

- Track the pinned documentation-tool lockfile so clean CI checkouts install
  the exact validated cspell dependency.

- Reconcile standalone dependency checksums against deterministic current
  module archives so CI, local verification, and release consumers resolve
  identical content.

- Harden standalone documentation validation with deterministic spelling and
  link checks, package-specific documentation gates, and repository-local
  contributor guidance.

### Changed

- Publish the module from its standalone `github.com/faustbrian/go-fault-injection` identity while preserving its documented API and behavior.

### Documentation

- Link the package README to package-owned documentation.

### Added

- Immutable validated rule configuration with stable precedence, bounded
  composition, deterministic nth/every/sequence/seeded schedules, typed
  metadata predicates, snapshots, and generation-safe reset.
- Explicit error, latency, cancellation, deadline, bounded panic, byte, partial
  IO, network, reset, half-close, and interruption faults.
- Generic execution, HTTP transport/body, reader/writer, connection, dialer,
  listener, filesystem, sleeper, and timer-factory adapters with documented
  ownership and partial-result semantics.
- Bounded attribution events and a fail-closed runtime experiment gate with
  authorization, allowlist, expiry, evaluation budget, audit, and terminal
  emergency disable.
- Deterministic golden, exact statement coverage, race/stress, fuzz, adapter
  contract, leak, example, and benchmark evidence.
- Isolated Failsafe-Go/goresilience comparison benchmarks and retry/circuit
  breaker campaign integrations without downstream production dependencies.
- Adoption, API, adapter, operations, security, Kubernetes, infrastructure
  comparison, extension, and FAQ documentation.

### Fixed

- Timer-factory during-phase cancellation now reaches the factory with an
  ended context and stops any timer returned before the injected error.

[Unreleased]: https://github.com/faustbrian/go-fault-injection/compare/v2.0.0...HEAD
[2.0.0]: https://github.com/faustbrian/go-fault-injection/releases/tag/v2.0.0
[1.0.0]: https://github.com/faustbrian/go-fault-injection/releases/tag/v1.0.0
