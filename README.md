# fault-injection

[![CI](https://github.com/faustbrian/go-fault-injection/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/faustbrian/go-fault-injection/actions/workflows/ci.yml)
[![CodeQL](https://img.shields.io/badge/CodeQL-required-blue)](https://github.com/faustbrian/go-fault-injection/actions/workflows/ci.yml)
[![Coverage](https://img.shields.io/badge/coverage-100%25_required-blue)](CONTRIBUTING.md#verification)
[![Mutation](https://img.shields.io/badge/mutation-100%25_required-blue)](CONTRIBUTING.md#verification)
[![Documentation](https://img.shields.io/badge/docs-checked_in_CI-blue)](docs/)
[![Go Reference](https://pkg.go.dev/badge/github.com/faustbrian/go-fault-injection.svg)](https://pkg.go.dev/github.com/faustbrian/go-fault-injection)
[![Release](https://img.shields.io/github/v/release/faustbrian/go-fault-injection?sort=semver)](https://github.com/faustbrian/go-fault-injection/releases)
[![Go](https://img.shields.io/badge/go-1.27.0-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

`fault-injection` is a deterministic, concurrency-safe, bounded toolkit for
exercising failure paths in Go tests and explicitly wired controlled
experiments. Its zero value is disabled, configuration is copied and validated
before use, and it has no environment-variable activation path, background
worker, unbounded history, or remote control surface.

It is not a mocking framework, production chaos control plane, Kubernetes
operator, broker simulator, or substitute for a real network proxy.

Browse the versioned [Golib ecosystem index](https://github.com/faustbrian/go-library-tools/blob/v1.4.0/docs/ecosystem/README.md)
and its [resilience family](https://github.com/faustbrian/go-library-tools/blob/v1.4.0/docs/ecosystem/design-language.md#package-families-and-selection)
to compare focused policies and composition boundaries.

The published v1.0.0 artifact declares Go 1.26.6. Current source is planned v2,
requires Go 1.27.0 or newer, and is not published yet.

## Install

```sh
go get github.com/faustbrian/go-fault-injection@v1
```

After v2 is published, adopters can migrate with:

```sh
go get github.com/faustbrian/go-fault-injection/v2@v2
```

## Quick start

The example below describes the pending v2 source, not the published v1 API.

```go
injector, err := faultinject.New(faultinject.Config{Rules: []faultinject.Rule{{
    ID:          "second-call",
    Scope:       faultinject.BoundaryFunction,
    Activation:  faultinject.Active,
    Maximum:     1,
    Terminal:    faultinject.Continue,
    Observation: faultinject.Suppress,
    Schedule:    faultinject.Nth(2),
    Faults: []faultinject.Fault{
        faultinject.ErrorFault(faultinject.PhaseBefore, ErrUnavailable),
    },
}}})
if err != nil {
    return err
}

runtime, err := faultinject.NewRuntime(faultinject.RuntimeConfig{
    Injector:           injector,
    Authorizer:         authorizeExperiment,
    Allowlist:          []faultinject.Boundary{faultinject.BoundaryFunction},
    ExpiresAt:          time.Now().Add(15 * time.Minute),
    MaximumEvaluations: 10,
    Auditor:            auditExperiment,
})
if err != nil {
    return err
}

value, err := faultinject.Run(ctx, runtime,
    faultinject.Metadata{Boundary: faultinject.BoundaryFunction}, operation)
```

## Lifecycle and ownership

An `Injector` and `Runtime` own only bounded in-memory rule, schedule,
counter, allowlist, and safety-gate state. They start no goroutines and own no
external resources or shutdown operation. Both are safe for concurrent use.

Construct both objects inside the test or experiment composition root and pass
the runtime explicitly to application APIs and adapters. A nil or zero
`Runtime` delegates directly and cannot become active later. Application APIs
do not accept an `Injector`, so authorization, allowlisting, expiry, budget,
auditing, and emergency disable cannot be bypassed by adapter wiring.

Constructors copy slices and fault value data. Interface collaborators are
borrowed: keep `Clock`, `Sleeper`, and `Observer` valid for the `Injector`
lifetime, and keep the `Injector`, `Authorizer`, `Clock`, and `Auditor` valid
for the `Runtime` lifetime. Collaborators may be invoked concurrently and must
satisfy the concurrency contract documented by their interface.

## Deterministic model

- Rules are ordered by `Order`, then stable `ID`.
- Mutex acquisition defines concurrent evaluation order. Record event
  `Sequence` values when replaying a concurrent campaign.
- `Nth`, `Every`, finite/repeating `Sequence`, and seeded `Probability`
  schedules are deterministic for the same eligible-call order.
- Probability uses an explicit seed and a deterministic call sequence; an
  unseeded probability-only mode does not exist.
- `Maximum` bounds rule activations. `MaxRules`, `MaxFaultsPerDecision`,
  `MaxLatency`, and `MaxBytes` bound configuration, work, and retained data.
- `Reset` starts a new generation. Decisions already returned retain their old
  generation and remain valid.
- Synchronous observers run after selection and outside engine locks. They
  cannot veto or rewrite a fault.

See [API](docs/api.md) and [deterministic recipes](docs/operations.md).

## Release sequence

The non-releasable resilience and comparison modules temporarily consume
published v1.0.0 so ordinary module-local CI can resolve their dependencies
before root v2 publication. They do not currently prove the pending v2 API.
Root v2 retains its own runtime security regressions and Go 1.27 minimum.

Publish root v2 from `main` using its root major-version tag, then migrate both
internal modules to the published `/v2` artifact. The rollout is unfinished
until both published-v2 consumers and ordinary main CI pass. No workspace
substitution, permanent replacement, or reduced CI selection supplies that
release evidence. Support changes at publication as described in
[SECURITY.md](SECURITY.md).

## Faults and adapters

The core model supports injected errors, bounded latency, cancellation,
deadline expiry, bounded safe-string panics, byte drop/truncation/duplication/
reordering/corruption, short IO, temporary and permanent network failures,
reset, half-close, and stream interruption.

Adapters cover:

- generic `Run` execution;
- `http.RoundTripper` and caller-owned response bodies;
- `io.Reader` and `io.Writer`;
- `net.Conn`, context dialers, and listeners;
- `fs.FS` open/read boundaries; and
- context sleepers and timer factories.

Every adapter defines its partial-result, close, and ownership behavior in
[adapter contracts](docs/adapters.md). During-operation faults are observable
in-process boundary simulations, not packet-, kernel-, broker-, or scheduler-
level failures.

## Controlled runtime experiments

Every application path uses `Runtime`, including tests. It requires an explicit
injector, authorizer, exact boundary
allowlist, expiry, maximum evaluation budget, audit sink, and terminal emergency
disable. It fails closed on authorization or clock failure and never consults
the environment. See [security](docs/safety.md).

## Kubernetes and infrastructure scope

An in-process injector affects one process in one pod. Independent pod-local
seeds do not implement a fleet percentage. Replica selection, blast radius,
rollout coordination, and disruption are owned by an external orchestrator.
See [Kubernetes semantics](docs/kubernetes.md) and the
[Toxiproxy comparison](docs/comparison.md).

## Choosing the right test boundary

Use this module when deterministic call- or adapter-boundary behavior is the
contract under test. Prefer a direct test double for a single isolated return
value. Use Toxiproxy or another infrastructure tool when TCP proxy or real
network behavior matters. Use a cluster experiment system when pod selection
or fleet blast radius matters.

Dependency-heavy database, cache, queue, Kafka, object-storage, and RPC
integrations belong in nested modules or downstream repositories so the root
production package remains standard-library-only. The root module uses only a
test-scoped leak detector. See [extension guidance](docs/extension.md).

## Verification

From the repository root:

```sh
make inventory
make check MODULES=.
make ci-changed BASE=<revision>
```

The module includes exact statement coverage, deterministic golden schedules,
race/stress coverage, fuzz targets, adapter contract tests, and benchmarks.
Repository gates add mutation, API compatibility, documentation, security,
supply-chain, and clean-consumer checks.

## References

- [Failsafe-Go policy composition and events](https://failsafe-go.dev/)
- [goresilience chaos behavior](https://pkg.go.dev/github.com/slok/goresilience)
- [Toxiproxy fault model](https://github.com/Shopify/toxiproxy)
- [Go `net/http` contracts](https://pkg.go.dev/net/http)
- [Go `io` contracts](https://pkg.go.dev/io)
- [Go context](https://pkg.go.dev/context)
- [Kubernetes pod lifecycle](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/)

## Documentation

- [Documentation index](docs/README.md)
- [API and adapter contracts](docs/api.md)
- [Safety and controlled runtime experiments](docs/safety.md)
- [Threat model](docs/threat-model.md)
- [Performance methodology](docs/performance.md)
- [FAQ](docs/faq.md)
- [Support](SUPPORT.md)
- [Security policy and reporting guidance](SECURITY.md)
- [Compatibility policy](COMPATIBILITY.md)
- [Release history](CHANGELOG.md)

## License

MIT. See [LICENSE](LICENSE).
