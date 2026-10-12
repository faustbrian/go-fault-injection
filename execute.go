package faultinject

import "context"

// Run selects one decision and applies its function-boundary faults around
// operation. An injected error returns the zero T value. During cancellation
// and deadline faults call operation with an already-ended child context so
// cooperative cleanup remains observable, then return the injected context
// error even if operation ignores it. During-phase latency/error failure likewise
// still invokes operation, then overrides its returned value/error. Before-phase
// failure skips operation; after-phase failure can discard an acquired result.
// Run does not close generic results or roll back effects: operation owns cleanup.
// Metadata.Boundary is used exactly as supplied, not overwritten by Run. Supply
// non-nil ctx/operation; selected and delegate panics propagate. Execution is
// synchronous with no timeout worker; caller context/cooperation bounds blocking.
func Run[T any](
	ctx context.Context,
	runtime *Runtime,
	metadata Metadata,
	operation func(context.Context) (T, error),
) (T, error) {
	if !runtime.active() {
		return operation(ctx)
	}
	decision := runtime.Decide(ctx, metadata)
	if !decision.Injected() {
		return operation(ctx)
	}

	if err := applyImmediate(ctx, runtime.sleeper(), decision.faults, PhaseBefore); err != nil {
		var zero T
		return zero, err
	}

	operationContext, cleanup, injectedDuring := prepareDuring(ctx, runtime.sleeper(), decision.faults)
	defer cleanup()
	value, operationError := operation(operationContext)
	if injectedDuring != nil {
		var zero T
		return zero, injectedDuring
	}
	if err := applyImmediate(ctx, runtime.sleeper(), decision.faults, PhaseAfter); err != nil {
		var zero T
		return zero, err
	}
	return value, operationError
}

func applyImmediate(ctx context.Context, sleeper Sleeper, faults []Fault, phase Phase) error {
	for _, fault := range faults {
		if fault.phase != phase {
			continue
		}
		switch fault.Kind {
		case KindLatency:
			if err := sleeper.Sleep(ctx, fault.delay); err != nil {
				return err
			}
		case KindError:
			return fault.err
		case KindCancel:
			return context.Canceled
		case KindDeadline:
			return context.DeadlineExceeded
		case KindPanic:
			panic(fault.panicValue)
		}
	}
	return nil
}

func prepareDuring(
	ctx context.Context,
	sleeper Sleeper,
	faults []Fault,
) (context.Context, func(), error) {
	operationContext := ctx
	cleanup := func() {}
	var injected error
	for _, fault := range faults {
		if fault.phase != PhaseDuring {
			continue
		}
		switch fault.Kind {
		case KindLatency:
			if err := sleeper.Sleep(ctx, fault.delay); err != nil {
				return ctx, cleanup, err
			}
		case KindError:
			injected = fault.err
		case KindCancel:
			var cancel context.CancelFunc
			operationContext, cancel = context.WithCancel(operationContext)
			cancel()
			injected = context.Canceled
		case KindDeadline:
			// Model an already-ended operation window without owning a timer. The
			// delegate sees Canceled; the adapter reports DeadlineExceeded afterward.
			var cancel context.CancelFunc
			operationContext, cancel = context.WithCancel(operationContext)
			cancel()
			injected = context.DeadlineExceeded
		case KindPanic:
			panic(fault.panicValue)
		}
	}
	return operationContext, cleanup, injected
}
