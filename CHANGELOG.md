# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

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

## [1.0.0] - 2026-08-25

### Changed

- Publish the module from its standalone `github.com/faustbrian/go-fault-injection` identity while preserving its documented API and behavior.

### Documentation

- Link the package README to the repository-wide Golib documentation portal.

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
