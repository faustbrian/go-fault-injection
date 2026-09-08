# Resilience campaign integration

This non-releasable module proves that `fault-injection` can drive deterministic
campaigns through the public retry and circuit-breaker contracts. The resilience
modules do not import fault-injection in production; the dependency direction
exists only in this integration module.

The retry campaign uses Retry v1.1's strict execution contract, injects one
retryable first-call failure, and then proves recovery. Its in-process
operation reports `OutcomeKnown` because each returned value or injected error
conclusively describes the completed call. The circuit-breaker campaign injects
one failure, proves the breaker opens, and proves rejection prevents a second
protected call.

Run with:

```sh
go test ./...
```
