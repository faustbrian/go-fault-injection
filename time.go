package faultinject

import (
	"context"
	"time"
)

// WrapSleeper returns a clock-boundary adapter around a caller-owned sleeper.
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
// *time.Timer.
type Timer interface {
	C() <-chan time.Time
	Stop() bool
	Reset(time.Duration) bool
}

// TimerFactory creates caller-owned timers through a bounded context.
type TimerFactory interface {
	NewTimer(context.Context, time.Duration) (Timer, error)
}

// WrapTimerFactory injects clock-boundary creation faults. A timer rejected
// after construction is stopped before the injected error is returned.
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
