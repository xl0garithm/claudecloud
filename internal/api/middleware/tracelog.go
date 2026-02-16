package middleware

import (
	"log/slog"
	"net/http"

	"go.opentelemetry.io/otel/trace"
)

// TraceLog returns middleware that injects trace_id and span_id into the
// request's slog context for log correlation.
func TraceLog(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			span := trace.SpanFromContext(r.Context())
			if span.SpanContext().IsValid() {
				traceID := span.SpanContext().TraceID().String()
				spanID := span.SpanContext().SpanID().String()
				logger := logger.With("trace_id", traceID, "span_id", spanID)
				slog.SetDefault(logger)
			}
			next.ServeHTTP(w, r)
		})
	}
}
