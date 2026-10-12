package faultinject

import (
	"io"
	"net/http"
)

// NewRoundTripper returns a concurrent-safe in-process HTTP adapter. The base
// transport retains responsibility for HTTP semantics. Injected returned errors close
// request or response bodies at the same ownership boundary as transport
// failures; successful response bodies remain caller-owned. base must satisfy
// http.RoundTripper's concurrent-use, request/body and response-ownership contracts;
// its lifetime and optional transport methods are not acquired by this wrapper.
// Nil base returns ErrInvalidConfig; typed-nil delegates remain caller-invalid.
// A nil/zero Runtime returns base unchanged. With a configured Runtime, response
// bodies are wrapped at BoundaryHTTPBody even when establishment selects no fault.
// Request context governs establishment, but contextless body Read admission and
// selected latency use Background. Returned responses may have Body replaced in
// place; supply exclusively owned responses rather than sharing response objects.
// Selected panics propagate without adding post-acquisition cleanup.
func NewRoundTripper(
	base http.RoundTripper,
	runtime *Runtime,
	operation uint32,
	bodyOperation uint32,
) (http.RoundTripper, error) {
	if base == nil {
		return nil, invalid("RoundTripper", "must be non-nil")
	}
	if !runtime.active() {
		return base, nil
	}
	return &roundTripper{
		base: base, runtime: runtime, operation: operation,
		bodyOperation: bodyOperation,
	}, nil
}

type roundTripper struct {
	base          http.RoundTripper
	runtime       *Runtime
	operation     uint32
	bodyOperation uint32
}

func (transport *roundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	decision := transport.runtime.Decide(request.Context(), Metadata{Boundary: BoundaryHTTP, Operation: transport.operation})
	if !decision.Injected() {
		response, err := transport.base.RoundTrip(request)
		return transport.wrapBody(response), err
	}
	if err := faultPhaseError(request.Context(), decision.faults, PhaseBefore, transport.runtime.sleeper()); err != nil {
		closeRequestBody(request)
		return nil, err
	}
	operationContext, cleanup, duringError := prepareDuring(request.Context(), transport.runtime.sleeper(), decision.faults)
	defer cleanup()
	operationRequest := request
	if operationContext != request.Context() {
		operationRequest = request.Clone(operationContext)
	}
	response, organicError := transport.base.RoundTrip(operationRequest)
	if duringError != nil {
		closeResponseBody(response)
		return nil, duringError
	}
	if err := faultPhaseError(request.Context(), decision.faults, PhaseAfter, transport.runtime.sleeper()); err != nil {
		closeResponseBody(response)
		return nil, err
	}
	return transport.wrapBody(response), organicError
}

func (transport *roundTripper) wrapBody(response *http.Response) *http.Response {
	if response == nil || response.Body == nil {
		return response
	}
	response.Body = &injectedReadCloser{
		Reader: &injectedReader{
			reader: response.Body, runtime: transport.runtime,
			operation: transport.bodyOperation, boundary: BoundaryHTTPBody,
		},
		Closer: response.Body,
	}
	return response
}

func closeRequestBody(request *http.Request) {
	if request != nil && request.Body != nil {
		_ = request.Body.Close()
	}
}

func closeResponseBody(response *http.Response) {
	if response != nil && response.Body != nil {
		_ = response.Body.Close()
	}
}

type injectedReadCloser struct {
	io.Reader
	io.Closer
}
