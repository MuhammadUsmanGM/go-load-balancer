package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	// RequestIDHeader is the header key for request IDs.
	RequestIDHeader = "X-Request-ID"

	// RequestIDContextKey is the context key for request IDs.
	RequestIDContextKey = "request-id"
)

// RequestIDMiddleware adds a unique request ID to every request.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Use existing request ID if present, otherwise generate new one
		requestID := r.Header.Get(RequestIDHeader)
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Add request ID to response headers
		w.Header().Set(RequestIDHeader, requestID)

		// Store in context
		ctx := context.WithValue(r.Context(), RequestIDContextKey, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID extracts the request ID from context.
func GetRequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value(RequestIDContextKey).(string); ok {
		return requestID
	}
	return ""
}

// TracingMiddleware creates OpenTelemetry spans for HTTP requests.
func TracingMiddleware(serviceName string) func(http.Handler) http.Handler {
	tracer := otel.Tracer(serviceName)
	propagator := otel.GetTextMapPropagator()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract trace context from incoming request
			ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

			// Create span
			spanName := r.Method + " " + r.URL.Path
			opts := []trace.SpanStartOption{
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					semconv.HTTPMethodKey.String(r.Method),
					semconv.HTTPURLKey.String(r.URL.String()),
					semconv.HTTPSchemeKey.String(r.URL.Scheme),
					semconv.HTTPRouteKey.String(r.URL.Path),
				),
			}

			ctx, span := tracer.Start(ctx, spanName, opts...)
			defer span.End()

			// Propagate trace context to downstream request
			propagator.Inject(ctx, propagation.HeaderCarrier(w.Header()))

			// Add request ID to span if available
			if requestID := GetRequestID(ctx); requestID != "" {
				span.SetAttributes(attribute.String("request.id", requestID))
			}

			// Wrap response writer to capture status code
			rw := &statusWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// Call next handler
			next.ServeHTTP(rw, r.WithContext(ctx))

			// Set span attributes with response info
			span.SetAttributes(
				semconv.HTTPStatusCodeKey.Int(rw.statusCode),
			)

			// Set span status based on HTTP status code
			if rw.statusCode >= 500 {
				span.SetStatus(1, "Server Error")
			}
		})
	}
}

// statusWriter wraps http.ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	statusCode int
}

func (sw *statusWriter) WriteHeader(code int) {
	sw.statusCode = code
	sw.ResponseWriter.WriteHeader(code)
}

func (sw *statusWriter) Write(b []byte) (int, error) {
	if sw.statusCode == 0 {
		sw.statusCode = http.StatusOK
	}
	return sw.ResponseWriter.Write(b)
}

// ProxyTracingMiddleware creates spans for proxied requests to backends.
func ProxyTracingMiddleware(backendURL string) func(http.Handler) http.Handler {
	tracer := otel.Tracer("proxy")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, span := tracer.Start(r.Context(), "proxy.request",
				trace.WithSpanKind(trace.SpanKindClient),
				trace.WithAttributes(
					semconv.HTTPURL(backendURL+r.URL.Path),
					semconv.HTTPMethod(r.Method),
					semconv.NetPeerName(backendURL),
				),
			)
			defer span.End()

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
