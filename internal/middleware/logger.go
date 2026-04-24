package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// ResponseWriter wraps http.ResponseWriter to capture status code and bytes written.
type ResponseWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (rw *ResponseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *ResponseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytes += n
	return n, err
}

// RequestLogger logs method, path, status, duration, client IP, trace_id via slog.
func RequestLogger(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &ResponseWriter{ResponseWriter: w, status: http.StatusOK}
		reqID := GetRequestID(r.Context())
		next.ServeHTTP(rw, r)
		duration := time.Since(start)
		slog.Info("http request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rw.status),
			slog.Duration("duration", duration),
			slog.String("ip", r.RemoteAddr),
			slog.String("trace_id", reqID),
		)
	}
}
