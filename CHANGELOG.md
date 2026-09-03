# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- Adopt the checksum-verified `go-library-tools` v1.4.0 CLI and immutable W14
  reusable workflow, and reconcile nested Golib dependency checksums with
  their published v1.0.0 archives without changing selected versions, the
  fault-injection API, or runtime behavior.

- Publish schema-v2 cohesion metadata for the public fault-injection module,
  including its ownership, construction, lifecycle, and companion boundaries.
- Adopt checksum-verified go-library-tools v1.3.0 validation locally and pin
  CI to its immutable cohesion-enforcing reusable workflow.

- Adopt the released go-library-tools v1.0.13 contract through a pinned
  reusable workflow and strict repository configuration while preserving
  the root API baseline and approved mutation evidence.

### Documentation

- Link ecosystem and resilience-family guidance to the immutable v1.4.0
  documentation release.

- Document copied configuration data and the retained lifetime of borrowed
  injector and runtime collaborators.
- Link the package entry point to the versioned Golib ecosystem and resilience
  family guidance.

- Use task-oriented README headings instead of internal planning terminology.

- Replace the archived monorepo link with package-owned documentation.

## [1.0.0] - 2026-08-25

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

[Unreleased]: https://github.com/faustbrian/go-fault-injection/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/faustbrian/go-fault-injection/releases/tag/v1.0.0
