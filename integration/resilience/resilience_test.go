package resilience_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	breaker "github.com/faustbrian/go-circuit-breaker"
	faultinject "github.com/faustbrian/go-fault-injection/v2"
	"github.com/faustbrian/go-retry/v2"
)

var errCampaign = errors.New("injected campaign failure")

func TestInjectorDrivesRetryRecoveryWithoutProductionDependency(t *testing.T) {
	t.Parallel()

	campaign := firstCallCampaign(t)
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
		value, runErr := faultinject.Run(ctx, campaign.runtime,
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
	if campaign.injector.Snapshot().Injections != 1 || campaign.runtime.Snapshot().Evaluations != 2 || campaign.authorizations.Load() != 2 || campaign.auditCount.Load() != 2 || !campaign.audits[0].Injected || campaign.audits[1].Injected {
		t.Fatalf("retry runtime = %+v, authorizations=%d audits=%+v", campaign.runtime.Snapshot(), campaign.authorizations.Load(), campaign.audits)
	}
}

func TestInjectorDrivesCircuitBreakerOpeningWithoutProductionDependency(t *testing.T) {
	t.Parallel()

	campaign := firstCallCampaign(t)
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
		return faultinject.Run(ctx, campaign.runtime,
			faultinject.Metadata{Boundary: faultinject.BoundaryFunction},
			func(context.Context) (struct{}, error) {
				t.Fatal("injected before-fault called the protected operation")
				return struct{}{}, nil
			},
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
	if snapshot := campaign.injector.Snapshot(); snapshot.Injections != 1 {
		t.Fatalf("injector campaign snapshot = %+v", snapshot)
	}
	if campaign.runtime.Snapshot().Evaluations != 1 || campaign.authorizations.Load() != 1 || campaign.auditCount.Load() != 1 || !campaign.audits[0].Injected {
		t.Fatalf("breaker runtime = %+v, authorizations=%d audits=%+v", campaign.runtime.Snapshot(), campaign.authorizations.Load(), campaign.audits)
	}
}

type campaignClock struct{ now time.Time }

func (clock campaignClock) Now() time.Time { return clock.now }

type faultCampaign struct {
	injector       *faultinject.Injector
	runtime        *faultinject.Runtime
	authorizations atomic.Uint64
	auditCount     atomic.Uint64
	audits         [2]faultinject.AuditEvent
}

func firstCallCampaign(t testing.TB) *faultCampaign {
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
	campaign := &faultCampaign{injector: injector}
	clock := campaignClock{now: time.Unix(1_700_000_000, 0)}
	campaign.runtime, err = faultinject.NewRuntime(faultinject.RuntimeConfig{
		Injector: injector,
		Authorizer: faultinject.AuthorizerFunc(func(context.Context, faultinject.Metadata) bool {
			campaign.authorizations.Add(1)
			return true
		}),
		Allowlist: []faultinject.Boundary{faultinject.BoundaryFunction},
		Clock:     clock, ExpiresAt: clock.Now().Add(time.Minute),
		MaximumEvaluations: 2,
		Auditor: faultinject.AuditorFunc(func(event faultinject.AuditEvent) {
			index := campaign.auditCount.Add(1) - 1
			if index >= uint64(len(campaign.audits)) {
				t.Error("campaign exceeded its bounded audit capacity")
				return
			}
			campaign.audits[index] = event
			if event.Outcome != faultinject.AuditEvaluated || event.Metadata.Boundary != faultinject.BoundaryFunction {
				t.Errorf("campaign audit = %+v", event)
			}
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	return campaign
}
