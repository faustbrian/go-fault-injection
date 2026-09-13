package faultinject_test

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	faultinject "github.com/faustbrian/go-fault-injection/v2"
)

func TestRunAppliesFaultsOnlyThroughRuntimeSafetyGates(t *testing.T) {
	t.Parallel()

	clock := &fixedClock{now: time.Unix(100, 0)}
	var authorized atomic.Bool
	auditor := &auditRecorder{}
	runtime, err := faultinject.NewRuntime(faultinject.RuntimeConfig{
		Injector: scopedInjector(t, faultinject.BoundaryFunction,
			faultinject.ErrorFault(faultinject.PhaseBefore, errInjected)),
		Authorizer: faultinject.AuthorizerFunc(func(context.Context, faultinject.Metadata) bool {
			return authorized.Load()
		}),
		Allowlist:          []faultinject.Boundary{faultinject.BoundaryFunction},
		ExpiresAt:          clock.now.Add(time.Minute),
		MaximumEvaluations: 1,
		Clock:              clock,
		Auditor:            auditor,
	})
	if err != nil {
		t.Fatal(err)
	}

	operation := func(context.Context) (string, error) { return "organic", nil }
	metadata := faultinject.Metadata{Boundary: faultinject.BoundaryFunction}
	if value, err := faultinject.Run(context.Background(), runtime, metadata, operation); err != nil || value != "organic" {
		t.Fatalf("denied Run() = %q, %v", value, err)
	}
	authorized.Store(true)
	if _, err := faultinject.Run(context.Background(), runtime, metadata, operation); !errors.Is(err, errInjected) {
		t.Fatalf("authorized Run() error = %v", err)
	}
	if value, err := faultinject.Run(context.Background(), runtime, metadata, operation); err != nil || value != "organic" {
		t.Fatalf("budget-exhausted Run() = %q, %v", value, err)
	}
	clock.now = clock.now.Add(2 * time.Minute)
	if value, err := faultinject.Run(context.Background(), runtime, metadata, operation); err != nil || value != "organic" {
		t.Fatalf("expired Run() = %q, %v", value, err)
	}
	runtime.Disable()
	if value, err := faultinject.Run(context.Background(), runtime, metadata, operation); err != nil || value != "organic" {
		t.Fatalf("disabled Run() = %q, %v", value, err)
	}

	if got := auditor.outcomes(); !equalOutcomes(got, []faultinject.AuditOutcome{
		faultinject.AuditDenied,
		faultinject.AuditEvaluated,
		faultinject.AuditBudgetExhausted,
		faultinject.AuditExpired,
		faultinject.AuditDisabled,
	}) {
		t.Fatalf("audit outcomes = %v", got)
	}
}

func TestHTTPAdapterAppliesFaultsOnlyThroughRuntimeSafetyGates(t *testing.T) {
	t.Parallel()

	clock := &fixedClock{now: time.Unix(100, 0)}
	auditor := &auditRecorder{}
	runtime, err := faultinject.NewRuntime(faultinject.RuntimeConfig{
		Injector: scopedInjector(t, faultinject.BoundaryHTTP,
			faultinject.ErrorFault(faultinject.PhaseBefore, errInjected)),
		Authorizer:         faultinject.AuthorizerFunc(func(context.Context, faultinject.Metadata) bool { return false }),
		Allowlist:          []faultinject.Boundary{faultinject.BoundaryHTTP},
		ExpiresAt:          clock.now.Add(time.Minute),
		MaximumEvaluations: 1,
		Clock:              clock,
		Auditor:            auditor,
	})
	if err != nil {
		t.Fatal(err)
	}

	called := false
	transport, err := faultinject.NewRoundTripper(roundTripFunc(func(*http.Request) (*http.Response, error) {
		called = true
		return &http.Response{StatusCode: http.StatusNoContent}, nil
	}), runtime, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.test", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := transport.RoundTrip(request)
	if err != nil || response == nil || !called {
		t.Fatalf("denied RoundTrip() = %v, %v; called=%t", response, err, called)
	}
	if got := auditor.outcomes(); !equalOutcomes(got, []faultinject.AuditOutcome{faultinject.AuditDenied}) {
		t.Fatalf("audit outcomes = %v", got)
	}
}
