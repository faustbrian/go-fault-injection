package resilience_test

import (
	"context"
	"errors"
	"testing"
	"time"

	faultinject "github.com/faustbrian/go-fault-injection/v2"
	resilience "github.com/faustbrian/go-resilience/v2"
	"github.com/faustbrian/go-retry/v2"
)

func TestInjectorRetryVersion2BudgetAdmissionAndRefusal(t *testing.T) {
	campaign := firstCallCampaign(t)
	budget, err := resilience.NewBudget(resilience.BudgetConfig{
		MaxResources: 1, MaxScopes: 1, MaxAdditionalPerExecution: 1,
		MaxConcurrentAdditional: 1, MaxAdditionalPerWindow: 1,
		AdditionalWindow: time.Minute, PermitTTL: time.Minute, Clock: retry.SystemClock{},
	})
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := resilience.NewMetadata("logical", "lookup", "dependency")
	if err != nil {
		t.Fatal(err)
	}
	scope, ctx, err := budget.Start(context.Background(), metadata)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := scope.Close(); err != nil {
			t.Errorf("close scope: %v", err)
		}
	})
	policy, err := retry.NewPolicyStrict(retry.Config{
		Backoff: retry.Constant(0), MaxAttempts: 3,
		Clock: retry.SystemClock{}, Sleeper: retry.SystemSleeper{},
		Classifier: retry.RetryableClassifier(), UseResilienceBudget: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	var calls, protectedCalls uint64
	result, err := retry.DoStrict(ctx, policy, func(ctx context.Context) (retry.AttemptResult[struct{}], error) {
		calls++
		attempt, ok := resilience.AttemptFromContext(ctx)
		if !ok || attempt.Ordinal != calls {
			t.Fatal("missing attempt lineage")
		}
		if calls == 1 && (attempt.Origin != resilience.OriginOriginal || attempt.ParentOrdinal != 0) {
			t.Fatal("wrong original lineage")
		}
		if calls == 2 && (attempt.Origin != resilience.OriginRetry || attempt.ParentOrdinal != 1) {
			t.Fatal("wrong retry lineage")
		}
		value, runErr := faultinject.Run(ctx, campaign.runtime,
			faultinject.Metadata{Boundary: faultinject.BoundaryFunction},
			func(context.Context) (struct{}, error) {
				protectedCalls++
				return struct{}{}, errors.New("temporary failure")
			})
		return retry.AttemptResult[struct{}]{Value: value, Outcome: retry.OutcomeKnown}, retry.Retryable(runErr)
	})
	var rejection *resilience.BudgetRejectionError
	if calls != 2 || result.Outcome != retry.OutcomeKnown || result.Retry.Attempts != 2 ||
		result.Retry.Reason != retry.ReasonWorkBudget || !errors.Is(err, resilience.ErrBudgetRejected) ||
		!errors.As(err, &rejection) || rejection.Reason != resilience.ReasonExecutionLimit {
		t.Fatalf("budget refusal: calls=%d result=%+v error=%v", calls, result, err)
	}
	if snapshot := scope.Snapshot(); snapshot.AdditionalAdmitted != 1 || snapshot.AdditionalActive != 0 {
		t.Fatalf("scope snapshot=%+v", snapshot)
	}
	if protectedCalls != 1 || campaign.injector.Snapshot().Injections != 1 ||
		campaign.runtime.Snapshot().Evaluations != 2 || campaign.authorizations.Load() != 2 ||
		campaign.auditCount.Load() != 2 || !campaign.audits[0].Injected || campaign.audits[1].Injected {
		t.Fatalf("bounded runtime: protected=%d snapshot=%+v audits=%+v", protectedCalls, campaign.runtime.Snapshot(), campaign.audits)
	}
}
