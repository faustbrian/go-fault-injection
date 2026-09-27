package faultinject

import (
	"context"
	"sync/atomic"
	"time"
)

// Authorizer approves one runtime evaluation at a bounded metadata boundary.
// Implementations must be concurrency-safe. Panics fail closed.
type Authorizer interface {
	Authorize(context.Context, Metadata) bool
}

// AuthorizerFunc adapts a function to Authorizer.
type AuthorizerFunc func(context.Context, Metadata) bool

// Authorize calls function with the bounded metadata.
func (function AuthorizerFunc) Authorize(ctx context.Context, metadata Metadata) bool {
	return function(ctx, metadata)
}

// AuditOutcome identifies the safety gate that handled an evaluation.
type AuditOutcome string

const (
	AuditEvaluated       AuditOutcome = "evaluated"
	AuditDenied          AuditOutcome = "denied"
	AuditNotAllowlisted  AuditOutcome = "not_allowlisted"
	AuditExpired         AuditOutcome = "expired"
	AuditBudgetExhausted AuditOutcome = "budget_exhausted"
	AuditDisabled        AuditOutcome = "disabled"
	AuditClockFailure    AuditOutcome = "clock_failure"
)

// AuditEvent contains only bounded typed metadata and result attribution.
type AuditEvent struct {
	Metadata   Metadata
	Outcome    AuditOutcome
	Injected   bool
	Sequence   uint64
	Generation uint64
	At         time.Time
}

// Auditor records every attempted runtime evaluation. It cannot change a
// decision, and panics are contained.
type Auditor interface {
	Audit(AuditEvent)
}

// AuditorFunc adapts a function to Auditor.
type AuditorFunc func(AuditEvent)

// Audit calls function with event.
func (function AuditorFunc) Audit(event AuditEvent) { function(event) }

// RuntimeConfig requires every production experiment safety boundary
// explicitly. No environment variable or ambient global state is consulted.
type RuntimeConfig struct {
	Injector           *Injector
	Authorizer         Authorizer
	Allowlist          []Boundary
	ExpiresAt          time.Time
	MaximumEvaluations uint64
	Clock              Clock
	Auditor            Auditor
}

// Runtime is the fail-closed application gate for explicitly wired controlled
// experiments. A nil or zero Runtime disables built-in fault application.
// Disable is terminal and concurrency-safe.
type Runtime struct {
	injector    *Injector
	authorizer  Authorizer
	allowlist   map[Boundary]struct{}
	expiresAt   time.Time
	maximum     uint64
	clock       Clock
	auditor     Auditor
	evaluations atomic.Uint64
	disabled    atomic.Bool
}

// NewRuntime validates authorization, allowlist, expiry, budget, audit, and
// emergency-disable prerequisites before use.
func NewRuntime(config RuntimeConfig) (*Runtime, error) {
	if config.Injector == nil || !config.Injector.enabled {
		return nil, invalid("Runtime.Injector", "must be explicitly constructed")
	}
	if nilInterface(config.Authorizer) {
		return nil, invalid("Runtime.Authorizer", "must be non-nil")
	}
	if len(config.Allowlist) == 0 || len(config.Allowlist) > 64 {
		return nil, invalid("Runtime.Allowlist", "must contain between 1 and 64 boundaries")
	}
	if config.ExpiresAt.IsZero() {
		return nil, invalid("Runtime.ExpiresAt", "must be declared")
	}
	if config.MaximumEvaluations == 0 || config.MaximumEvaluations > 1_000_000_000 {
		return nil, invalid("Runtime.MaximumEvaluations", "must be between 1 and 1000000000")
	}
	if nilInterface(config.Auditor) {
		return nil, invalid("Runtime.Auditor", "must be non-nil")
	}
	clock := config.Clock
	if nilInterface(clock) && clock != nil {
		return nil, invalid("Runtime.Clock", "must not be typed nil")
	}
	if clock == nil {
		clock = systemClock{}
	}
	allowlist := make(map[Boundary]struct{}, len(config.Allowlist))
	for _, boundary := range config.Allowlist {
		if !validIdentity(string(boundary)) {
			return nil, invalid("Runtime.Allowlist", "contains an invalid boundary")
		}
		if _, duplicate := allowlist[boundary]; duplicate {
			return nil, invalid("Runtime.Allowlist", "contains a duplicate boundary")
		}
		allowlist[boundary] = struct{}{}
	}
	return &Runtime{
		injector: config.Injector, authorizer: config.Authorizer,
		allowlist: allowlist, expiresAt: config.ExpiresAt,
		maximum: config.MaximumEvaluations, clock: clock, auditor: config.Auditor,
	}, nil
}

// Decide applies the fail-closed runtime safety gates before evaluating the
// underlying injector.
func (runtime *Runtime) Decide(ctx context.Context, metadata Metadata) Decision {
	now, admitted := runtime.admit(ctx, metadata)
	if !admitted {
		return Decision{}
	}
	return runtime.selectDecision(metadata, now)
}

