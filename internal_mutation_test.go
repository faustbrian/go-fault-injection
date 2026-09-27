package faultinject

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"runtime"
	"testing"
	"time"
)

func TestInternalByteTransformPhaseAndEmptyGuards(t *testing.T) {
	t.Parallel()

	reader := &injectedReader{}
	data := []byte("ab")
	for _, fault := range []Fault{
		{Kind: KindCorrupt, phase: PhaseBefore, limit: 2, mask: 1},
		{Kind: KindReorder, phase: PhaseBefore, limit: 2},
		{Kind: KindDrop, phase: PhaseBefore},
		{Kind: KindDuplicate, phase: PhaseBefore, limit: 2},
		{Kind: KindInterrupt, phase: PhaseAfter, limit: 1},
	} {
		copyData := append([]byte(nil), data...)
		n, err := reader.transform(copyData, Decision{faults: []Fault{fault}})
		if n != 2 || err != nil || !bytes.Equal(copyData, data) || len(reader.duplicate) != 0 {
			t.Fatalf("wrong-phase transform = %q, %d, %v, duplicate=%q", copyData, n, err, reader.duplicate)
		}
	}
	for _, fault := range []Fault{
		{Kind: KindCorrupt, phase: PhaseAfter, limit: 1, mask: 1},
		{Kind: KindDrop, phase: PhaseAfter},
		{Kind: KindDuplicate, phase: PhaseAfter, limit: 1},
		{Kind: KindInterrupt, phase: PhaseDuring, limit: 1},
	} {
		if n, err := reader.transform([]byte{}, Decision{faults: []Fault{fault}}); n != 0 || err != nil || len(reader.duplicate) != 0 {
			t.Fatalf("empty transform = %d, %v, duplicate=%q", n, err, reader.duplicate)
		}
	}
}

func TestInternalBufferSelectionAndPhaseIteration(t *testing.T) {
	t.Parallel()

	buffer := []byte("abcd")
	if got := boundedBuffer(buffer, []Fault{{Kind: KindShortRead, phase: PhaseBefore, limit: 1}}, true); len(got) != 4 {
		t.Fatalf("before-phase reader length = %d", len(got))
	}
	if got := boundedBuffer(buffer, []Fault{{Kind: KindShortWrite, phase: PhaseDuring, limit: 1}}, true); len(got) != 4 {
		t.Fatalf("writer kind changed reader length = %d", len(got))
	}
	if got := boundedBuffer(buffer, []Fault{{Kind: KindShortRead, phase: PhaseDuring, limit: 1}}, false); len(got) != 4 {
		t.Fatalf("reader kind changed writer length = %d", len(got))
	}
	if got := boundedBuffer(buffer, []Fault{{Kind: KindDrop, phase: PhaseDuring}, {Kind: KindShortRead, phase: PhaseDuring, limit: 2}}, true); len(got) != 2 {
		t.Fatalf("later reader bound length = %d", len(got))
	}
	if got := boundedBuffer(buffer, []Fault{{Kind: KindDrop, phase: PhaseAfter}, {Kind: KindShortRead, phase: PhaseDuring, limit: 2}}, true); len(got) != 2 {
		t.Fatalf("wrong-phase fault stopped later bound: %d", len(got))
	}
	if got := boundedBuffer(buffer, []Fault{{Kind: KindDrop, phase: PhaseAfter}, {Kind: KindDuplicate, phase: PhaseAfter, limit: 3}}, true); len(got) != 3 {
		t.Fatalf("duplicate reader bound length = %d", len(got))
	}

	transformed, changed := transformWriteBuffer(buffer, []Fault{
		{Kind: KindCorrupt, phase: PhaseAfter, limit: 1, mask: 1},
		{Kind: KindDrop, phase: PhaseDuring},
	})
	if changed || !bytes.Equal(transformed, buffer) {
		t.Fatalf("wrong-phase write transform = %q, %t", transformed, changed)
	}
	transformed, changed = transformWriteBuffer(buffer, []Fault{
		{Kind: KindDrop, phase: PhaseDuring},
		{Kind: KindReorder, phase: PhaseDuring, limit: 2},
	})
	if !changed || string(transformed) != "bacd" {
		t.Fatalf("later write transform = %q, %t", transformed, changed)
	}
}

func TestInternalFaultPhaseIterationAndHTTPNilCleanup(t *testing.T) {
	t.Parallel()

	err := faultPhaseError(context.Background(), []Fault{
		{Kind: KindTemporary, phase: PhaseAfter},
		{Kind: KindPermanent, phase: PhaseBefore},
	}, PhaseBefore, systemSleeper{})
	if !errors.Is(err, ErrPermanentNetwork) {
		t.Fatalf("phase error = %v", err)
	}
	closeRequestBody(nil)
	closeRequestBody(&http.Request{})
	closeResponseBody(nil)
	closeResponseBody(&http.Response{})
}

func TestInternalDuplicateRequiresCompleteFirstWrite(t *testing.T) {
	t.Parallel()

	base := &shortNilWriter{}
	writer := &injectedWriter{
		writer:   base,
		runtime:  mustInternalRuntime(t, BoundaryWriter, Fault{Kind: KindDuplicate, phase: PhaseAfter, limit: 2}),
		boundary: BoundaryWriter,
	}
	n, err := writer.Write([]byte("ab"))
	if n != 1 || err != nil || base.calls != 1 {
		t.Fatalf("Write() = %d, %v, calls=%d", n, err, base.calls)
	}
}

