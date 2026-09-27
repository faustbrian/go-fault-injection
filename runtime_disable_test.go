package faultinject_test

import (
	"bytes"
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	faultinject "github.com/faustbrian/go-fault-injection/v2"
)

func TestRuntimeDisableRejectsUnselectedEvaluations(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ surface, pause string }{
		{"decision", "authorizer"}, {"run", "authorizer"},
		{"reader", "authorizer"}, {"replay", "authorizer"},
		{"decision", "predicate"}, {"reader", "predicate"},
	} {
		t.Run(test.surface+"/"+test.pause, func(t *testing.T) {
			t.Parallel()
			entered, release := make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			t.Cleanup(unblock)
			pause := func() { close(entered); <-release }
			clock := &fixedClock{now: time.Unix(100, 0)}
			boundary := faultinject.BoundaryFunction
			if test.surface == "reader" || test.surface == "replay" {
				boundary = faultinject.BoundaryReader
			}
			rule := validRule("disable")
			rule.Scope = boundary
			if test.surface == "replay" {
				rule.Faults = []faultinject.Fault{faultinject.ByteFault(faultinject.KindDuplicate, faultinject.PhaseAfter, 4, 0)}
			}
			if test.pause == "predicate" {
				rule.Predicate = func(faultinject.Metadata) bool { pause(); return true }
			}
			injector := injectorWithConfig(t, faultinject.Config{Rules: []faultinject.Rule{rule}})
			auditor := &auditRecorder{}
			var authorizations atomic.Uint64
			pauseAt := uint64(1)
			if test.surface == "replay" {
				pauseAt = 2
			}
			gate, err := faultinject.NewRuntime(faultinject.RuntimeConfig{
				Injector: injector,
				Authorizer: faultinject.AuthorizerFunc(func(context.Context, faultinject.Metadata) bool {
					if authorizations.Add(1) == pauseAt && test.pause == "authorizer" {
						pause()
					}
					return true
				}),
				Allowlist: []faultinject.Boundary{boundary}, ExpiresAt: clock.now.Add(time.Minute),
				MaximumEvaluations: 3, Clock: clock, Auditor: auditor,
			})
			if err != nil {
				t.Fatal(err)
			}
			reader := faultinject.WrapReader(bytes.NewBufferString("abcdefghijkl"), gate, 7)
			initial := uint64(0)
			if test.surface == "replay" {
				buffer := make([]byte, 4)
				n, err := reader.Read(buffer)
				if err != nil || string(buffer[:n]) != "abcd" {
					t.Fatalf("initial Read = %q, %v", buffer[:n], err)
				}
				initial = 1
			}
			type result struct {
				value    string
				err      error
				injected bool
			}
			done := make(chan result, 1)
			go func() {
				switch test.surface {
				case "decision":
					decision := gate.Decide(context.Background(), faultinject.Metadata{Boundary: boundary})
					done <- result{injected: decision.Injected()}
				case "run":
					value, err := faultinject.Run(context.Background(), gate, faultinject.Metadata{Boundary: boundary},
						func(context.Context) (string, error) { return "organic", nil })
					done <- result{value: value, err: err}
				default:
					buffer := make([]byte, 4)
					n, err := reader.Read(buffer)
					done <- result{value: string(buffer[:n]), err: err}
				}
			}()
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("evaluation did not enter callback")
			}
			disabled := make(chan struct{})
			go func() { gate.Disable(); close(disabled) }()
			select {
			case <-disabled:
			case <-time.After(5 * time.Second):
				t.Fatal("Disable waited for an unselected collaborator")
			}
			unblock()
			var got result
			select {
			case got = <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("evaluation did not finish after callback release")
			}
			want := map[string]string{"decision": "", "run": "organic", "reader": "abcd", "replay": "efgh"}[test.surface]
			if got.injected || got.err != nil || got.value != want {
				t.Fatalf("after Disable: value=%q error=%v injected=%t; want organic %q", got.value, got.err, got.injected, want)
			}
			if snapshot := gate.Snapshot(); !snapshot.Disabled || snapshot.Evaluations != initial {
				t.Fatalf("runtime snapshot = %+v; want %d evaluations", snapshot, initial)
			}
			if snapshot := injector.Snapshot(); snapshot.Evaluations != initial || snapshot.Injections != initial {
				t.Fatalf("injector snapshot = %+v; want %d pre-disable selections", snapshot, initial)
			}
			wantAudits := []faultinject.AuditOutcome{faultinject.AuditDisabled}
			if initial != 0 {
				wantAudits = append([]faultinject.AuditOutcome{faultinject.AuditEvaluated}, wantAudits...)
			}
			if got := auditor.outcomes(); !equalOutcomes(got, wantAudits) {
				t.Fatalf("audit outcomes = %v; want %v", got, wantAudits)
			}
		})
	}
}

func TestRuntimeCallbacksCanDisableWithoutDeadlock(t *testing.T) {
	t.Parallel()
	for _, callback := range []string{"authorizer", "predicate", "observer", "auditor"} {
		t.Run(callback, func(t *testing.T) {
			t.Parallel()
			var gate *faultinject.Runtime
			rule := validRule("callback-disable")
			rule.Observation = faultinject.Observe
			rule.Predicate = func(faultinject.Metadata) bool {
				if callback == "predicate" {
					gate.Disable()
				}
				return true
			}
			injector := injectorWithConfig(t, faultinject.Config{
				Rules: []faultinject.Rule{rule},
				Observer: faultinject.ObserverFunc(func(faultinject.Event) {
					if callback == "observer" {
						gate.Disable()
					}
				}),
			})
			clock := &fixedClock{now: time.Unix(100, 0)}
			auditor := &auditRecorder{}
			var err error
			gate, err = faultinject.NewRuntime(faultinject.RuntimeConfig{
				Injector: injector,
				Authorizer: faultinject.AuthorizerFunc(func(context.Context, faultinject.Metadata) bool {
					if callback == "authorizer" {
						gate.Disable()
					}
					return true
				}),
				Allowlist: []faultinject.Boundary{faultinject.BoundaryFunction},
				ExpiresAt: clock.now.Add(time.Minute), MaximumEvaluations: 2, Clock: clock,
				Auditor: faultinject.AuditorFunc(func(event faultinject.AuditEvent) {
					if callback == "auditor" {
						gate.Disable()
					}
					auditor.Audit(event)
				}),
			})
			if err != nil {
				t.Fatal(err)
			}
			done := make(chan bool, 1)
			go func() {
				done <- gate.Decide(context.Background(), faultinject.Metadata{Boundary: faultinject.BoundaryFunction}).Injected()
			}()
			selectedBeforeDisable := callback == "observer" || callback == "auditor"
			select {
			case injected := <-done:
				if injected != selectedBeforeDisable {
					t.Fatalf("Injected = %t; selected before Disable = %t", injected, selectedBeforeDisable)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("callback Disable deadlocked")
			}
			if snapshot := gate.Snapshot(); !snapshot.Disabled {
				t.Fatalf("callback did not disable runtime: %+v", snapshot)
			}
			if gate.Decide(context.Background(), faultinject.Metadata{Boundary: faultinject.BoundaryFunction}).Injected() {
				t.Fatal("later decision injected after callback Disable")
			}
			outcome := faultinject.AuditDisabled
			if selectedBeforeDisable {
				outcome = faultinject.AuditEvaluated
			}
			if got := auditor.outcomes(); !equalOutcomes(got, []faultinject.AuditOutcome{outcome, faultinject.AuditDisabled}) {
				t.Fatalf("audit outcomes = %v", got)
			}
		})
	}
}
