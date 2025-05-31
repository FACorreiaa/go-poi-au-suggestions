package logger

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/middleware"
)

func StructuredLogger(logger *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Start timer
			start := time.Now()

			// Use Chi's WrapResponseWriter to capture status code
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			// Get request ID from context (ensure RequestID middleware runs before this)
			reqID := middleware.GetReqID(r.Context())

			// Prepare logger with initial fields
			requestLogger := logger.With(
				slog.String("req_id", reqID),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("remote_addr", r.RemoteAddr), // Use RealIP if behind proxy
				slog.String("user_agent", r.UserAgent()),
				slog.String("proto", r.Proto),
			)

			// Log the incoming request (optional, could log only at the end)
			requestLogger.InfoContext(r.Context(), "Request started")

			// Process request using the next HandlerImpl
			next.ServeHTTP(ww, r)

			// Log completed request
			requestLogger.InfoContext(r.Context(), "Request completed",
				slog.Int("status", ww.Status()),                          // Get status code from wrapped writer
				slog.Int("bytes_written", ww.BytesWritten()),             // Get bytes written
				slog.Duration("latency", time.Since(start)),              // Calculate latency
				slog.String("latency_human", time.Since(start).String()), // Human-readable latency
			)
		})
	}
}
