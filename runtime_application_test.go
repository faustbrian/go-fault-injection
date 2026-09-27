package faultinject_test

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	faultinject "github.com/faustbrian/go-fault-injection/v2"
)

func TestDistinctAdaptersRejectUnauthorizedFaultApplication(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		boundary faultinject.Boundary
		call     func(*testing.T, *faultinject.Runtime)
	}{
		{"writer", faultinject.BoundaryWriter, func(t *testing.T, runtime *faultinject.Runtime) {
			var base bytes.Buffer
			n, err := faultinject.WrapWriter(&base, runtime, 7).Write([]byte("organic"))
			if err != nil || n != 7 || base.String() != "organic" {
				t.Fatalf("Write = %d, %v, %q", n, err, base.String())
			}
		}},
		{"filesystem-open", faultinject.BoundaryFilesystemOpen, func(t *testing.T, runtime *faultinject.Runtime) {
			base := &trackingFS{file: &trackingFile{data: []byte("organic")}}
			wrapped, err := faultinject.WrapFS(base, runtime, 7, 8)
			if err != nil {
				t.Fatal(err)
			}
			file, err := wrapped.Open("safe")
			if err != nil || file == nil || base.file.closed {
				t.Fatalf("Open = %v, %v; closed=%t", file, err, base.file.closed)
			}
			defer func() {
				if err := file.Close(); err != nil {
					t.Error(err)
				}
			}()
			info, err := file.Stat()
			if err != nil || info.Name() != "safe.txt" {
				t.Fatalf("Stat = %v, %v", info, err)
			}
		}},
		{"listener", faultinject.BoundaryListen, func(t *testing.T, runtime *faultinject.Runtime) {
			connection := &basicConn{}
			base := &stubListener{connection: connection}
			wrapped, err := faultinject.WrapListener(base, runtime, 7)
			if err != nil {
				t.Fatal(err)
			}
			got, err := wrapped.Accept()
			if err != nil || got != connection || connection.closed || base.closed {
				t.Fatalf("Accept = %v, %v; ownership changed", got, err)
			}
			defer func() {
				if err := got.Close(); err != nil {
					t.Error(err)
				}
			}()
		}},
		{"dialer", faultinject.BoundaryDial, func(t *testing.T, runtime *faultinject.Runtime) {
			connection := &basicConn{}
			wrapped := faultinject.WrapDialer(func(context.Context, string, string) (net.Conn, error) { return connection, nil }, runtime, 7)
			got, err := wrapped(context.Background(), "tcp", "example.test")
			if err != nil || got != connection || connection.closed {
				t.Fatalf("Dial = %v, %v; closed=%t", got, err, connection.closed)
			}
			defer func() {
				if err := got.Close(); err != nil {
					t.Error(err)
				}
			}()
		}},
		{"sleeper", faultinject.BoundaryClock, func(t *testing.T, runtime *faultinject.Runtime) {
			base := &recordingSleeper{}
			wrapped, err := faultinject.WrapSleeper(base, runtime, 7)
			if err != nil {
				t.Fatal(err)
			}
			if err := wrapped.Sleep(context.Background(), time.Second); err != nil {
				t.Fatal(err)
			}
			if len(base.delays) != 1 || base.delays[0] != time.Second {
				t.Fatalf("organic sleeps = %v", base.delays)
			}
		}},
		{"timer", faultinject.BoundaryClock, func(t *testing.T, runtime *faultinject.Runtime) {
			timer := &stubTimer{channel: make(chan time.Time)}
			wrapped, err := faultinject.WrapTimerFactory(&stubTimerFactory{timer: timer}, runtime, 7)
			if err != nil {
				t.Fatal(err)
			}
			got, err := wrapped.NewTimer(context.Background(), time.Second)
			if err != nil || got != timer || timer.stopped {
				t.Fatalf("NewTimer = %v, %v; stopped=%t", got, err, timer.stopped)
			}
			defer got.Stop()
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			clock := &fixedClock{now: time.Unix(100, 0)}
			auditor := &auditRecorder{}
			runtime, err := faultinject.NewRuntime(faultinject.RuntimeConfig{
				Injector:   scopedInjector(t, test.boundary, faultinject.ErrorFault(faultinject.PhaseBefore, errInjected)),
				Authorizer: faultinject.AuthorizerFunc(func(context.Context, faultinject.Metadata) bool { return false }),
				Allowlist:  []faultinject.Boundary{test.boundary}, ExpiresAt: clock.now.Add(time.Minute),
				MaximumEvaluations: 1, Clock: clock, Auditor: auditor,
			})
			if err != nil {
				t.Fatal(err)
			}
			test.call(t, runtime)
			auditor.mu.Lock()
			defer auditor.mu.Unlock()
			if len(auditor.events) != 1 {
				t.Fatalf("audit events = %v", auditor.events)
			}
			event := auditor.events[0]
			if event.Outcome != faultinject.AuditDenied || event.Injected || event.Metadata.Boundary != test.boundary || event.Metadata.Operation != 7 || runtime.Snapshot().Evaluations != 0 {
				t.Fatalf("denied audit = %+v; budget = %+v", event, runtime.Snapshot())
			}
		})
	}
}