// admit evaluates caller-controlled prerequisites without holding a selection
// lock. Final budget reservation and disable validation happen together with
// selection, or with admission of an already-selected buffered replay.
func (runtime *Runtime) admit(ctx context.Context, metadata Metadata) (time.Time, bool) {
	if runtime == nil || runtime.injector == nil {
		return time.Time{}, false
	}
	now, clockOK := runtimeNow(runtime.clock)
	if !clockOK {
		runtime.audit(metadata, AuditClockFailure, Decision{}, time.Time{})
		return time.Time{}, false
	}
	if runtime.disabled.Load() {
		runtime.audit(metadata, AuditDisabled, Decision{}, now)
		return now, false
	}
	if _, allowed := runtime.allowlist[metadata.Boundary]; !allowed {
		runtime.audit(metadata, AuditNotAllowlisted, Decision{}, now)
		return now, false
	}
	if !now.Before(runtime.expiresAt) {
		runtime.audit(metadata, AuditExpired, Decision{}, now)
		return now, false
	}
	if !safeAuthorize(runtime.authorizer, ctx, metadata) {
		runtime.audit(metadata, AuditDenied, Decision{}, now)
		return now, false
	}
	if runtime.disabled.Load() {
		runtime.audit(metadata, AuditDisabled, Decision{}, now)
		return now, false
	}
	if runtime.evaluations.Load() >= runtime.maximum {
		runtime.audit(metadata, AuditBudgetExhausted, Decision{}, now)
		return now, false
	}
	return now, true
}

func (runtime *Runtime) selectDecision(metadata Metadata, now time.Time) Decision {
	outcome := AuditEvaluated
	decision := runtime.injector.decide(metadata, func() bool {
		outcome = runtime.commitAdmission()
		return outcome == AuditEvaluated
	})
	runtime.audit(metadata, outcome, decision, now)
	return decision
}

// commitAdmission requires the injector selection lock. Disable uses that same
// lock, so no reservation or selection can cross a completed emergency stop.
// This function invokes no application collaborator.
func (runtime *Runtime) commitAdmission() AuditOutcome {
	if runtime.disabled.Load() {
		return AuditDisabled
	}
	if !runtime.reserve() {
		return AuditBudgetExhausted
	}
	return AuditEvaluated
}

func (runtime *Runtime) active() bool {
	return runtime != nil && runtime.injector != nil
}

func (runtime *Runtime) sleeper() Sleeper {
	return runtime.injector.sleeper
}

func (runtime *Runtime) reserve() bool {
	for {
		current := runtime.evaluations.Load()
		if current >= runtime.maximum {
			return false
		}
		if runtime.evaluations.CompareAndSwap(current, current+1) {
			return true
		}
	}
}

func safeAuthorize(authorizer Authorizer, ctx context.Context, metadata Metadata) (authorized bool) {
	defer func() {
		if recover() != nil {
			authorized = false
		}
	}()
	return authorizer.Authorize(ctx, metadata)
}

func (runtime *Runtime) audit(metadata Metadata, outcome AuditOutcome, decision Decision, at time.Time) {
	event := AuditEvent{
		Metadata: safeMetadata(metadata), Outcome: outcome, Injected: decision.Injected(),
		Sequence: decision.Sequence(), Generation: decision.Generation(), At: at,
	}
	defer func() { _ = recover() }()
	runtime.auditor.Audit(event)
}

func runtimeNow(clock Clock) (now time.Time, ok bool) {
	defer func() {
		if recover() != nil {
			now, ok = time.Time{}, false
		}
	}()
	return clock.Now(), true
}

func safeMetadata(metadata Metadata) Metadata {
	if !validIdentity(string(metadata.Boundary)) {
		metadata.Boundary = Boundary("invalid")
	}
	return metadata
}

// Disable permanently engages the emergency stop for this runtime gate.
func (runtime *Runtime) Disable() {
	if runtime.injector == nil {
		runtime.disabled.Store(true)
		return
	}
	runtime.injector.mu.Lock()
	runtime.disabled.Store(true)
	runtime.injector.mu.Unlock()
}

// RuntimeSnapshot is bounded safety-gate state.
type RuntimeSnapshot struct {
	Disabled    bool
	Evaluations uint64
	Remaining   uint64
	ExpiresAt   time.Time
}

// Snapshot returns current runtime safety state.
func (runtime *Runtime) Snapshot() RuntimeSnapshot {
	evaluations := runtime.evaluations.Load()
	remaining := runtime.maximum - min(evaluations, runtime.maximum)
	return RuntimeSnapshot{
		Disabled: runtime.disabled.Load(), Evaluations: evaluations,
		Remaining: remaining, ExpiresAt: runtime.expiresAt,
	}
}
