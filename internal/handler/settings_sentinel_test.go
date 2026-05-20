//go:build !windows

package handler

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ---------- IsRunningUnderSupervisor ----------

func TestIsRunningUnderSupervisor_CLAWBENCH_NO_SUPERVISOR(t *testing.T) {
	orig := os.Getenv("CLAWBENCH_NO_SUPERVISOR")
	os.Setenv("CLAWBENCH_NO_SUPERVISOR", "1")
	defer os.Setenv("CLAWBENCH_NO_SUPERVISOR", orig)

	assert.False(t, IsRunningUnderSupervisor(), "CLAWBENCH_NO_SUPERVISOR=1 should return false")
}

func TestIsRunningUnderSupervisor_INVOCATION_ID(t *testing.T) {
	origNo := os.Getenv("CLAWBENCH_NO_SUPERVISOR")
	origInv := os.Getenv("INVOCATION_ID")
	os.Setenv("CLAWBENCH_NO_SUPERVISOR", "")
	os.Setenv("INVOCATION_ID", "test-id")
	defer func() {
		os.Setenv("CLAWBENCH_NO_SUPERVISOR", origNo)
		os.Setenv("INVOCATION_ID", origInv)
	}()

	assert.True(t, IsRunningUnderSupervisor(), "INVOCATION_ID set should return true")
}

func TestIsRunningUnderSupervisor_ContainerEnv(t *testing.T) {
	origNo := os.Getenv("CLAWBENCH_NO_SUPERVISOR")
	origCont := os.Getenv("container")
	os.Setenv("CLAWBENCH_NO_SUPERVISOR", "")
	os.Setenv("container", "docker")
	defer func() {
		os.Setenv("CLAWBENCH_NO_SUPERVISOR", origNo)
		os.Setenv("container", origCont)
	}()

	assert.True(t, IsRunningUnderSupervisor(), "container env set should return true")
}

func TestIsRunningUnderSupervisor_NoIndicators(t *testing.T) {
	origNo := os.Getenv("CLAWBENCH_NO_SUPERVISOR")
	origInv := os.Getenv("INVOCATION_ID")
	origCont := os.Getenv("container")
	os.Setenv("CLAWBENCH_NO_SUPERVISOR", "")
	os.Setenv("INVOCATION_ID", "")
	os.Setenv("container", "")
	defer func() {
		os.Setenv("CLAWBENCH_NO_SUPERVISOR", origNo)
		os.Setenv("INVOCATION_ID", origInv)
		os.Setenv("container", origCont)
	}()

	// Under normal test execution (not PID 1, no dockerenv), should be false
	// unless running in CI with these indicators set
	result := IsRunningUnderSupervisor()
	// Can't assert exact value since PID 1 check depends on environment,
	// but it should not panic
	_ = result
}

func TestIsRunningUnderSupervisor_DockerenvFile(t *testing.T) {
	origNo := os.Getenv("CLAWBENCH_NO_SUPERVISOR")
	origInv := os.Getenv("INVOCATION_ID")
	origCont := os.Getenv("container")
	os.Setenv("CLAWBENCH_NO_SUPERVISOR", "")
	os.Setenv("INVOCATION_ID", "")
	os.Setenv("container", "")
	defer func() {
		os.Setenv("CLAWBENCH_NO_SUPERVISOR", origNo)
		os.Setenv("INVOCATION_ID", origInv)
		os.Setenv("container", origCont)
	}()

	// Create /.dockerenv temporarily
	if os.Getuid() == 0 {
		os.WriteFile("/.dockerenv", []byte{}, 0644)
		defer os.Remove("/.dockerenv")
		assert.True(t, IsRunningUnderSupervisor(), "/.dockerenv exists should return true")
	}
	// If not root, we can't create /.dockerenv, so just verify it doesn't panic
	IsRunningUnderSupervisor()
}

// ---------- shellQuote ----------

func TestShellQuote(t *testing.T) {
	assert.Equal(t, "'hello'", shellQuote("hello"))
	assert.Equal(t, "''", shellQuote(""))
	assert.Equal(t, `'it'\''s'`, shellQuote("it's"))
	assert.Equal(t, `'a'\''b'\''c'`, shellQuote("a'b'c"))
}

// ---------- joinArgs ----------

func TestJoinArgs(t *testing.T) {
	assert.Equal(t, "", joinArgs(nil))
	assert.Equal(t, "'hello'", joinArgs([]string{"hello"}))
	assert.Equal(t, "'hello' 'world'", joinArgs([]string{"hello", "world"}))
	assert.Equal(t, `'it'\''s' 'nice'`, joinArgs([]string{"it's", "nice"}))
}

// ---------- maskAPIKey ----------

func TestMaskAPIKey_Empty(t *testing.T) {
	assert.Equal(t, "", maskAPIKey(""))
}

func TestMaskAPIKey_Short(t *testing.T) {
	assert.Equal(t, "****", maskAPIKey("abc"))
	assert.Equal(t, "****", maskAPIKey("1234567"))
}

func TestMaskAPIKey_LongEnough(t *testing.T) {
	result := maskAPIKey("abcdefgh")
	assert.Equal(t, "abcd***fgh", result)
}

func TestMaskAPIKey_16Chars(t *testing.T) {
	result := maskAPIKey("abcdefghijklmnop")
	assert.Equal(t, "abcd***nop", result)
}
