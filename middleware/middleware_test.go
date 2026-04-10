package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestIDMiddleware_GeneratesID(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	RequestIDMiddleware(handler).ServeHTTP(rr, req)

	// Check that request ID was added to response headers
	requestID := rr.Header().Get(RequestIDHeader)
	if requestID == "" {
		t.Error("expected request ID in response headers, got empty string")
	}
}

func TestRequestIDMiddleware_PreservesExisting(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set(RequestIDHeader, "test-request-id-123")
	rr := httptest.NewRecorder()

	RequestIDMiddleware(handler).ServeHTTP(rr, req)

	// Check that existing request ID is preserved
	requestID := rr.Header().Get(RequestIDHeader)
	expectedID := "test-request-id-123"
	if requestID != expectedID {
		t.Errorf("expected request ID '%s', got '%s'", expectedID, requestID)
	}
}

func TestGetRequestID_FromContext(t *testing.T) {
	var capturedID string
	
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = GetRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set(RequestIDHeader, "test-context-id")
	rr := httptest.NewRecorder()

	RequestIDMiddleware(handler).ServeHTTP(rr, req)

	if capturedID != "test-context-id" {
		t.Errorf("expected request ID 'test-context-id' in context, got '%s'", capturedID)
	}
}

func TestRequestIDMiddleware_UniqueIDs(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ids := make(map[string]bool)
	
	// Make multiple requests and ensure unique IDs
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()
		
		RequestIDMiddleware(handler).ServeHTTP(rr, req)
		
		requestID := rr.Header().Get(RequestIDHeader)
		if ids[requestID] {
			t.Errorf("duplicate request ID generated: %s", requestID)
		}
		ids[requestID] = true
	}
}
