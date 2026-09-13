# Safety and runtime controls

## Test construction

Construct an `Injector` and a `Runtime` inside a test and pass the runtime to
the owned adapter. Application APIs never accept an injector directly. The
zero and nil runtime are disabled. Build tags are not the sole safety boundary.

## Runtime experiments

Runtime use requires `Runtime`. Construction fails unless all of these are
explicit:

1. a validated injector compiled and wired by the application;
2. a concurrency-safe authorization callback;
3. an exact bounded boundary allowlist;
4. a fixed expiry;
5. a finite maximum evaluation budget;
6. a bounded audit sink; and
7. access to the terminal emergency disable.

Authorization and clock panics fail closed. Expired, denied, non-allowlisted,
budget-exhausted, disabled, and clock-failed attempts emit distinct audit
outcomes. Disable cannot be reversed. The package never reads an environment
variable and provides no remote endpoint, wildcard target, discovery, gossip,
or global registration mechanism.

The evaluation budget is a conservative cap on authorized engine evaluations;
a scheduled no-match still consumes one budget unit. Applications needing a
rate cap must enforce it before authorization in addition to this finite total
budget.

Context-aware application APIs pass the caller context to authorization.
Synchronous adapter methods that cannot accept a context authorize with a
background context; authorizers are therefore required to return promptly and
must not perform network or other blocking I/O.

## Safe data

Rule IDs, scopes, and panic strings accept only bounded safe identifiers.
Predicates receive a boundary and numeric operation/attempt values. Fault
errors are never copied into events. Runtime audit sanitizes invalid boundary
strings. Do not derive identifiers from request bodies, credentials, headers,
database values, tenant IDs, arbitrary errors, or other domain payloads.

## Denial-of-service bounds

Configuration caps rule count, faults per decision, latency per fault, byte
state, sequence length, activation count, allowlist size, and runtime budget.
There are no background goroutines, event queues, histories, dynamic key
registries, or per-call timers retained by the engine.

## Threat review checklist

- prove activation cannot occur through ambient configuration;
- prove authorization, allowlist, expiry, budget, and audit cannot be bypassed;
- verify wildcard or user-derived scopes are rejected;
- inspect events and fixtures for secrets;
- bound caller contexts as well as configured latency;
- verify cleanup for every after-acquisition fault; and
- exercise emergency disable under concurrent load.

See the versioned [threat model](threat-model.md) for trust boundaries,
controls, evidence, and accepted residual risks.
