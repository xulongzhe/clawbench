package middleware

import (
	"context"
	"net/http"

	i18npkg "clawbench/internal/i18n"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type contextKey string

const localizerKey contextKey = "localizer"

// WithLocalizer creates a per-request Localizer and stores it in context.
func WithLocalizer(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		loc := i18npkg.Localizer(r)
		ctx := context.WithValue(r.Context(), localizerKey, loc)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// GetLocalizer retrieves the Localizer from request context.
func GetLocalizer(r *http.Request) *i18n.Localizer {
	if loc, ok := r.Context().Value(localizerKey).(*i18n.Localizer); ok {
		return loc
	}
	return i18npkg.Localizer(r) // fallback
}
