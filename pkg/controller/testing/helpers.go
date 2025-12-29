package testing

import (
	"net/http"

	"github.com/kylegalloway/forge/pkg/telemetry"
)

// MustNewMetrics creates a new Metrics instance or panics on error.
// This is a shared test helper to avoid duplication across test files.
func MustNewMetrics() *telemetry.Metrics {
	metrics, err := telemetry.NewMetrics()
	if err != nil {
		panic(err)
	}
	return metrics
}

// FakeResponseWriter is a test implementation of http.ResponseWriter
// for testing HTTP handlers without a real HTTP server.
type FakeResponseWriter struct {
	status int
	body   []byte
}

// Header returns an empty header map
func (f *FakeResponseWriter) Header() http.Header {
	return http.Header{}
}

// Write appends data to the response body
func (f *FakeResponseWriter) Write(data []byte) (int, error) {
	f.body = append(f.body, data...)
	return len(data), nil
}

// WriteHeader sets the HTTP status code
func (f *FakeResponseWriter) WriteHeader(statusCode int) {
	f.status = statusCode
}

// Status returns the HTTP status code that was written
func (f *FakeResponseWriter) Status() int {
	return f.status
}

// Body returns the response body that was written
func (f *FakeResponseWriter) Body() []byte {
	return f.body
}
