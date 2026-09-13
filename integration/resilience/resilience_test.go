package resilience_test

import (
	"context"
	"errors"
	"testing"
	"time"

	breaker "github.com/faustbrian/go-circuit-breaker"
	faultinject "github.com/faustbrian/go-fault-injection/v2"
	"github.com/faustbrian/go-retry"
)

var errCampaign = errors.New("injected campaign failure")

func TestInjectorDrivesRetryRecoveryWithoutProductionDependency(t *testing.T) {
	t.Parallel()

	runtime, _ := firstCallRuntime(t)
	policy, err := retry.NewPolicyStrict(retry.Config{
		Backoff: retry.Constant(0), MaxAttempts: 2,
		Clock: retry.SystemClock{}, Sleeper: retry.SystemSleeper{},
		Classifier: retry.RetryableClassifier(),
	})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	result, err := retry.DoStrict(context.Background(), policy, func(ctx context.Context) (retry.AttemptResult[string], error) {
		calls++
		value, runErr := faultinject.Run(ctx, runtime,
			faultinject.Metadata{Boundary: faultinject.BoundaryFunction},
			func(context.Context) (string, error) { return "recovered", nil },
		)
		if runErr != nil {
			return retry.AttemptResult[string]{Outcome: retry.OutcomeKnown}, retry.Retryable(runErr)
		}
		return retry.AttemptResult[string]{Value: value, Outcome: retry.OutcomeKnown}, nil
	})
	if err != nil || result.Value != "recovered" || result.Outcome != retry.OutcomeKnown || calls != 2 || result.Retry.Attempts != 2 || result.Retry.Reason != retry.ReasonSucceeded {
		t.Fatalf("retry campaign = %+v, %v, calls=%d", result, err, calls)
	}
}

func TestInjectorDrivesCircuitBreakerOpeningWithoutProductionDependency(t *testing.T) {
	t.Parallel()

	runtime, injector := firstCallRuntime(t)
	circuit, err := breaker.New(breaker.Config{
		Name: "fault-campaign", Window: breaker.CountWindow{Size: 1},
		MinimumThroughput: 1,
		Opening:           &breaker.OpeningRules{FailureRatio: 1},
		OpenDuration:      breaker.FixedOpenDuration(time.Minute),
		HalfOpen:          &breaker.HalfOpenPolicy{MaxProbes: 1, RequiredSuccesses: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, firstErr := breaker.Execute(context.Background(), circuit, func(ctx context.Context) (struct{}, error) {
		return faultinject.Run(ctx, runtime,
			faultinject.Metadata{Boundary: faultinject.BoundaryFunction},
			func(context.Context) (struct{}, error) { return struct{}{}, nil },
		)
	})
	if !errors.Is(firstErr, errCampaign) {
		t.Fatalf("first breaker campaign error = %v", firstErr)
	}
	if _, secondErr := breaker.Execute(context.Background(), circuit, func(context.Context) (struct{}, error) {
		t.Fatal("open breaker called the protected operation")
		return struct{}{}, nil
	}); !errors.Is(secondErr, breaker.ErrOpen) {
		t.Fatalf("second breaker campaign error = %v", secondErr)
	}
	if snapshot := injector.Snapshot(); snapshot.Injections != 1 {
		t.Fatalf("injector campaign snapshot = %+v", snapshot)
	}
}

func firstCallRuntime(t testing.TB) (*faultinject.Runtime, *faultinject.Injector) {
	t.Helper()
	injector, err := faultinject.New(faultinject.Config{Rules: []faultinject.Rule{{
		ID: "first-call", Scope: faultinject.BoundaryFunction,
		Activation: faultinject.Active, Maximum: 1,
		Terminal: faultinject.Continue, Observation: faultinject.Suppress,
		Schedule: faultinject.Nth(1),
		Faults: []faultinject.Fault{
			faultinject.ErrorFault(faultinject.PhaseBefore, errCampaign),
		},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := faultinject.NewRuntime(faultinject.RuntimeConfig{
		Injector:           injector,
		Authorizer:         faultinject.AuthorizerFunc(func(context.Context, faultinject.Metadata) bool { return true }),
		Allowlist:          []faultinject.Boundary{faultinject.BoundaryFunction},
		ExpiresAt:          time.Now().Add(time.Minute),
		MaximumEvaluations: 2,
		Auditor:            faultinject.AuditorFunc(func(faultinject.AuditEvent) {}),
	})
	if err != nil {
		t.Fatal(err)
	}
	return runtime, injector
}