type shortNilWriter struct{ calls int }

func (writer *shortNilWriter) Write([]byte) (int, error) {
	writer.calls++
	return 1, nil
}

func mustInternalInjector(t *testing.T, boundary Boundary, fault Fault) *Injector {
	t.Helper()
	injector, err := New(Config{Rules: []Rule{{
		ID: "internal", Scope: boundary, Activation: Active, Maximum: 1,
		Terminal: Continue, Observation: Suppress, Schedule: Every(1),
		Faults: []Fault{fault},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	return injector
}

func mustInternalRuntime(t *testing.T, boundary Boundary, fault Fault) *Runtime {
	t.Helper()
	runtime, err := NewRuntime(RuntimeConfig{
		Injector:           mustInternalInjector(t, boundary, fault),
		Authorizer:         AuthorizerFunc(func(context.Context, Metadata) bool { return true }),
		Allowlist:          []Boundary{boundary},
		ExpiresAt:          time.Now().Add(time.Hour),
		MaximumEvaluations: 1,
		Auditor:            AuditorFunc(func(AuditEvent) {}),
	})
	if err != nil {
		t.Fatal(err)
	}
	return runtime
}

var _ io.Writer = (*shortNilWriter)(nil)

func TestInternalZeroRuntimeDisableIsTerminal(t *testing.T) {
	gate := &Runtime{}
	gate.Disable()
	gate.Disable()
	if snapshot := gate.Snapshot(); !snapshot.Disabled || snapshot.Evaluations != 0 || snapshot.Remaining != 0 {
		t.Fatalf("disabled zero runtime = %+v", snapshot)
	}
	if gate.Decide(context.Background(), Metadata{Boundary: BoundaryReader}).Injected() {
		t.Fatal("zero runtime selected a fault")
	}
}

func TestInternalExhaustedRuntimeDoesNotEvaluatePredicates(t *testing.T) {
	gate := mustInternalRuntime(t, BoundaryReader, Fault{Kind: KindDrop, phase: PhaseAfter})
	calls := 0
	gate.injector.rules[0].rule.Predicate = func(Metadata) bool { calls++; return true }
	var outcomes []AuditOutcome
	gate.auditor = AuditorFunc(func(event AuditEvent) { outcomes = append(outcomes, event.Outcome) })
	metadata := Metadata{Boundary: BoundaryReader}
	if !gate.Decide(context.Background(), metadata).Injected() {
		t.Fatal("authorized first evaluation did not select a fault")
	}
	for range 3 {
		if gate.Decide(context.Background(), metadata).Injected() {
			t.Fatal("exhausted runtime selected a fault")
		}
	}
	if calls != 1 || gate.Snapshot().Evaluations != 1 {
		t.Fatalf("exhausted runtime evaluated caller code: predicates=%d, snapshot=%+v", calls, gate.Snapshot())
	}
	if len(outcomes) != 4 || outcomes[0] != AuditEvaluated {
		t.Fatalf("evaluation audits = %v", outcomes)
	}
	for _, outcome := range outcomes[1:] {
		if outcome != AuditBudgetExhausted {
			t.Fatalf("exhausted audit = %s", outcome)
		}
	}
}

func TestInternalReaderReplayFinalAdmissionRejects(t *testing.T) {
	for _, outcome := range []AuditOutcome{AuditDisabled, AuditBudgetExhausted} {
		t.Run(string(outcome), func(t *testing.T) {
			gate := mustInternalRuntime(t, BoundaryReader, Fault{Kind: KindDrop, phase: PhaseAfter})
			var event AuditEvent
			gate.auditor = AuditorFunc(func(value AuditEvent) { event = value })
			reader := &injectedReader{
				reader: bytes.NewBufferString("organic"), runtime: gate, boundary: BoundaryReader,
				duplicate: []byte("stale"), duplicateDecision: Decision{faults: []Fault{{Kind: KindDrop}}},
			}
			// Hold the real final-admission owner until Read has passed its
			// preliminary gates and owns the buffered-replay lock.
			gate.injector.mu.Lock()
			done := make(chan struct{})
			buffer := make([]byte, 16)
			var n int
			var err error
			go func() { n, err = reader.Read(buffer); close(done) }()
			deadline := time.Now().Add(5 * time.Second)
			for reader.duplicateMu.TryLock() {
				reader.duplicateMu.Unlock()
				if time.Now().After(deadline) {
					gate.injector.mu.Unlock()
					<-done
					t.Fatal("Read never reached final replay admission")
				}
				runtime.Gosched()
			}
			if outcome == AuditDisabled {
				gate.disabled.Store(true)
			} else if got := gate.commitAdmission(); got != AuditEvaluated {
				gate.injector.mu.Unlock()
				<-done
				t.Fatalf("competing final reservation = %s", got)
			}
			gate.injector.mu.Unlock()
			<-done
			if err != nil || string(buffer[:n]) != "organic" {
				t.Fatalf("rejected replay = %q, %v", buffer[:n], err)
			}
			if len(reader.duplicate) != 0 || reader.duplicateDecision.Injected() {
				t.Fatal("rejected replay retained bytes or fault attribution")
			}
			if event.Outcome != outcome || event.Injected || event.Sequence != 0 {
				t.Fatalf("rejected replay audit = %+v", event)
			}
			want := uint64(0)
			if outcome == AuditBudgetExhausted {
				want = 1
			}
			if gate.Snapshot().Evaluations != want {
				t.Fatal("rejected replay consumed an evaluation")
			}
		})
	}
}
