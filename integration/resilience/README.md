# Resilience campaign integration

This non-releasable module proves that `fault-injection` can drive deterministic
campaigns through the public retry and circuit-breaker contracts. The resilience
modules do not import fault-injection in production; the dependency direction
exists only in this integration module.

The campaigns consume published `/v2` v2.0.0 through explicit Runtime
authorization, function-boundary allowlisting, fixed-clock expiry, a two-call
evaluation budget, and bounded audit attribution. Run with `GOWORK=off` to
exercise the published artifact rather than substitute the local root module.

The retry campaign uses Retry v2.1's strict execution contract, injects one
retryable first-call failure, and then proves recovery. Its in-process
operation reports `OutcomeKnown` because each returned value or injected error
conclusively describes the completed call. The circuit-breaker campaign injects
one failure, proves the breaker opens, and proves rejection prevents a second
protected call.

The attached Resilience v2 scope admits one additional retry with original/retry
lineage, then refuses further dispatch. Completed attempts release their permits;
only the two admitted calls reach Runtime authorization and audit.
Resilience v1 remains an indirect dependency of Retry's intentionally retained
legacy-budget support; these tests do not attach a legacy scope.

Run from this module with `GOWORK=off`:

```sh
go test ./...
```
