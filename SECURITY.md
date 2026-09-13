# Security policy

## Supported versions

The latest stable v1 release receives security fixes. Older releases and the
`main` branch are unsupported; upgrade before reporting unless the issue is a
regression under active development.

| Version | Supported |
| --- | --- |
| Latest stable v1 release | Yes |
| Older releases | No |
| `main` | No |

## Reporting a vulnerability

Do not disclose a suspected vulnerability in a public issue. Use the
[private vulnerability reporting form](https://github.com/faustbrian/go-fault-injection/security/advisories/new).

Do not include credentials, request bodies, raw headers, database values,
tenant identifiers, or production experiment tokens in a public report,
fixture, event, log, or initial contact request.

## Activation boundary

The zero runtime is disabled. The package does not read environment variables,
configuration files, flags, network endpoints, or global registries. Tests
construct an `Injector` and `Runtime` explicitly. Every built-in fault
application API accepts only the fail-closed runtime gate described in
[docs/safety.md](docs/safety.md); direct injector wiring cannot apply faults
through package adapters. See the versioned
[threat model](docs/threat-model.md) for security boundaries and residual risk.

## Supported reports

Security reports should identify unauthorized activation, stale authorization,
expiry bypass, budget bypass, allowlist widening, unsafe event data, unbounded
latency or allocation, resource leaks, adapter ownership violations,
nondeterministic selection, or a way to reactivate a disabled runtime gate.
