package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type requestIDKey struct{}

func generateRequestID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey{}).(string); ok {
		return id
	}
	return ""
}

// WithRequestID adds X-Request-ID header to every response and stores ID in r.Context().
func WithRequestID(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := generateRequestID()
		ctx := context.WithValue(r.Context(), requestIDKey{}, id)
		r = r.WithContext(ctx)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r)
	}
}
