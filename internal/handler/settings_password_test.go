package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"clawbench/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestServeConfigPassword_Success(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	// Set up plaintext password with bcrypt hash
	password := "test-password"
	hash := sha256.Sum256([]byte(password + "clawbench-salt"))
	model.SessionToken = hex.EncodeToString(hash[:])
	model.PasswordIsSHA256 = false
	bcryptHash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	model.PasswordHash = bcryptHash
	model.ConfigInstance = model.Config{}
	model.BinDir = t.TempDir()
	os.MkdirAll(filepath.Join(model.BinDir, "config"), 0755)

	req := newRequest(t, http.MethodPost, "/api/config/password", map[string]string{
		"current_password": password,
		"new_password":     "new-password-123",
	})
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfigPassword, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, true, resp["needs_restart"])

	// Verify config.yaml was written with sha256: prefix
	configData, err := os.ReadFile(filepath.Join(model.BinDir, "config", "config.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(configData), "sha256:")

	// Verify ConfigInstance.Password was updated
	assert.True(t, model.IsSHA256Password(model.ConfigInstance.Password))
}

func TestServeConfigPassword_WrongCurrentPassword(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	password := "correct-password"
	hash := sha256.Sum256([]byte(password + "clawbench-salt"))
	model.SessionToken = hex.EncodeToString(hash[:])
	model.PasswordIsSHA256 = false
	bcryptHash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	model.PasswordHash = bcryptHash
	model.ConfigInstance = model.Config{}

	req := newRequest(t, http.MethodPost, "/api/config/password", map[string]string{
		"current_password": "wrong-password",
		"new_password":     "new-password-123",
	})
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfigPassword, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "wrong_password", resp["error"])
}

func TestServeConfigPassword_EmptyFields(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	model.SessionToken = "sometoken"
	model.ConfigInstance = model.Config{}

	req := newRequest(t, http.MethodPost, "/api/config/password", map[string]string{
		"current_password": "",
		"new_password":     "",
	})
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfigPassword, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "empty_password", resp["error"])
}

func TestServeConfigPassword_TooShort(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	password := "test-password"
	hash := sha256.Sum256([]byte(password + "clawbench-salt"))
	model.SessionToken = hex.EncodeToString(hash[:])
	model.PasswordIsSHA256 = false
	bcryptHash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	model.PasswordHash = bcryptHash
	model.ConfigInstance = model.Config{}

	req := newRequest(t, http.MethodPost, "/api/config/password", map[string]string{
		"current_password": password,
		"new_password":     "abc",
	})
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfigPassword, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "password_too_short", resp["error"])
}