func TestReaderPartialDuplicateRechecksAdmissionAndPreservesRemainder(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"admitted", "denied", "budget-exhausted"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			clock := &fixedClock{now: time.Unix(100, 0)}
			var authorized atomic.Bool
			authorized.Store(true)
			maximum := uint64(3)
			if name == "budget-exhausted" {
				maximum = 2
			}
			auditor := &auditRecorder{}
			runtime, err := faultinject.NewRuntime(faultinject.RuntimeConfig{
				Injector:   scopedInjector(t, faultinject.BoundaryReader, faultinject.ByteFault(faultinject.KindDuplicate, faultinject.PhaseAfter, 4, 0)),
				Authorizer: faultinject.AuthorizerFunc(func(context.Context, faultinject.Metadata) bool { return authorized.Load() }),
				Allowlist:  []faultinject.Boundary{faultinject.BoundaryReader}, ExpiresAt: clock.now.Add(time.Minute),
				MaximumEvaluations: maximum, Clock: clock, Auditor: auditor,
			})
			if err != nil {
				t.Fatal(err)
			}
			reader := faultinject.WrapReader(bytes.NewBufferString("abcdefgh"), runtime, 7)
			buffer := make([]byte, 4)
			if n, err := reader.Read(buffer); err != nil || string(buffer[:n]) != "abcd" {
				t.Fatalf("initial read = %q, %v", buffer[:n], err)
			}
			buffer = buffer[:2]
			if n, err := reader.Read(buffer); err != nil || string(buffer[:n]) != "ab" {
				t.Fatalf("partial replay = %q, %v", buffer[:n], err)
			}
			want, outcome := "cd", faultinject.AuditEvaluated
			if name == "denied" {
				authorized.Store(false)
				want, outcome = "ef", faultinject.AuditDenied
			}
			if name == "budget-exhausted" {
				want, outcome = "ef", faultinject.AuditBudgetExhausted
			}
			if n, err := reader.Read(buffer); err != nil || string(buffer[:n]) != want {
				t.Fatalf("read after partial replay = %q, %v; want %q", buffer[:n], err, want)
			}
			authorized.Store(true)
			later := "gh"
			if name == "admitted" {
				later = "ef"
			}
			if n, err := reader.Read(buffer); err != nil || string(buffer[:n]) != later {
				t.Fatalf("organic continuation = %q, %v; want %q", buffer[:n], err, later)
			}
			auditor.mu.Lock()
			defer auditor.mu.Unlock()
			if len(auditor.events) != 4 {
				t.Fatalf("audit events = %+v", auditor.events)
			}
			first, replay, next := auditor.events[0], auditor.events[1], auditor.events[2]
			if !first.Injected || replay.Outcome != faultinject.AuditEvaluated || !replay.Injected || replay.Sequence != first.Sequence || replay.Generation != first.Generation || replay.Metadata.Attempt != 2 || next.Metadata.Attempt != 3 || next.Outcome != outcome || next.Injected != (name == "admitted") {
				t.Fatalf("partial replay audit = %+v", auditor.events)
			}
			if name == "admitted" && (next.Sequence != first.Sequence || next.Generation != first.Generation) {
				t.Fatalf("remaining replay attribution = %+v", next)
			}
			if snapshot := runtime.Snapshot(); snapshot.Evaluations != maximum || snapshot.Remaining != 0 {
				t.Fatalf("partial replay budget = %+v", snapshot)
			}
		})
	}
}

