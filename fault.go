package faultinject

import (
	"reflect"
	"time"
)

// Fault is an immutable-after-construction fault description copied into an
// Injector. Its zero value is invalid for New; constructors defer validation to
// New. Adapters ignore kind/phase combinations they do not explicitly support.
// Config and Decision copy Fault values, not the object behind an injected error.
type Fault struct {
	// Kind identifies the behavior; configure payloads through the constructors.
	// Changing a retained caller Fault after New cannot change copied rules.
	Kind       Kind
	phase      Phase
	err        error
	delay      time.Duration
	panicValue string
	limit      int
	mask       byte
}

// ErrorFault describes err at phase; it does not apply the fault immediately.
// New rejects nil/typed-nil errors and retains the original error object. Keep
// its payload non-sensitive and safe for intended concurrent application.
func ErrorFault(phase Phase, err error) Fault {
	return Fault{Kind: KindError, phase: phase, err: err}
}

// LatencyFault describes a delay at phase. New requires a positive duration
// within Config.MaxLatency; the constructor itself does not sleep or validate.
func LatencyFault(phase Phase, delay time.Duration) Fault {
	return Fault{Kind: KindLatency, phase: phase, delay: delay}
}

// CancelFault describes cancellation at phase without canceling the parent.
// Context-aware during-phase adapters call their delegate with an already-
// canceled child and then return context.Canceled.
func CancelFault(phase Phase) Fault { return Fault{Kind: KindCancel, phase: phase} }

// DeadlineFault describes synthetic deadline expiry at phase. Context-aware
// during-phase adapters pass an already-canceled child (its Err is Canceled,
// not DeadlineExceeded), then return context.DeadlineExceeded themselves. No
// actual timer or new deadline is installed.
func DeadlineFault(phase Phase) Fault { return Fault{Kind: KindDeadline, phase: phase} }

// PanicFault describes a panic at phase, not an immediate constructor panic.
// New requires value to be a 1–64 byte ASCII letter/digit/dot/dash/underscore
// identity. Selected application panics are intentional and not recovered.
func PanicFault(phase Phase, value string) Fault {
	return Fault{Kind: KindPanic, phase: phase, panicValue: value}
}

// ByteFault describes an adapter-specific byte/stream/network fault. New requires
// positive limit within Config.MaxBytes for duplicate, truncate, reorder, corrupt,
// short-read, short-write and interruption; corruption also requires nonzero mask.
// Other supported byte/network kinds may use zero limit. Construction does not
// validate support or phase; consult docs/adapters.md for the applied semantics.
func ByteFault(kind Kind, phase Phase, limit int, mask byte) Fault {
	return Fault{Kind: kind, phase: phase, limit: limit, mask: mask}
}

// Phase returns when the fault applies.
func (f Fault) Phase() Phase { return f.phase }

// Error returns the configured injected error, if any.
func (f Fault) Error() error { return f.err }

// Delay returns the configured latency.
func (f Fault) Delay() time.Duration { return f.delay }

// PanicValue returns the bounded panic string.
func (f Fault) PanicValue() string { return f.panicValue }

// Limit returns the byte or operation bound.
func (f Fault) Limit() int { return f.limit }

// Mask returns the byte corruption mask.
func (f Fault) Mask() byte { return f.mask }

func validateFault(field string, fault Fault, maxLatency time.Duration, maxBytes int) error {
	if fault.phase < PhaseBefore || fault.phase > PhaseAfter {
		return invalid(field+".phase", "must be declared")
	}
	switch fault.Kind {
	case KindError:
		if nilError(fault.err) {
			return invalid(field+".error", "must be non-nil")
		}
	case KindLatency:
		if fault.delay < time.Nanosecond || fault.delay > maxLatency {
			return invalid(field+".delay", "must be positive and within MaxLatency")
		}
	case KindCancel, KindDeadline, KindDrop, KindTemporary,
		KindPermanent, KindReset, KindHalfClose:
	case KindPanic:
		if !validIdentity(fault.panicValue) {
			return invalid(field+".panic", "must be a bounded safe value")
		}
	case KindDuplicate, KindTruncate, KindReorder, KindShortRead, KindShortWrite, KindInterrupt:
		if fault.limit < 1 || fault.limit > maxBytes {
			return invalid(field+".limit", "must be positive and within MaxBytes")
		}
	case KindCorrupt:
		if fault.limit < 1 || fault.limit > maxBytes {
			return invalid(field+".corruption", "requires a bounded limit")
		}
		if fault.mask == 0 {
			return invalid(field+".corruption", "requires a non-zero mask and bounded limit")
		}
	default:
		return invalid(field+".kind", "is unsupported")
	}
	return nil
}

func nilError(err error) bool {
	if err == nil {
		return true
	}
	value := reflect.ValueOf(err)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
