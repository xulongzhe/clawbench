package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderRegistry_ContainsAllWizardReadyProviders(t *testing.T) {
	expectedProviders := []string{
		"openai", "anthropic", "google", "deepseek", "groq",
		"openrouter", "cerebras", "xai", "mistral", "fireworks",
		"minimax", "minimax-cn", "kimi-coding", "moonshotai", "moonshotai-cn",
		"xiaomi", "xiaomi-token-plan-cn", "xiaomi-token-plan-ams", "xiaomi-token-plan-sgp",
		"zai", "huggingface", "opencode", "vercel-ai-gateway",
	}

	for _, id := range expectedProviders {
		spec, ok := ProviderRegistry[id]
		require.True(t, ok, "ProviderRegistry missing provider: %s", id)
		assert.True(t, spec.WizardReady, "provider %s should be WizardReady", id)
		assert.Equal(t, id, spec.ID)
		assert.NotEmpty(t, spec.Name)
		assert.NotEmpty(t, spec.EnvVar, "provider %s should have EnvVar", id)
		assert.True(t, spec.SupportsCLI, "provider %s should support CLI", id)
	}
}

func TestProviderRegistry_EnterpriseProvidersNotWizardReady(t *testing.T) {
	enterpriseProviders := []string{
		"amazon-bedrock", "azure-openai-responses",
		"cloudflare-ai-gateway", "cloudflare-workers-ai",
		"google-vertex",
	}

	for _, id := range enterpriseProviders {
		spec, ok := ProviderRegistry[id]
		require.True(t, ok, "ProviderRegistry missing enterprise provider: %s", id)
		assert.False(t, spec.WizardReady, "enterprise provider %s should NOT be WizardReady", id)
	}
}

func TestProviderRegistry_AllProvidersHaveRequiredFields(t *testing.T) {
	for id, spec := range ProviderRegistry {
		assert.Equal(t, id, spec.ID, "ProviderRegistry key %s should match spec.ID %s", id, spec.ID)
		assert.NotEmpty(t, spec.Name, "provider %s missing Name", id)
		assert.NotEmpty(t, spec.ID, "provider %s missing ID", id)

		if spec.WizardReady {
			assert.NotEmpty(t, spec.EnvVar, "WizardReady provider %s missing EnvVar", id)
		}

		// APIFormat must be "openai" or "anthropic" (or empty for enterprise)
		if spec.APIFormat != "" {
			assert.True(t, spec.APIFormat == "openai" || spec.APIFormat == "anthropic",
				"provider %s has invalid APIFormat: %s", id, spec.APIFormat)
		}

		// Providers with ModelsEndpoint may also have KnownModels (from generated JSON)
		// as a fallback when the endpoint is unreachable — this is intentional.
		// Only assert KnownModels are populated for Anthropic-format providers (no ModelsEndpoint).
		if spec.ModelsEndpoint == "" && spec.WizardReady {
			assert.NotEmpty(t, spec.KnownModels,
				"WizardReady provider %s with no ModelsEndpoint should have KnownModels (from generated JSON)", id)
		}
	}
}

func TestProviderRegistry_AnthropicFormatProvidersHaveKnownModels(t *testing.T) {
	anthropicProviders := []string{
		"anthropic", "fireworks", "minimax", "minimax-cn",
		"kimi-coding", "vercel-ai-gateway",
	}

	for _, id := range anthropicProviders {
		spec, ok := ProviderRegistry[id]
		require.True(t, ok, "missing provider: %s", id)
		assert.Equal(t, "anthropic", spec.APIFormat, "provider %s should be anthropic format", id)
		assert.Empty(t, spec.ModelsEndpoint, "anthropic-format provider %s should have empty ModelsEndpoint", id)
		assert.NotEmpty(t, spec.KnownModels, "anthropic-format provider %s should have KnownModels", id)

		for _, m := range spec.KnownModels {
			assert.NotEmpty(t, m.ID, "KnownModel in provider %s missing ID", id)
			assert.NotEmpty(t, m.Name, "KnownModel in provider %s missing Name", id)
			assert.NotEmpty(t, m.CostTier, "KnownModel %s in provider %s missing CostTier", m.ID, id)
			assert.True(t, m.CostTier == "cheap" || m.CostTier == "moderate" || m.CostTier == "expensive",
				"KnownModel %s has invalid CostTier: %s", m.ID, m.CostTier)
		}
	}
}

func TestProviderRegistry_OpenAIFormatProvidersHaveEndpoints(t *testing.T) {
	openaiProviders := []string{
		"openai", "deepseek", "groq", "openrouter", "cerebras", "xai",
		"mistral", "moonshotai", "moonshotai-cn", "xiaomi",
		"xiaomi-token-plan-cn", "xiaomi-token-plan-ams", "xiaomi-token-plan-sgp",
		"zai", "huggingface", "opencode", "google",
	}

	for _, id := range openaiProviders {
		spec, ok := ProviderRegistry[id]
		require.True(t, ok, "missing provider: %s", id)
		assert.Equal(t, "openai", spec.APIFormat, "provider %s should be openai format", id)
		assert.NotEmpty(t, spec.ChatEndpoint, "openai-format provider %s missing ChatEndpoint", id)
		assert.NotEmpty(t, spec.ModelsEndpoint, "openai-format provider %s missing ModelsEndpoint", id)
	}
}

