package handler

import (
	"net/http"
	"testing"

	"clawbench/internal/model"

	"github.com/stretchr/testify/assert"
)

func TestServeAuthCheck(t *testing.T) {
	t.Run("NoPasswordSet_Returns200", func(t *testing.T) {
		env, teardown := setupTestEnv(t)
		defer teardown()

		// No password set
		model.SessionToken = ""

		req := newRequest(t, http.MethodGet, "/api/auth/check", nil)
		w := callHandler(ServeAuthCheck, req)

		assert.Equal(t, http.StatusOK, w.Code)
		_ = env
	})

	t.Run("PasswordSet_ValidCookie_Returns200", func(t *testing.T) {
		_, teardown := setupTestEnv(t)
		defer teardown()

		model.SessionToken = hashPassword("testpass")

		req := newRequest(t, http.MethodGet, "/api/auth/check", nil)
		withAuthCookie(req, model.SessionToken)

		w := callHandler(ServeAuthCheck, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("PasswordSet_NoCookie_Returns401", func(t *testing.T) {
		_, teardown := setupTestEnv(t)
		defer teardown()

		model.SessionToken = hashPassword("testpass")

		req := newRequest(t, http.MethodGet, "/api/auth/check", nil)
		// No cookie added

		w := callHandler(ServeAuthCheck, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("PasswordSet_WrongCookie_Returns401", func(t *testing.T) {
		_, teardown := setupTestEnv(t)
		defer teardown()

		model.SessionToken = hashPassword("testpass")

		req := newRequest(t, http.MethodGet, "/api/auth/check", nil)
		withAuthCookie(req, "wrong-token-value")

		w := callHandler(ServeAuthCheck, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestServeLogin(t *testing.T) {
	t.Run("GET_DoesNotCrash", func(t *testing.T) {
		_, teardown := setupTestEnv(t)
		defer teardown()

		req := newRequest(t, http.MethodGet, "/login", nil)
		w := callHandler(ServeLogin, req)

		// May be 200 (if public/index.html exists) or 404 — just verify no panic
		assert.Contains(t, []int{http.StatusOK, http.StatusNotFound}, w.Code)
	})

	t.Run("POST_CorrectPassword_Returns200WithCookie", func(t *testing.T) {
		_, teardown := setupTestEnv(t)
		defer teardown()

		model.SessionToken = hashPassword("testpass")

		req := newRequest(t, http.MethodPost, "/login", map[string]string{
			"password": "testpass",
		})
		w := callHandler(ServeLogin, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assertJSONField(t, w, "ok", true)

		// Verify cookie is set
		var foundCookie bool
		for _, c := range w.Result().Cookies() {
			if c.Name == model.SessionCookie {
				foundCookie = true
				assert.Equal(t, model.SessionToken, c.Value)
				assert.Equal(t, "/", c.Path)
				assert.True(t, c.HttpOnly)
			}
		}
		assert.True(t, foundCookie, "expected session cookie to be set")
	})

	t.Run("POST_WrongPassword_Returns401", func(t *testing.T) {
		_, teardown := setupTestEnv(t)
		defer teardown()

		model.SessionToken = hashPassword("testpass")

		req := newRequest(t, http.MethodPost, "/login", map[string]string{
			"password": "wrongpass",
		})
		w := callHandler(ServeLogin, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assertJSONField(t, w, "ok", false)
	})

	t.Run("POST_EmptyBody_Returns401", func(t *testing.T) {
		_, teardown := setupTestEnv(t)
		defer teardown()

		model.SessionToken = hashPassword("testpass")

		req := newRequest(t, http.MethodPost, "/login", map[string]string{
			"password": "",
		})
		w := callHandler(ServeLogin, req)

		// Empty password hash won't match the set token
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assertJSONField(t, w, "ok", false)
	})

	t.Run("POST_NoPasswordRequired_Returns200", func(t *testing.T) {
		_, teardown := setupTestEnv(t)
		defer teardown()

		// No password configured — any password should work
		model.SessionToken = ""

		req := newRequest(t, http.MethodPost, "/login", map[string]string{
			"password": "anything",
		})
		w := callHandler(ServeLogin, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assertJSONField(t, w, "ok", true)
	})

	t.Run("OtherMethod_Returns405", func(t *testing.T) {
		_, teardown := setupTestEnv(t)
		defer teardown()

		req := newRequest(t, http.MethodPut, "/login", nil)
		w := callHandler(ServeLogin, req)

		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	})
}
