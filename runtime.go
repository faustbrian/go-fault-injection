package faultinject

import (
	"context"
	"sync/atomic"
	"time"
)

// Authorizer approves one runtime evaluation at a bounded metadata boundary.
// Implementations are borrowed, must be concurrency-safe and return promptly
// without blocking I/O. Panics fail closed only when invoked through Runtime;
// directly calling AuthorizerFunc uses ordinary function panic semantics.
type Authorizer interface {
	// Authorize returns whether this bounded metadata may be evaluated.
	// Honor caller context and return promptly; the Runtime adds no timeout.
	// Contextless adapters pass Background. Runtime invocation contains panics.
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
	// AuditEvaluated records a budget-reserved evaluation, including no-match or replay.
	AuditEvaluated AuditOutcome = "evaluated"
	// AuditDenied records authorization refusal or an authorizer panic.
	AuditDenied AuditOutcome = "denied"
	// AuditNotAllowlisted records a boundary absent from the exact allowlist.
	AuditNotAllowlisted AuditOutcome = "not_allowlisted"
	// AuditExpired records sampled time at or after the configured expiry.
	AuditExpired AuditOutcome = "expired"
	// AuditBudgetExhausted records refusal by the finite evaluation allowance.
	AuditBudgetExhausted AuditOutcome = "budget_exhausted"
	// AuditDisabled records refusal by this Runtime's terminal emergency stop.
	AuditDisabled AuditOutcome = "disabled"
	// AuditClockFailure records a panicking Runtime clock, with zero event time.
	AuditClockFailure AuditOutcome = "clock_failure"
)

// AuditEvent contains only bounded typed metadata and result attribution.
type AuditEvent struct {
	// Metadata is copied attribution; invalid Boundary spelling becomes invalid.
	// Numeric Operation/Attempt remain caller-supplied and must be non-sensitive.
	Metadata Metadata
	// Outcome identifies admission/evaluation, not application success.
	Outcome AuditOutcome
	// Injected reports selected fault presence, not whether the adapter applied it.
	Injected bool
	// Sequence is selected Injector attribution, zero for unselected refusals;
	// buffered replay retains the original decision sequence.
	Sequence uint64
	// Generation is selected Injector attribution; replay retains its original value.
	Generation uint64
	// At is time sampled before preliminary admission, not an operation finish time.
	At time.Time
}

// Auditor records every attempted runtime evaluation. It cannot change a
// decision, and Runtime invocation contains panics. Implementations must be
// concurrency-safe, return promptly and bound their own retained state. Delivery
// is synchronous outside selection locks, may interleave and is not retried;
// nil/zero Runtime decisions have no configured auditor and emit no event.
type Auditor interface {
	// Audit receives one copied attempt event. It may call Disable without a
	// selection lock held; blocking still delays the caller and panics do not
	// undo selection. The package cannot guarantee external persistence.
	Audit(AuditEvent)
}

// AuditorFunc adapts a function to Auditor.
type AuditorFunc func(AuditEvent)

// Audit calls function with event.
func (function AuditorFunc) Audit(event AuditEvent) { function(event) }

// RuntimeConfig requires every production experiment safety boundary
// explicitly. No environment variable or ambient global state is consulted.
type RuntimeConfig struct {
	// Injector must come from New and is borrowed, not copied or exclusively
	// owned. Other Runtime gates can share its rule counters/generations.
	Injector *Injector
	// Authorizer is mandatory and borrowed; nil and typed nil are rejected.
	Authorizer Authorizer
	// Allowlist is copied into an exact set of 1–64 unique safe boundaries.
	Allowlist []Boundary
	// ExpiresAt must be nonzero and is compared to sampled Clock time per call.
	// Construction accepts past expiry; such calls fail admission.
	ExpiresAt time.Time
	// MaximumEvaluations must be 1–1,000,000,000. No-match selection and each
	// buffered replay consume one; denied preliminary gates consume none.
	MaximumEvaluations uint64
	// Clock is borrowed; nil uses system time, typed nil refuses construction.
	// It controls sampled expiry/audit time independently of Injector event time.
	Clock Clock
	// Auditor is mandatory and borrowed; nil and typed nil are rejected. Its
	// concurrency safety, bounded retention and delivery durability are caller-owned.
	Auditor Auditor
}

// Runtime is the fail-closed application gate for explicitly wired controlled
// experiments. A nil or zero Runtime disables built-in fault application.
// Disable is terminal and concurrency-safe. Decide, Disable and Snapshot may
// run concurrently, but borrowed collaborators must also support that usage.
// No goroutine, resource close or re-enable protocol is owned. Disable and
// Snapshot require non-nil receivers; Decide alone accepts a nil Runtime.
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
// emergency-disable prerequisites before use. Invalid configuration returns nil
// and InvalidConfigError wrapping ErrInvalidConfig; callbacks are not invoked.
// Allowlist is copied, other collaborators are retained. Expiry is enforced on
// sampled per-call time, not by a timer or a construction-time future check.
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
// underlying injector. Nil/zero Runtime returns an empty decision without audit.
// Use a non-nil context; authorization owns cancellation semantics. Clock and
// authorization run before selection; their sampled result is not re-evaluated
// while callbacks block. Final budget/Disable checks share the Injector lock.
// Refusal means no experiment fault: built-in adapters still delegate organic
// work. Already-selected faults may finish even after a callback calls Disable.
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

// Disable permanently engages the emergency stop for this non-nil Runtime.
// Repeated calls are safe, including on a zero Runtime. It serializes final
// admission with selection but does not wait for callbacks, cancel delegated I/O
// or undo selected faults. Injector.Reset cannot re-enable this gate.
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
	// Disabled reports the terminal flag; zero/uninitialized Runtime initially
	// reports false even though it cannot inject.
	Disabled bool
	// Evaluations is the budget-reserved count, including no-match/replay.
	Evaluations uint64
	// Remaining is the non-negative budget remainder at the sampled count.
	Remaining uint64
	// ExpiresAt is the configured expiry, not a guarantee of active admission.
	ExpiresAt time.Time
}

// Snapshot samples a non-nil Runtime's atomic count and disabled flag separately;
// concurrent changes need not appear as one atomic state. The zero Runtime has
// zero count/budget/expiry and may be explicitly disabled.
func (runtime *Runtime) Snapshot() RuntimeSnapshot {
	evaluations := runtime.evaluations.Load()
	remaining := runtime.maximum - min(evaluations, runtime.maximum)
	return RuntimeSnapshot{
		Disabled: runtime.disabled.Load(), Evaluations: evaluations,
		Remaining: remaining, ExpiresAt: runtime.expiresAt,
	}
}
