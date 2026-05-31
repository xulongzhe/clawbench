package ai

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"clawbench/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- CLIBackend ExecuteStream ---

func TestCLIBackend_ExecuteStream_CommandFailure(t *testing.T) {
	b := &CLIBackend{
		name:           "test",
		defaultCommand: "nonexistent-cli-command-12345",
		buildArgs:      func(req ChatRequest) []string { return []string{} },
		newParser:      func() LineParser { return &StreamParser{} },
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err := b.ExecuteStream(ctx, ChatRequest{
		Prompt:    "test",
		SessionID: "test-session",
		WorkDir:   t.TempDir(),
	})
	// Command doesn't exist, so Start should fail
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "test stream: failed to start command")
}

func TestCLIBackend_ExecuteStream_ContextCancellation(t *testing.T) {
	b := &CLIBackend{
		name:           "test",
		defaultCommand: "sleep", // will be cancelled
		buildArgs:      func(req ChatRequest) []string { return []string{"300"} },
		newParser:      func() LineParser { return &StreamParser{} },
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel before calling ExecuteStream — the command start should fail
	cancel()

	_, err := b.ExecuteStream(ctx, ChatRequest{
		Prompt:    "test",
		SessionID: "test-session",
		WorkDir:   t.TempDir(),
	})
	// With context already cancelled, either the command fails to start
	// or starts and is immediately killed. Either way, an error is expected.
	assert.Error(t, err, "pre-cancelled context should produce an error")
}

// --- CLIBackend filterLine helpers ---

func TestFilterSkipNonJSON(t *testing.T) {
	f := filterSkipNonJSON()

	_, ok := f("")
	assert.False(t, ok)

	_, ok = f("not json")
	assert.False(t, ok)

	line, ok := f(`{"type":"content"}`)
	assert.True(t, ok)
	assert.Equal(t, `{"type":"content"}`, line)
}

// --- injectAgentAPIKey ---

func TestInjectAgentAPIKey_NilLoader(t *testing.T) {
	orig := agentAPIKeyLoader
	agentAPIKeyLoader = nil
	defer func() { agentAPIKeyLoader = orig }()

	cmd := exec.CommandContext(context.Background(), "echo", "test")
	req := ChatRequest{AgentID: "test-agent"}
	injectAgentAPIKey(cmd, req)
	// Should be a no-op — no panic, no changes
	assert.Equal(t, "echo", cmd.Args[0])
}

func TestInjectAgentAPIKey_AgentNotFound(t *testing.T) {
	origLoader := agentAPIKeyLoader
	origAgents := model.Agents
	defer func() { agentAPIKeyLoader = origLoader; model.Agents = origAgents }()

	agentAPIKeyLoader = func(agentID string) (string, string, string, bool) {
		return "", "", "", false
	}
	model.Agents = map[string]*model.Agent{} // no agents

	cmd := exec.CommandContext(context.Background(), "echo", "test")
	req := ChatRequest{AgentID: "nonexistent"}
	injectAgentAPIKey(cmd, req)
	assert.Equal(t, "echo", cmd.Args[0])
}

func TestInjectAgentAPIKey_NonPiBackend(t *testing.T) {
	origLoader := agentAPIKeyLoader
	origAgents := model.Agents
	defer func() { agentAPIKeyLoader = origLoader; model.Agents = origAgents }()

	agentAPIKeyLoader = func(agentID string) (string, string, string, bool) {
		return "openai", "", "sk-test", true
	}
	model.Agents = map[string]*model.Agent{
		"codebuddy": {ID: "codebuddy", Backend: "codebuddy"},
	}

	cmd := exec.CommandContext(context.Background(), "echo", "test")
	req := ChatRequest{AgentID: "codebuddy"}
	injectAgentAPIKey(cmd, req)
	// Non-Pi backend — should be a no-op
	assert.Equal(t, "echo", cmd.Args[0])
}

func TestInjectAgentAPIKey_KeyNotFound(t *testing.T) {
	origLoader := agentAPIKeyLoader
	origAgents := model.Agents
	defer func() { agentAPIKeyLoader = origLoader; model.Agents = origAgents }()

	agentAPIKeyLoader = func(agentID string) (string, string, string, bool) {
		return "", "", "", false
	}
	model.Agents = map[string]*model.Agent{
		"pi-agent": {ID: "pi-agent", Backend: "pi"},
	}

	cmd := exec.CommandContext(context.Background(), "echo", "test")
	req := ChatRequest{AgentID: "pi-agent"}
	injectAgentAPIKey(cmd, req)
	// No key found — should be a no-op
	assert.Equal(t, "echo", cmd.Args[0])
}

func TestInjectAgentAPIKey_EmptyKey(t *testing.T) {
	origLoader := agentAPIKeyLoader
	origAgents := model.Agents
	defer func() { agentAPIKeyLoader = origLoader; model.Agents = origAgents }()

	agentAPIKeyLoader = func(agentID string) (string, string, string, bool) {
		return "openai", "", "", true
	}
	model.Agents = map[string]*model.Agent{
		"pi-agent": {ID: "pi-agent", Backend: "pi"},
	}

	cmd := exec.CommandContext(context.Background(), "echo", "test")
	req := ChatRequest{AgentID: "pi-agent"}
	injectAgentAPIKey(cmd, req)
	// Empty key — should be a no-op
	assert.Equal(t, "echo", cmd.Args[0])
}

func TestInjectAgentAPIKey_CustomURL(t *testing.T) {
	origLoader := agentAPIKeyLoader
	origAgents := model.Agents
	defer func() { agentAPIKeyLoader = origLoader; model.Agents = origAgents }()

	agentAPIKeyLoader = func(agentID string) (string, string, string, bool) {
		return "custom-agent", "https://api.deepseek.com/v1/chat/completions", "sk-deepseek-key", true
	}
	model.Agents = map[string]*model.Agent{
		"custom-agent": {ID: "custom-agent", Backend: "pi"},
	}

	cmd := exec.CommandContext(context.Background(), "pi", "--mode", "json", "prompt")
	cmd.Env = os.Environ()
	req := ChatRequest{AgentID: "custom-agent"}
	injectAgentAPIKey(cmd, req)

	// Custom URL: append(cmd.Args[:len-1], "--provider", provider, "--api-key", apiKey, cmd.Args[len-1])
	// Original: [pi, --mode, json, prompt] → before last: [pi, --mode, json] + [prompt] = last
	// Result: [pi, --mode, json, --provider, custom-agent, --api-key, sk-deepseek-key, prompt]
	require.Len(t, cmd.Args, 8, "should have 3 prefix + 4 injected + 1 last = 8")
	assert.Equal(t, "pi", cmd.Args[0])
	assert.Equal(t, "--mode", cmd.Args[1])
	assert.Equal(t, "json", cmd.Args[2])
	assert.Equal(t, "--provider", cmd.Args[3])
	assert.Equal(t, "custom-agent", cmd.Args[4])
	assert.Equal(t, "--api-key", cmd.Args[5])
	assert.Equal(t, "sk-deepseek-key", cmd.Args[6])
	assert.Equal(t, "prompt", cmd.Args[7])
}

func TestInjectAgentAPIKey_BuiltInProvider(t *testing.T) {
	origLoader := agentAPIKeyLoader
	origAgents := model.Agents
	defer func() { agentAPIKeyLoader = origLoader; model.Agents = origAgents }()

	agentAPIKeyLoader = func(agentID string) (string, string, string, bool) {
		return "openai", "", "sk-openai-key", true
	}
	model.Agents = map[string]*model.Agent{
		"openai-pi": {ID: "openai-pi", Backend: "pi"},
	}

	cmd := exec.CommandContext(context.Background(), "pi", "--mode", "json", "prompt")
	cmd.Env = os.Environ()
	req := ChatRequest{AgentID: "openai-pi"}
	injectAgentAPIKey(cmd, req)

	// Built-in provider: should inject env var + --provider before last arg
	require.Len(t, cmd.Args, 6, "should have 3 prefix + --provider openai + 1 last = 6")
	assert.Equal(t, "--provider", cmd.Args[3])
	assert.Equal(t, "openai", cmd.Args[4])
	assert.Equal(t, "prompt", cmd.Args[5])

	// Check env var was injected
	hasEnvVar := false
	for _, env := range cmd.Env {
		if env == "OPENAI_API_KEY=sk-openai-key" {
			hasEnvVar = true
			break
		}
	}
	assert.True(t, hasEnvVar, "should inject OPENAI_API_KEY env var")
}

func TestInjectAgentAPIKey_BuiltInProviderNoEnvVar(t *testing.T) {
	origLoader := agentAPIKeyLoader
	origAgents := model.Agents
	origRegistry := model.ProviderRegistry
	defer func() {
		agentAPIKeyLoader = origLoader
		model.Agents = origAgents
		model.ProviderRegistry = origRegistry
	}()

	agentAPIKeyLoader = func(agentID string) (string, string, string, bool) {
		return "test-no-envvar", "", "test-key", true
	}
	model.Agents = map[string]*model.Agent{
		"test-agent": {ID: "test-agent", Backend: "pi"},
	}
	model.ProviderRegistry["test-no-envvar"] = model.ProviderSpec{
		ID: "test-no-envvar", Name: "Test", EnvVar: "",
		ChatEndpoint: "https://api.example.com/v1/chat/completions",
		APIFormat: "openai", WizardReady: true,
	}

	cmd := exec.CommandContext(context.Background(), "pi", "--mode", "json", "prompt")
	cmd.Env = os.Environ()
	req := ChatRequest{AgentID: "test-agent"}
	injectAgentAPIKey(cmd, req)

	// No EnvVar → no --provider flag, no env var
	assert.Equal(t, []string{"pi", "--mode", "json", "prompt"}, cmd.Args)
}

