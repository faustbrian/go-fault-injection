# Threat model

Model version: 1
Last reviewed: 2026-09-13

## Scope and security objective

This model covers the root `faultinject` module, including `Run`, HTTP, IO,
network, filesystem, sleeper, and timer adapters. Its objective is that no
built-in fault application path can select or apply a fault without first
passing the same explicit runtime authorization, boundary allowlist, expiry,
finite evaluation budget, audit, and terminal-disable controls.

The model does not cover external experiment orchestration, fleet selection,
configuration delivery, process isolation, or application code that implements
its own failures without using this package.

## Assets and trust boundaries

Assets are application availability, caller-owned resources, the finite fault
budget, authorization and expiry policy, audit attribution, and secrets held by
the host application. The following cross trust boundaries:

- application composition code supplies validated rules and runtime controls;
- operation metadata crosses from the host application into authorization,
  audit, and deterministic rule selection;
- injected faults cross back into the owned application boundary;
- caller-provided `Authorizer`, `Auditor`, `Clock`, `Sleeper`, `Observer`, and
  delegated adapters execute inside the host process; and
- external orchestration selects processes or pods outside this module.

The host application and its composition root are trusted to retain the
runtime, protect any experiment-enabling configuration, choose safe numeric
metadata, and invoke terminal disable during removal. Operation inputs,
request-derived data, and remote users are untrusted and must not control rule
identity, boundary names, authorization, expiry, or budgets.

## Activation and application flow

1. The composition root constructs an `Injector` from explicit validated
   rules; there is no ambient environment, file, flag, network, or registry
   activation path.
2. It constructs a `Runtime` with an authorizer, exact allowlist, expiry,
   finite evaluation budget, auditor, and optional clock.
3. `Run` or an adapter receives only the runtime. A nil or zero runtime
   delegates without injection.
4. The runtime rejects disabled, expired, denied, non-allowlisted,
   budget-exhausted, and clock-failed attempts, and attempts to audit the exact
   outcome.
5. Only an authorized runtime decision reaches fault application. Terminal
   disable prevents all subsequent decisions.

`Injector.Decide` remains a public deterministic selection primitive for
analysis and custom tooling. It cannot apply a package fault by itself, and no
built-in application API accepts an injector.

## Threats, controls, and evidence

| Threat | Control | Evidence |
| --- | --- | --- |
| Adapter wiring bypasses runtime policy | Every built-in application API accepts `*Runtime`; selection occurs through `Runtime.Decide` | Runtime application tests exercise `Run` and HTTP through denial, authorization, budget, expiry, audit, and disable outcomes; API compatibility gate records the intentional break |
| Ambient or remote activation | Constructors require explicit in-process values; the package reads no environment, files, flags, endpoints, or global registry | Constructor and zero-value tests; source and security scans |
| Boundary widening or request-controlled targets | Runtime requires a bounded exact allowlist; metadata uses known boundaries and numeric operation and attempt values | Validation, allowlist, metadata, and fuzz tests |
| Stale authorization, expired campaign, or unbounded total evaluations | Authorization runs per evaluation before selection; expiry and a finite evaluation budget fail closed | Runtime lifecycle and application-path tests |
| Emergency disable can be reversed | Disable is terminal and checked by every subsequent runtime decision | Concurrent disable and application-path tests |
| Secrets leak through events | Identifiers and panic strings are bounded safe strings; metadata excludes domain payloads; selected errors are not copied into events | Validation, event, audit, and fuzz tests |
| Fault corrupts ownership or leaks acquired resources | Adapters define partial results and close rejected responses, files, connections, and timers | Adapter-boundary and leak tests |
| Callback panic bypasses policy | Authorization and clock panics fail closed; observer and auditor panics are contained | Runtime, hardening, and mutation tests |
| Resource exhaustion inside the engine | Rule count, selected faults, latency, byte state, schedules, allowlist, identifiers, and evaluations are bounded | Limit, fuzz, benchmark, and runtime-budget tests |

## Accepted residual risks

| Risk | Owner | Rationale and mitigation | Review condition |
| --- | --- | --- | --- |
| A malicious or compromised host process can ignore this library and implement failures directly | Host application owner | An in-process library cannot constrain arbitrary host code. Restrict experiment configuration and deployment authority outside this module | Revisit if the package gains a control plane, plugin loading, or cross-process authority |
| A selected in-flight fault may complete after `Disable` | Host application owner | Disable prevents subsequent decisions but cannot safely recall a decision already returned. Bound caller contexts and configured latency; stop external experiment selection before disable | Revisit if cancellable in-flight campaign semantics become a requirement |
| Auditor failure or panic can mean an attempted audit record is not durably stored | Audit integration owner | The package contains callback failure so it cannot enable a fault; use a bounded reliable sink and monitor sink health | Revisit if durable or transactional audit delivery becomes part of the contract |
| Synchronous Reader, Writer, Conn, Listener, FS, sleeper, and timer methods have no caller context for authorization | Authorizer implementer | They authorize with a background context. Authorizers must return promptly and must not perform network or other blocking I/O | Revisit if an adapter can accept an operation context without violating its standard-library interface |
| Authorizer, clock, sleeper, observer, auditor, and delegated implementations can block or consume resources | Host application owner | Interfaces are borrowed and must be concurrency-safe and bounded; caller contexts bound context-aware calls | Revisit when adding a collaborator or permitting remote callbacks |
| Fault error values can contain sensitive data and are returned to the direct caller | Rule author | Errors are intentionally part of the injected behavior but are excluded from events. Use static, non-sensitive error values | Revisit if errors are ever serialized, logged, or audited by this module |
| A finite total evaluation budget is not a rate limit | Host application owner | The budget caps cumulative authorized evaluations. Enforce per-time or per-tenant rate limits before authorization | Revisit if this module adds time-windowed or tenant-aware policy |
| Independent process runtimes do not enforce a fleet-wide blast radius | Experiment orchestrator owner | The module is deliberately in-process. Use external process selection, rollout controls, and kill switches | Revisit if fleet discovery or orchestration enters package scope |

## Change review requirements

Changes to application signatures, runtime decision ordering, authorizer or
auditor behavior, allowlist or expiry semantics, evaluation accounting,
disable, metadata, fault payloads, or adapter ownership require a Tier C
security review. That review must inventory every built-in application API and
repo-owned consumer, exercise an end-to-end application path, check the public
API transition, and update this model when a trust boundary or residual risk
changes.
