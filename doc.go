// Package faultinject provides deterministic, bounded fault injection for Go
// tests and explicitly wired controlled experiments.
//
// Construct copied rules with New, then wire NewRuntime into Run or a selected
// HTTP, I/O, network, filesystem or time adapter. A nil/zero Runtime applies no
// faults and delegates organic operations; refusal disables the experiment, not
// the application's underlying work. There is no environment activation, remote
// control plane, background worker or cgo dependency.
//
// Injector selection is ordered by lock acquisition, not goroutine launch or
// observation delivery. Reproduction requires the same eligible-call ordering
// and configuration/seed. Runtime adds authorization, allowlisting, sampled
// expiry, a finite evaluation budget, audit and terminal Disable; it is not a
// rate limiter and does not abort faults already selected before Disable.
//
// Predicates, clocks, sleepers, observers, authorizers, auditors and delegated
// resources are borrowed for the whole adapter lifetime. Callers own concurrent
// use, prompt callback return, cancellation and closing. The package's contained
// callback panics do not imply recovery of injected or delegated operation panics.
// Adapter phases and partial-result differences are described in docs/adapters.md;
// these in-process faults do not simulate packet, kernel or cluster behavior.
package faultinject
