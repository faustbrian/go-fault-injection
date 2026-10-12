package faultinject

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	maxIdentityLength = 64
	defaultMaxRules   = 64
	defaultMaxFaults  = 16
	defaultMaxLatency = 30 * time.Second
	defaultMaxBytes   = 64 * 1024
)

// ErrInvalidConfig identifies configuration rejected before activation.
var ErrInvalidConfig = errors.New("invalid fault-injection configuration")

// InvalidConfigError identifies a rejected field and reason and unwraps to
// ErrInvalidConfig. Constructor-produced errors omit fault payloads; exported
// fields on caller-created values are not sanitized. Error requires a non-nil
// receiver; Unwrap always returns the category, including on a nil receiver.
type InvalidConfigError struct {
	// Field names the rejected configuration dimension.
	Field string
	// Reason explains the refusal; caller-supplied text is rendered verbatim.
	Reason string
}

// Error formats Field and Reason without quoting, truncation or redaction.
func (e *InvalidConfigError) Error() string {
	return fmt.Sprintf("faultinject: invalid %s: %s", e.Field, e.Reason)
}

// Unwrap exposes ErrInvalidConfig regardless of receiver state.
func (e *InvalidConfigError) Unwrap() error { return ErrInvalidConfig }

func invalid(field, reason string) error {
	return &InvalidConfigError{Field: field, Reason: reason}
}

// Boundary identifies the adapter seam at which a rule can apply. Values are
// safe identifiers, not request or tenant data.
type Boundary string

const (
	// BoundaryFunction identifies explicitly supplied Run metadata for functions.
	BoundaryFunction Boundary = "function"
	// BoundaryHTTP selects RoundTrip establishment faults.
	BoundaryHTTP Boundary = "http"
	// BoundaryHTTPBody selects returned HTTP response-body read faults.
	BoundaryHTTPBody Boundary = "http_body"
	// BoundaryConn selects connection reads/writes distinguished by Operation.
	BoundaryConn Boundary = "conn"
	// BoundaryDial selects connection establishment by a wrapped dialer.
	BoundaryDial Boundary = "dial"
	// BoundaryListen selects wrapped listener Accept operations.
	BoundaryListen Boundary = "listen"
	// BoundaryReader selects standalone wrapped reader calls.
	BoundaryReader Boundary = "reader"
	// BoundaryWriter selects standalone wrapped writer calls.
	BoundaryWriter Boundary = "writer"
	// BoundaryClock selects wrapped sleeper or timer-creation calls.
	BoundaryClock Boundary = "clock"
	// BoundaryFilesystemOpen selects wrapped filesystem Open calls.
	BoundaryFilesystemOpen Boundary = "filesystem_open"
	// BoundaryFilesystemRead selects reads on successfully opened wrapped files.
	BoundaryFilesystemRead Boundary = "filesystem_read"
)

// Metadata is the complete caller-controlled rule input, with no payload, header
// or arbitrary error field. Callers must map operations/attempts to non-sensitive
// numeric values rather than encode tenant identifiers or other private data.
type Metadata struct {
	// Boundary selects an exact rule scope; callers must use safe identities.
	Boundary Boundary
	// Operation is the stable caller-assigned numeric seam identity.
	Operation uint32
	// Attempt is caller-supplied for Run and adapter-counted for reads/writes;
	// zero is allowed and has no implicit retry meaning.
	Attempt uint64
}

// Predicate selects calls using bounded typed metadata. Predicates must be
// deterministic and safe for concurrent invocation. They run without an
// Injector lock held. Nil means no predicate restriction; a panic means no match.
// Return promptly: selection does not impose a callback timeout or cancel it.
type Predicate func(Metadata) bool

// Activation states whether a fully configured rule participates.
type Activation uint8

const (
	// Inactive validates a rule but excludes it from eligible-call accounting.
	Inactive Activation = iota + 1
	// Active allows scope/predicate-matching calls to reach schedule accounting.
	Active
)

// Terminal controls whether lower-precedence rules may compose after a match.
type Terminal uint8

const (
	// Continue permits later rules after selection until the decision cap.
	Continue Terminal = iota + 1
	// Stop ends rule composition after this rule selects faults.
	Stop
)

// Observation controls whether a matched rule emits events.
type Observation uint8

const (
	// Suppress omits per-fault observer events without changing selection.
	Suppress Observation = iota + 1
	// Observe delivers one Event per selected fault when an Observer is configured.
	Observe
)

// Phase defines when an adapter applies a selected fault.
type Phase uint8

