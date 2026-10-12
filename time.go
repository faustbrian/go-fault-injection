package faultinject

import (
	"context"
	"time"
)

// WrapSleeper returns a BoundaryClock adapter around a caller-owned sleeper.
// Nil base returns ErrInvalidConfig; typed-nil use is caller-invalid. Nil/zero
// Runtime returns base unchanged. Caller context/delay reach the delegate unless
// a selected fault changes the context or skips it. Borrowed sleepers must be
// concurrency-safe and cooperative; no worker or forced cancellation is added.
func WrapSleeper(base Sleeper, runtime *Runtime, operation uint32) (Sleeper, error) {
	if base == nil {
		return nil, invalid("Sleeper", "must be non-nil")
	}
	if !runtime.active() {
		return base, nil
	}
	return &injectedSleeper{base: base, runtime: runtime, operation: operation}, nil
}

type injectedSleeper struct {
	base      Sleeper
	runtime   *Runtime
	operation uint32
}

func (sleeper *injectedSleeper) Sleep(ctx context.Context, delay time.Duration) error {
	_, err := Run(ctx, sleeper.runtime, Metadata{Boundary: BoundaryClock, Operation: sleeper.operation}, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, sleeper.base.Sleep(ctx, delay)
	})
	return err
}

// Timer exposes the standard timer lifecycle without exposing a concrete
// *time.Timer. The supplied factory defines channel draining, stop/reset races
// and concurrent-use semantics; the adapter does not strengthen them.
type Timer interface {
	// C returns the factory-owned delivery channel; callers must not close it.
	C() <-chan time.Time
	// Stop stops the timer according to the underlying factory's active-state
	// contract. The creator/caller owns stopping and any required channel draining.
	Stop() bool
	// Reset changes the delay according to the factory's active-state contract;
	// its boolean/stop/drain requirements remain those of the supplied timer.
	Reset(time.Duration) bool
}

// TimerFactory creates caller-owned timers through caller context; adapters do
// not manufacture a deadline or strengthen the supplied factory's cancellation.
type TimerFactory interface {
	// NewTimer receives caller context/delay and returns its timer/error.
	// Successful ownership passes to the caller; adapters Stop an acquired timer
	// when a selected failure replaces the result, not on ordinary returned errors.
	NewTimer(context.Context, time.Duration) (Timer, error)
}

// WrapTimerFactory injects clock-boundary creation faults. A timer rejected
// after construction is stopped before the injected error is returned. Selected
// panics propagate without adding post-acquisition cleanup. Nil base
// returns ErrInvalidConfig; callers must avoid typed-nil collaborators/timers.
// Nil/zero Runtime returns base unchanged. Successful timer lifecycle remains
// caller-owned, and the wrapper imposes no worker or new delay-validity policy.
func WrapTimerFactory(base TimerFactory, runtime *Runtime, operation uint32) (TimerFactory, error) {
	if base == nil {
		return nil, invalid("TimerFactory", "must be non-nil")
	}
	if !runtime.active() {
		return base, nil
	}
	return &injectedTimerFactory{base: base, runtime: runtime, operation: operation}, nil
}

type injectedTimerFactory struct {
	base      TimerFactory
	runtime   *Runtime
	operation uint32
}

func (factory *injectedTimerFactory) NewTimer(ctx context.Context, delay time.Duration) (Timer, error) {
	decision := factory.runtime.Decide(ctx, Metadata{Boundary: BoundaryClock, Operation: factory.operation})
	if err := faultPhaseError(ctx, decision.faults, PhaseBefore, factory.runtime.sleeper()); err != nil {
		return nil, err
	}
	operationContext, cleanup, duringError := prepareDuring(ctx, factory.runtime.sleeper(), decision.faults)
	defer cleanup()
	timer, organicError := factory.base.NewTimer(operationContext, delay)
	if duringError != nil {
		stopTimer(timer)
		return nil, duringError
	}
	if err := faultPhaseError(ctx, decision.faults, PhaseAfter, factory.runtime.sleeper()); err != nil {
		stopTimer(timer)
		return nil, err
	}
	return timer, organicError
}

func stopTimer(timer Timer) {
	if timer != nil {
		timer.Stop()
	}
}
