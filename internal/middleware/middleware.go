package middleware

import "net/http"

// Middleware wraps an http.HandlerFunc and returns a new one.
type Middleware func(http.HandlerFunc) http.HandlerFunc

// Chain composes multiple middlewares into a single Middleware.
// Middlewares are applied from left to right (first in list wraps outermost).
// Usage:
//
//	Chain(RecoverPanic, WithRequestID, RequestLogger, WithLocalizer)(handler)
func Chain(middlewares ...Middleware) Middleware {
	return func(final http.HandlerFunc) http.HandlerFunc {
		for i := len(middlewares) - 1; i >= 0; i-- {
			final = middlewares[i](final)
		}
		return final
	}
}
