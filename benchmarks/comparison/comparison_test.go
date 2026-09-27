package comparison_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/failsafe-go/failsafe-go"
	faultinject "github.com/faustbrian/go-fault-injection/v2"
	"github.com/slok/goresilience/chaos"
)

var errComparison = errors.New("comparison failure")

func BenchmarkEquivalentFailureOutcome(b *testing.B) {
	b.Run("fault-injection", benchmarkFaultInjection)
	b.Run("goresilience", benchmarkGoresilience)
	b.Run("failsafe-caller-failure", benchmarkFailsafeCallerFailure)
	b.Run("direct-double", benchmarkDirectDouble)
}

func TestEquivalentFailureOutcomes(t *testing.T) {
	t.Parallel()

	for _, candidate := range []struct {
		name string
		run  func() error
	}{
		{name: "fault-injection", run: newFaultInjectionFailure(t)},
		{name: "goresilience", run: newGoresilienceFailure(t)},
		{name: "failsafe-caller-failure", run: failsafeCallerFailure},
		{name: "direct-double", run: directDouble},
	} {
		t.Run(candidate.name, func(t *testing.T) {
			if err := candidate.run(); err == nil {
				t.Fatal("failure outcome was not returned")
			}
		})
	}
}

func benchmarkFaultInjection(b *testing.B) {
	run := newFaultInjectionFailure(b)
	benchmarkFailure(b, run)
}

func benchmarkGoresilience(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		run := newGoresilienceFailure(b)
		if err := run(); err == nil {
			b.Fatal("failure outcome was not returned")
		}
	}
}

func benchmarkFailsafeCallerFailure(b *testing.B) {
	benchmarkFailure(b, failsafeCallerFailure)
}

func benchmarkDirectDouble(b *testing.B) {
	benchmarkFailure(b, directDouble)
}

func benchmarkFailure(b *testing.B, run func() error) {
	b.Helper()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := run(); err == nil {
			b.Fatal("failure outcome was not returned")
		}
	}
}

func newFaultInjectionFailure(t testing.TB) func() error {
	t.Helper()
	injector, err := faultinject.New(faultinject.Config{Rules: []faultinject.Rule{{
		ID: "always", Scope: faultinject.BoundaryFunction,
		Activation: faultinject.Active, Maximum: 1_000_000_000,
		Terminal: faultinject.Continue, Observation: faultinject.Suppress,
		Schedule: faultinject.Every(1),
		Faults: []faultinject.Fault{
			faultinject.ErrorFault(faultinject.PhaseBefore, errComparison),
		},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	clock := campaignClock{now: time.Unix(1_700_000_000, 0)}
	var authorizations, audits atomic.Uint64
	runtime, err := faultinject.NewRuntime(faultinject.RuntimeConfig{
		Injector: injector,
		Authorizer: faultinject.AuthorizerFunc(func(context.Context, faultinject.Metadata) bool {
			authorizations.Add(1)
			return true
		}),
		Allowlist: []faultinject.Boundary{faultinject.BoundaryFunction},
		Clock:     clock, ExpiresAt: clock.Now().Add(time.Minute),
		MaximumEvaluations: 1_000_000_000,
		Auditor: faultinject.AuditorFunc(func(event faultinject.AuditEvent) {
			audits.Add(1)
			if event.Outcome != faultinject.AuditEvaluated || !event.Injected || event.Metadata.Boundary != faultinject.BoundaryFunction {
				t.Errorf("comparison audit = %+v", event)
			}
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	return func() error {
		before := runtime.Snapshot().Evaluations
		_, err := faultinject.Run(context.Background(), runtime,
			faultinject.Metadata{Boundary: faultinject.BoundaryFunction},
			func(context.Context) (struct{}, error) {
				t.Fatal("injected before-fault called the operation")
				return struct{}{}, nil
			},
		)
		if !errors.Is(err, errComparison) || runtime.Snapshot().Evaluations != before+1 || authorizations.Load() != before+1 || audits.Load() != before+1 {
			t.Fatalf("comparison runtime: err=%v evaluations=%d authorizations=%d audits=%d", err, runtime.Snapshot().Evaluations, authorizations.Load(), audits.Load())
		}
		return err
	}
}

type campaignClock struct{ now time.Time }

func (clock campaignClock) Now() time.Time { return clock.now }

func newGoresilienceFailure(t testing.TB) func() error {
	t.Helper()
	injector := &chaos.Injector{}
	if err := injector.SetErrorPercent(100); err != nil {
		t.Fatal(err)
	}
	runner := chaos.New(chaos.Config{Injector: injector})
	return func() error {
		return runner.Run(context.Background(), func(context.Context) error {
			t.Fatal("goresilience 100-percent failure called the operation")
			return nil
		})
	}
}

func failsafeCallerFailure() error {
	_, err := failsafe.With[struct{}]().Get(func() (struct{}, error) {
		return struct{}{}, errComparison
	})
	return err
}

func directDouble() error { return errComparison }