func TestProviderRegistry_AnthropicProviderModels(t *testing.T) {
	spec, ok := ProviderRegistry["anthropic"]
	require.True(t, ok)
	require.NotEmpty(t, spec.KnownModels)

	// Check for key models
	modelIDs := make(map[string]bool)
	for _, m := range spec.KnownModels {
		modelIDs[m.ID] = true
	}
	assert.True(t, modelIDs["claude-sonnet-4-20250514"], "anthropic should include Claude Sonnet 4")
	assert.True(t, modelIDs["claude-3-5-haiku-20241022"], "anthropic should include Claude 3.5 Haiku")

	// Check all KnownModels have valid cost tiers
	for _, m := range spec.KnownModels {
		assert.True(t, m.CostTier == "cheap" || m.CostTier == "moderate" || m.CostTier == "expensive",
			"KnownModel %s has invalid CostTier: %s", m.ID, m.CostTier)
	}
}

func TestGetWizardProviders_ReturnsOnlyWizardReady(t *testing.T) {
	providers := GetWizardProviders()

	assert.NotEmpty(t, providers)
	for _, p := range providers {
		assert.True(t, p.WizardReady, "GetWizardProviders should only return WizardReady providers, got: %s", p.ID)
	}

	// Verify enterprise providers are NOT included
	providerIDs := make(map[string]bool)
	for _, p := range providers {
		providerIDs[p.ID] = true
	}
	assert.False(t, providerIDs["amazon-bedrock"], "enterprise providers should not be in wizard list")
	assert.False(t, providerIDs["azure-openai-responses"], "enterprise providers should not be in wizard list")
	assert.False(t, providerIDs["google-vertex"], "enterprise providers should not be in wizard list")
}

func TestGetWizardProviders_SortedByID(t *testing.T) {
	providers := GetWizardProviders()
	for i := 1; i < len(providers); i++ {
		assert.LessOrEqual(t, providers[i-1].ID, providers[i].ID,
			"GetWizardProviders should be sorted by ID")
	}
}

func TestGetSummarizeModelHint_KnownModels(t *testing.T) {
	spec := ProviderRegistry["anthropic"]
	hint := GetSummarizeModelHint(spec.KnownModels, nil)
	// Anthropic no longer has a "cheap" model (all are moderate/expensive per models.dev pricing)
	// so the hint should fall back to the first known model
	assert.NotEmpty(t, hint, "should return a model hint for anthropic")
}

func TestGetSummarizeModelHint_V1Models(t *testing.T) {
	models := []ModelInfo{
		{ID: "gpt-5.5", Name: "GPT-5.5"},
		{ID: "gpt-4o-mini", Name: "GPT-4o Mini"},
		{ID: "gpt-5.4", Name: "GPT-5.4"},
	}
	hint := GetSummarizeModelHint(nil, models)
	assert.Equal(t, "gpt-4o-mini", hint, "should pick model matching 'mini' keyword")
}

func TestGetSummarizeModelHint_V1Models_FlashKeyword(t *testing.T) {
	models := []ModelInfo{
		{ID: "gemini-2.5-pro", Name: "Gemini 2.5 Pro"},
		{ID: "gemini-2.5-flash", Name: "Gemini 2.5 Flash"},
	}
	hint := GetSummarizeModelHint(nil, models)
	assert.Equal(t, "gemini-2.5-flash", hint, "should pick model matching 'flash' keyword")
}

func TestGetSummarizeModelHint_V1Models_NoMatchFallsToFirst(t *testing.T) {
	models := []ModelInfo{
		{ID: "some-model-1", Name: "Some Model 1"},
		{ID: "some-model-2", Name: "Some Model 2"},
	}
	hint := GetSummarizeModelHint(nil, models)
	assert.Equal(t, "some-model-1", hint, "should fall back to first model when no keywords match")
}

func TestGetSummarizeModelHint_V1Models_MiniDoesNotMatchGemini(t *testing.T) {
	// "mini" keyword should NOT match "gemini" (false positive)
	// It should only match hyphen/dot/slash delimited segments
	models := []ModelInfo{
		{ID: "gemini-2.5-pro", Name: "Gemini 2.5 Pro"},
		{ID: "gemini-2.5-flash", Name: "Gemini 2.5 Flash"},
	}
	hint := GetSummarizeModelHint(nil, models)
	assert.Equal(t, "gemini-2.5-flash", hint, "should pick flash model, not match 'mini' inside 'gemini'")
}

func TestGetSummarizeModelHint_V1Models_MiniMatchesHyphenated(t *testing.T) {
	models := []ModelInfo{
		{ID: "gpt-5.5", Name: "GPT-5.5"},
		{ID: "gpt-4o-mini", Name: "GPT-4o Mini"},
	}
	hint := GetSummarizeModelHint(nil, models)
	assert.Equal(t, "gpt-4o-mini", hint, "should match '-mini' segment in 'gpt-4o-mini'")
}

func TestGetSummarizeModelHint_EmptyBoth(t *testing.T) {
	hint := GetSummarizeModelHint(nil, nil)
	assert.Equal(t, "", hint, "should return empty when no models available")
}

func TestFindProviderSpec(t *testing.T) {
	spec := FindProviderSpec("openai")
	require.NotNil(t, spec)
	assert.Equal(t, "OpenAI", spec.Name)

	spec = FindProviderSpec("nonexistent")
	assert.Nil(t, spec)
}