const (
	// PhaseBefore applies supported faults before the delegated operation.
	PhaseBefore Phase = iota + 1
	// PhaseDuring applies adapter-specific faults at the operation boundary;
	// context-aware cancellation uses an already-canceled child, not a worker.
	PhaseDuring
	// PhaseAfter applies supported faults after delegation and can replace results.
	PhaseAfter
)

// Kind identifies an injected behavior.
type Kind string

const (
	// KindError returns the retained injected error in supported phases.
	KindError Kind = "error"
	// KindLatency invokes the configured Sleeper before/during/after delegation.
	KindLatency Kind = "latency"
	// KindCancel exposes cancellation without canceling the parent context.
	KindCancel Kind = "cancel"
	// KindDeadline reports deadline expiry; during-phase delegates see cancellation.
	KindDeadline Kind = "deadline"
	// KindPanic panics with the validated attribution-safe string.
	KindPanic Kind = "panic"
	// KindDrop discards reader results after read or accepts writer bytes without IO.
	KindDrop Kind = "drop"
	// KindTruncate limits the delegated during-phase read/write prefix.
	KindTruncate Kind = "truncate"
	// KindDuplicate queues bounded reader replay or repeats a completed writer prefix.
	KindDuplicate Kind = "duplicate"
	// KindReorder reverses the bounded returned/read or supplied/write prefix.
	KindReorder Kind = "reorder"
	// KindCorrupt XORs the first byte using the configured nonzero mask.
	KindCorrupt Kind = "corrupt"
	// KindShortRead caps a delegated during-phase read without forcing EOF.
	KindShortRead Kind = "short_read"
	// KindShortWrite caps delegated input, returning io.ErrShortWrite for a short prefix.
	KindShortWrite Kind = "short_write"
	// KindTemporary reports a temporary injected NetworkError, not a timeout.
	KindTemporary Kind = "temporary_network"
	// KindPermanent reports a permanent injected NetworkError, not a timeout.
	KindPermanent Kind = "permanent_network"
	// KindReset reports ErrConnectionReset; WrapConn additionally closes its base.
	KindReset Kind = "connection_reset"
	// KindHalfClose reports ErrHalfClosed; WrapConn applies directional/full close.
	KindHalfClose Kind = "half_close"
	// KindInterrupt reports a bounded partial stream result with ErrStreamInterrupted.
	KindInterrupt Kind = "stream_interruption"
)

// Clock provides attributable event time without coupling selection to the
// process clock. Implementations are borrowed and must be concurrency-safe and
// return promptly; no engine lock is held while reading time.
type Clock interface {
	// Now returns event/expiry time. Injector event-time panics yield zero time;
	// Runtime clock panics refuse admission. The package adds no monotonicity policy.
	Now() time.Time
}

// Sleeper performs context-aware selected delays. Implementations must be
// concurrency-safe, honor cancellation and own any timers/resources they create.
// The package limits configured delays but cannot force an implementation to
// return or contain its panics.
type Sleeper interface {
	// Sleep waits for delay or cancellation and returns the resulting error.
	// Calls occur without selection locks; contextless IO adapters use Background.
	Sleep(context.Context, time.Duration) error
}

// Observer consumes already-selected fault events synchronously outside the
// selection lock. It cannot veto selection; engine invocation contains panics.
// Implementations must be concurrency-safe and return promptly. Delivery may
// interleave or occur after a later sequence, and no event queue/retry is owned.
type Observer interface {
	// Observe receives one copied Event for an observed selected fault.
	// The event has no injected error payload; blocking delays the selecting call.
	Observe(Event)
}

// ObserverFunc adapts a function to Observer.
type ObserverFunc func(Event)

// Observe calls function with event.
func (function ObserverFunc) Observe(event Event) { function(event) }

// Event is bounded attribution for one selected fault.
type Event struct {
	// RuleID is the validated bounded rule attribution identity.
	RuleID string
	// Boundary is the exact selected metadata scope, never a request payload.
	Boundary Boundary
	// Kind identifies this selected fault, not whether an adapter applies it.
	Kind Kind
	// Sequence orders evaluations within Generation, not observer arrival.
	Sequence uint64
	// Injection is this rule's activation count, shared by its composed faults.
	Injection uint64
	// SeedIdentity is the Probability seed, or zero for other schedules.
	SeedIdentity uint64
	// Generation identifies the Injector activation generation captured at selection.
	Generation uint64
	// At is sampled after selection and shared by events for that decision;
	// a panicking event clock produces zero time without undoing the decision.
	At time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

type systemSleeper struct{}

func (systemSleeper) Sleep(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