func TestServeConfigPassword_SHA256StoredPassword(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	// Simulate password already stored as SHA-256 hash
	password := "stored-sha256-password"
	hash := sha256.Sum256([]byte(password + "clawbench-salt"))
	hashStr := hex.EncodeToString(hash[:])
	model.SessionToken = hashStr
	model.PasswordIsSHA256 = true
	model.PasswordHash = nil // No bcrypt when stored as SHA-256
	model.ConfigInstance = model.Config{}
	model.BinDir = t.TempDir()
	os.MkdirAll(filepath.Join(model.BinDir, "config"), 0755)

	req := newRequest(t, http.MethodPost, "/api/config/password", map[string]string{
		"current_password": password,
		"new_password":     "brand-new-password",
	})
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfigPassword, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, true, resp["needs_restart"])

	// Verify config.yaml was written with sha256: prefix
	configData, err := os.ReadFile(filepath.Join(model.BinDir, "config", "config.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(configData), "sha256:")
}

func TestServeConfigPassword_SHA256StoredPassword_WrongCurrent(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	password := "correct-sha256-password"
	hash := sha256.Sum256([]byte(password + "clawbench-salt"))
	hashStr := hex.EncodeToString(hash[:])
	model.SessionToken = hashStr
	model.PasswordIsSHA256 = true
	model.PasswordHash = nil
	model.ConfigInstance = model.Config{}

	req := newRequest(t, http.MethodPost, "/api/config/password", map[string]string{
		"current_password": "wrong-password",
		"new_password":     "brand-new-password",
	})
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfigPassword, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestServeConfigPassword_MethodNotAllowed(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	req := httptest.NewRequest(http.MethodGet, "/api/config/password", nil)
	withAuthCookie(req, "sometoken")
	w := callHandler(ServeConfigPassword, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestServeConfigPassword_InvalidJSON(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	model.SessionToken = "sometoken"
	req := httptest.NewRequest(http.MethodPost, "/api/config/password", nil)
	req.Header.Set("Content-Type", "application/json")
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfigPassword, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestServeConfigPassword_RateLimited(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	password := "test-password"
	hash := sha256.Sum256([]byte(password + "clawbench-salt"))
	model.SessionToken = hex.EncodeToString(hash[:])
	model.PasswordIsSHA256 = false
	bcryptHash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	model.PasswordHash = bcryptHash
	model.ConfigInstance = model.Config{}

	// Make failed requests to trigger rate limiting
	// The global login limiter persists across tests, so count from current state
	blocked := false
	for i := 0; i < 10; i++ {
		req := newRequest(t, http.MethodPost, "/api/config/password", map[string]string{
			"current_password": "wrong-password",
			"new_password":     "new-password-123",
		})
		withAuthCookie(req, model.SessionToken)
		w := callHandler(ServeConfigPassword, req)
		if w.Code == http.StatusTooManyRequests {
			blocked = true
			break
		}
	}
	assert.True(t, blocked, "expected rate limiting to kick in after repeated failures")
}

func TestServeConfig_Get_HasPassword(t *testing.T) {
	_, teardown := setupTestEnv(t)
	defer teardown()

	cfg := model.Config{}
	model.ConfigInstance = cfg

	// With password set
	model.SessionToken = "sometoken"
	req := newRequest(t, http.MethodGet, "/api/config", nil)
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfig, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, true, resp["has_password"])

	// Without password
	model.SessionToken = ""
	req = newRequest(t, http.MethodGet, "/api/config", nil)
	withAuthCookie(req, "")
	w = callHandler(ServeConfig, req)

	assert.Equal(t, http.StatusOK, w.Code)
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, false, resp["has_password"])
}

func TestServeConfigPassword_TooLong(t *testing.T) {
	_, teardown := setupTestEnv(t)
	// Reset rate limiter to avoid interference from previous tests
	globalLoginLimiter = &loginLimiter{records: make(map[string]*ipRecord)}
	defer teardown()

	password := "test-password"
	hash := sha256.Sum256([]byte(password + "clawbench-salt"))
	model.SessionToken = hex.EncodeToString(hash[:])
	model.PasswordIsSHA256 = false
	bcryptHash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	model.PasswordHash = bcryptHash
	model.ConfigInstance = model.Config{}

	longPassword := ""
	for i := 0; i < 73; i++ {
		longPassword += "a"
	}

	req := newRequest(t, http.MethodPost, "/api/config/password", map[string]string{
		"current_password": password,
		"new_password":     longPassword,
	})
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfigPassword, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "password_too_long", resp["error"])
}

func TestServeConfigPassword_NoPasswordHash(t *testing.T) {
	_, teardown := setupTestEnv(t)
	// Reset rate limiter to avoid interference from previous tests
	globalLoginLimiter = &loginLimiter{records: make(map[string]*ipRecord)}
	defer teardown()

	// Simulate no password set (no bcrypt hash, not SHA-256)
	model.SessionToken = ""
	model.PasswordIsSHA256 = false
	model.PasswordHash = nil
	model.ConfigInstance = model.Config{}

	req := newRequest(t, http.MethodPost, "/api/config/password", map[string]string{
		"current_password": "any-password",
		"new_password":     "new-password-123",
	})
	// No auth cookie needed when no password is set, but require one for auth middleware
	withAuthCookie(req, "")
	w := callHandler(ServeConfigPassword, req)

	// Should fail with wrong_password since neither SHA-256 nor bcrypt matches
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "wrong_password", resp["error"])
}

func TestServeConfigPassword_WriteFailure(t *testing.T) {
	_, teardown := setupTestEnv(t)
	// Reset rate limiter to avoid interference from previous tests
	globalLoginLimiter = &loginLimiter{records: make(map[string]*ipRecord)}
	defer teardown()

	password := "test-password"
	hash := sha256.Sum256([]byte(password + "clawbench-salt"))
	model.SessionToken = hex.EncodeToString(hash[:])
	model.PasswordIsSHA256 = false
	bcryptHash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	model.PasswordHash = bcryptHash
	model.ConfigInstance = model.Config{Password: password}
	// Save and override BinDir to trigger write failure, restore on cleanup
	origBinDir := model.BinDir
	model.BinDir = "/nonexistent/path/for/test"
	defer func() { model.BinDir = origBinDir }()

	req := newRequest(t, http.MethodPost, "/api/config/password", map[string]string{
		"current_password": password,
		"new_password":     "new-password-123",
	})
	withAuthCookie(req, model.SessionToken)
	w := callHandler(ServeConfigPassword, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "write_failed", resp["error"])

	// Verify in-memory config was rolled back
	assert.Equal(t, password, model.ConfigInstance.Password)
}