func TestReaderQueuedDuplicateRespectsRuntimeGateTransitions(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"disabled", "denied", "expired", "budget-exhausted"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			clock := &fixedClock{now: time.Unix(100, 0)}
			var authorized atomic.Bool
			authorized.Store(true)
			maximum := uint64(3)
			if name == "budget-exhausted" {
				maximum = 1
			}
			auditor := &auditRecorder{}
			runtime, err := faultinject.NewRuntime(faultinject.RuntimeConfig{
				Injector: scopedInjector(t, faultinject.BoundaryReader,
					faultinject.ByteFault(faultinject.KindDuplicate, faultinject.PhaseAfter, 4, 0)),
				Authorizer: faultinject.AuthorizerFunc(func(context.Context, faultinject.Metadata) bool {
					return authorized.Load()
				}),
				Allowlist:          []faultinject.Boundary{faultinject.BoundaryReader},
				ExpiresAt:          clock.now.Add(time.Minute),
				MaximumEvaluations: maximum,
				Clock:              clock,
				Auditor:            auditor,
			})
			if err != nil {
				t.Fatal(err)
			}
			reader := faultinject.WrapReader(bytes.NewBufferString("abcdefghijkl"), runtime, 1)
			buffer := make([]byte, 4)
			if n, err := reader.Read(buffer); err != nil || string(buffer[:n]) != "abcd" {
				t.Fatalf("first read = %q, %v", buffer[:n], err)
			}
			var outcome faultinject.AuditOutcome
			switch name {
			case "disabled":
				runtime.Disable()
				outcome = faultinject.AuditDisabled
			case "denied":
				authorized.Store(false)
				outcome = faultinject.AuditDenied
			case "expired":
				clock.now = clock.now.Add(2 * time.Minute)
				outcome = faultinject.AuditExpired
			case "budget-exhausted":
				outcome = faultinject.AuditBudgetExhausted
			}
			if n, err := reader.Read(buffer); err != nil || string(buffer[:n]) != "efgh" {
				t.Fatalf("read after %s = %q, %v; want organic efgh", name, buffer[:n], err)
			}
			if got := auditor.outcomes(); !equalOutcomes(got, []faultinject.AuditOutcome{
				faultinject.AuditEvaluated, outcome,
			}) {
				t.Fatalf("audit outcomes = %v", got)
			}
			authorized.Store(true)
			clock.now = time.Unix(100, 0)
			if n, err := reader.Read(buffer); err != nil || string(buffer[:n]) != "ijkl" {
				t.Fatalf("later read = %q, %v; want organic ijkl without stale duplicate", buffer[:n], err)
			}
		})
	}
}

func TestReaderQueuedDuplicateConsumesBudgetAndRetainsAuditAttribution(t *testing.T) {
	t.Parallel()
	clock := &fixedClock{now: time.Unix(100, 0)}
	auditor := &auditRecorder{}
	runtime, err := faultinject.NewRuntime(faultinject.RuntimeConfig{
		Injector: scopedInjector(t, faultinject.BoundaryReader,
			faultinject.ByteFault(faultinject.KindDuplicate, faultinject.PhaseAfter, 4, 0)),
		Authorizer: faultinject.AuthorizerFunc(func(context.Context, faultinject.Metadata) bool { return true }),
		Allowlist:  []faultinject.Boundary{faultinject.BoundaryReader},
		ExpiresAt:  clock.now.Add(time.Minute), MaximumEvaluations: 2,
		Clock: clock, Auditor: auditor,
	})
	if err != nil {
		t.Fatal(err)
	}
	reader := faultinject.WrapReader(bytes.NewBufferString("abcdefgh"), runtime, 7)
	buffer := make([]byte, 4)
	for call, want := range []string{"abcd", "abcd", "efgh"} {
		n, err := reader.Read(buffer)
		if err != nil || string(buffer[:n]) != want {
			t.Fatalf("read %d = %q, %v; want %q", call+1, buffer[:n], err, want)
		}
	}
	if got := runtime.Snapshot(); got.Evaluations != 2 || got.Remaining != 0 {
		t.Fatalf("runtime snapshot = %+v", got)
	}
	auditor.mu.Lock()
	defer auditor.mu.Unlock()
	if len(auditor.events) != 3 {
		t.Fatalf("audit events = %d, want 3", len(auditor.events))
	}
	first, replay, exhausted := auditor.events[0], auditor.events[1], auditor.events[2]
	if first.Outcome != faultinject.AuditEvaluated || !first.Injected ||
		replay.Outcome != faultinject.AuditEvaluated || !replay.Injected ||
		replay.Sequence != first.Sequence || replay.Generation != first.Generation ||
		replay.Metadata.Attempt != 2 || replay.Metadata.Operation != 7 ||
		exhausted.Outcome != faultinject.AuditBudgetExhausted || exhausted.Injected {
		t.Fatalf("audit events = %+v", auditor.events)
	}
}

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
