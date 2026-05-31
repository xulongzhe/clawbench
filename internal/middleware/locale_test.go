package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/stretchr/testify/assert"
)

func TestWithLocalizer_SetsContext(t *testing.T) {
	var gotLoc *i18n.Localizer
	handler := WithLocalizer(func(w http.ResponseWriter, r *http.Request) {
		gotLoc = GetLocalizer(r)
		w.WriteHeader(http.StatusOK)
	})

	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	r.Header.Set("X-Locale", "zh")
	w := httptest.NewRecorder()
	handler(w, r)

	assert.NotNil(t, gotLoc)

	// Verify it's the Chinese localizer
	msg, err := gotLoc.Localize(&i18n.LocalizeConfig{MessageID: "FileTooLarge"})
	assert.NoError(t, err)
	assert.Equal(t, "文件过大", msg)
}

func TestWithLocalizer_DefaultEnglish(t *testing.T) {
	var gotLoc *i18n.Localizer
	handler := WithLocalizer(func(w http.ResponseWriter, r *http.Request) {
		gotLoc = GetLocalizer(r)
		w.WriteHeader(http.StatusOK)
	})

	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	w := httptest.NewRecorder()
	handler(w, r)

	assert.NotNil(t, gotLoc)

	msg, err := gotLoc.Localize(&i18n.LocalizeConfig{MessageID: "FileTooLarge"})
	assert.NoError(t, err)
	assert.Equal(t, "File too large", msg)
}

func TestGetLocalizer_FallbackWithoutMiddleware(t *testing.T) {
	// If GetLocalizer is called without WithLocalizer middleware,
	// it should fall back to creating a Localizer from the request
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	r.Header.Set("X-Locale", "zh")

	loc := GetLocalizer(r)
	assert.NotNil(t, loc)

	msg, err := loc.Localize(&i18n.LocalizeConfig{MessageID: "FileTooLarge"})
	assert.NoError(t, err)
	assert.Equal(t, "文件过大", msg)
}
