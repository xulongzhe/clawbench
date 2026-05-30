# Built-in Agent Setup Wizard Design

**Date:** 2026-05-30
**Status:** Draft (post-review revision)

## Overview

When ClawBench starts and detects zero agents (no CLI tools on PATH, no database records), a fullscreen unclosable step-by-step setup wizard guides the user to configure a built-in Pi agent. Pi's standalone binary is bundled into the release package, so no Node.js or npm is required.

Key changes:
1. **Agent configuration migrates from YAML files to SQLite database** (one-time migration, YAML loading permanently removed)
2. **Pi binary embedded in release package** (downloaded during `build.sh`, ~46MB)
3. **Setup wizard** (5-step flow: welcome → provider → API key → model+verify → agent name)
4. **New backend APIs** under `/api/setup/` (provider-agnostic naming)

## Trigger Condition

Backend returns `needs_setup: true` when `AgentList` is empty. Frontend shows fullscreen wizard when `needs_setup=true && embedded_agent=true`; shows restart prompt only when `needs_setup=true && embedded_agent=false`.

## Wizard Flow (5 Steps)

```
Step 1: Welcome
  - Message: "未检测到智能体，是否配置使用内置智能体？"
  - Button: "配置内置智能体" → Step 2
  - If embedded_agent=false: button disabled, only shows restart prompt
  - Cannot close wizard

Step 2: Select Provider
  - Recommended row: OpenAI / Anthropic / Google Gemini
  - Full provider list from Pi (scrollable, searchable)
  - "Custom Base URL" option at bottom
  - Select → Step 3

Step 3: Enter API Key
  - Shows provider name + env var hint (e.g. OPENAI_API_KEY)
  - Password input with visibility toggle
  - Custom URL mode: prompts for Base URL + API Key
  - Next → Step 4

Step 4: Select Model + Verify
  - Auto-calls POST /api/setup/models to list available models via OpenAI-compatible /v1/models endpoint
  - **两个模型选择：**
    1. "对话模型" — 默认选中列表中第一个模型，用户可切换
    2. "总结模型" — 用于聊天摘要、任务总结，应选更小更便宜的模型
       - 智能推荐：按关键词优先匹配（mini/flash/haiku/lite/small → 推荐），无匹配则默认选对话模型
       - 用户可手动切换
  - "验证配置" button → POST /api/setup/verify (lightweight API connectivity check using the chat model)
  - Must pass verification to proceed (cannot skip)
  - Failure: show error, allow retry or go back to change API key

Step 5: Agent Name & ID
  - Auto-generated from provider (e.g. Google Gemini → name "Google Gemini", id "google-gemini")
  - User can modify both fields
  - ID auto-derived from name (lowercase + hyphens), editable
  - Icon: 🥧 (Pi default)
  - "完成配置" → POST /api/setup/complete → close wizard → main UI
```

**Wizard state persistence:** Wizard state (current step, selected provider, API key, chat model, summarize model) is stored in `sessionStorage` so that a page refresh does not lose progress. The `api_key` field is kept only in memory (not persisted to sessionStorage) for security.

## Backend API

### Auth & Concurrency

All `/api/setup/` endpoints use `middleware.Auth` (same as all other `/api/` routes — localhost bypass for CLI, cookie auth for browser). This is critical because setup endpoints receive API keys.

**Concurrency guard:** `POST /api/setup/complete` is protected by a `sync.Mutex` to prevent duplicate agent creation from concurrent requests (e.g., double-click). The mutex is checked before any writes; if locked, returns 409 Conflict.

### Endpoints

```
GET  /api/setup/status       → { needs_setup, embedded_agent, agent_version }
GET  /api/setup/providers    → { providers: [{id, name, envVar}], custom_url_supported }
POST /api/setup/models       → { models: [{id, name, created}], summarize_model_hint }
POST /api/setup/verify       → { success, message, model }
POST /api/setup/complete     → { success, agent, default_agent_id }
DELETE /api/agents/{id}      → { success } (existing endpoint, needed for cleanup)
```

### `GET /api/setup/status`

```json
{
  "needs_setup": true,
  "embedded_agent": true,
  "agent_version": "0.78.0"
}
```

### `GET /api/setup/providers`

Returns provider list from `ProviderRegistry` in `internal/model/provider_registry.go`. **Only providers with `WizardReady=true`** are returned — enterprise providers (Bedrock, Azure, Cloudflare, Vertex) are excluded because the wizard's single API key field cannot configure their multi-field authentication.

**Provider list maintenance:** The provider list is a Go constant compiled into the binary. When Pi updates its provider list, ClawBench needs a corresponding code update. The `ProviderSpec` struct contains `ChatEndpoint`, `ModelsEndpoint`, and `APIFormat` which are used for summarize backend auto-configuration — these are NOT exposed to the frontend (only `id`, `name`, `envVar` are sent).

```json
{
  "providers": [
    { "id": "anthropic", "name": "Anthropic", "envVar": "ANTHROPIC_API_KEY" },
    { "id": "openai", "name": "OpenAI", "envVar": "OPENAI_API_KEY" },
    { "id": "google", "name": "Google Gemini", "envVar": "GEMINI_API_KEY" },
    { "id": "deepseek", "name": "DeepSeek", "envVar": "DEEPSEEK_API_KEY" },
    { "id": "minimax", "name": "MiniMax", "envVar": "MINIMAX_API_KEY" },
    { "id": "minimax-cn", "name": "MiniMax (China)", "envVar": "MINIMAX_API_KEY" },
    { "id": "groq", "name": "Groq", "envVar": "GROQ_API_KEY" },
    { "id": "openrouter", "name": "OpenRouter", "envVar": "OPENROUTER_API_KEY" },
    { "id": "mistral", "name": "Mistral", "envVar": "MISTRAL_API_KEY" },
    { "id": "xai", "name": "xAI Grok", "envVar": "XAI_API_KEY" },
    { "id": "cerebras", "name": "Cerebras", "envVar": "CEREBRAS_API_KEY" },
    { "id": "fireworks", "name": "Fireworks", "envVar": "FIREWORKS_API_KEY" },
    { "id": "moonshotai", "name": "Moonshot AI", "envVar": "MOONSHOT_API_KEY" },
    { "id": "moonshotai-cn", "name": "Moonshot AI (China)", "envVar": "MOONSHOT_API_KEY" },
    { "id": "opencode", "name": "OpenCode Zen", "envVar": "OPENCODE_API_KEY" },
    { "id": "kimi-coding", "name": "Kimi For Coding", "envVar": "KIMI_API_KEY" },
    { "id": "zai", "name": "ZAI", "envVar": "ZAI_API_KEY" },
    { "id": "huggingface", "name": "Hugging Face", "envVar": "HF_API_KEY" },
    { "id": "vercel-ai-gateway", "name": "Vercel AI Gateway", "envVar": "AI_GATEWAY_API_KEY" },
    { "id": "xiaomi", "name": "Xiaomi MiMo", "envVar": "XIAOMI_API_KEY" },
    { "id": "xiaomi-token-plan-cn", "name": "Xiaomi MiMo Token Plan (China)", "envVar": "XIAOMI_TOKEN_PLAN_CN_API_KEY" },
    { "id": "xiaomi-token-plan-ams", "name": "Xiaomi MiMo Token Plan (Amsterdam)", "envVar": "XIAOMI_TOKEN_PLAN_AMS_API_KEY" },
    { "id": "xiaomi-token-plan-sgp", "name": "Xiaomi MiMo Token Plan (Singapore)", "envVar": "XIAOMI_TOKEN_PLAN_SGP_API_KEY" }
  ],
  "custom_url_supported": true
}
```

### `POST /api/setup/models`

Lists available models via **OpenAI-compatible `/v1/models` endpoint** (direct HTTP, not Pi CLI subprocess). This is faster, more universal, and returns model IDs in their native format (no provider prefix).

**For Anthropic-format providers** (anthropic, fireworks, minimax, minimax-cn, kimi-coding, vercel-ai-gateway): Anthropic has no `/v1/models` endpoint (`ModelsEndpoint=""`). Instead, use the hardcoded `KnownModels` list from `ProviderRegistry`. Each `KnownModel` entry includes `id`, `name`, `context_length`, `supports_thinking`, and `cost_tier` for intelligent model selection.

**For Enterprise providers** (`WizardReady=false`): these are never returned by `GET /api/setup/providers` and cannot be selected in the wizard, so this endpoint will never be called with them.

Request:
```json
{
  "provider": "openai",
  "custom_url": "",
  "api_key": "sk-xxx"
}
```

Backend logic:
1. Look up `ProviderRegistry[provider]`
2. If `ModelsEndpoint != ""`: GET `{ModelsEndpoint}` with `Authorization: Bearer {api_key}` → parse OpenAI `/v1/models` response
3. If `ModelsEndpoint == ""` (Anthropic-format): return `ProviderSpec.KnownModels` as model list
4. If `custom_url != ""`: try `{custom_url}/../models` (strip last path segment, append `/models`); if fails, user must manually enter model IDs

**Note:** `ModelsEndpoint` and `ChatEndpoint` are separate fields in `ProviderSpec` because different providers have inconsistent path structures. For example, DeepSeek's chat endpoint is `/v1/chat/completions` but the path from `../models` derivation without `/v1/` would be wrong. Each endpoint is explicitly specified.

Response (from `/v1/models`):
```json
{
  "models": [
    { "id": "gpt-5.5", "name": "gpt-5.5", "created": 1700000000 },
    { "id": "gpt-4o-mini", "name": "gpt-4o-mini", "created": 1699999999 },
    { "id": "gpt-5.4", "name": "gpt-5.4", "created": 1699999998 }
  ],
  "summarize_model_hint": "gpt-4o-mini"
}
```

Response (from KnownModels, Anthropic-format providers):
```json
{
  "models": [
    { "id": "claude-sonnet-4-20250514", "name": "Claude Sonnet 4", "created": 0, "context_length": 200000, "supports_thinking": true, "cost_tier": "expensive" },
    { "id": "claude-3-7-sonnet-20250219", "name": "Claude 3.7 Sonnet", "created": 0, "context_length": 200000, "supports_thinking": true, "cost_tier": "expensive" },
    { "id": "claude-3-5-haiku-20241022", "name": "Claude 3.5 Haiku", "created": 0, "context_length": 200000, "supports_thinking": false, "cost_tier": "cheap" },
    { "id": "claude-3-5-sonnet-20241022", "name": "Claude 3.5 Sonnet", "created": 0, "context_length": 200000, "supports_thinking": true, "cost_tier": "moderate" }
  ],
  "summarize_model_hint": "claude-3-5-haiku-20241022"
}
```

**Field descriptions:**
- `id`: model ID (used in API calls and stored in config)
- `name`: human-readable display name
- `created`: Unix timestamp from `/v1/models` (0 for KnownModels)
- `context_length`: context window in tokens (0 = unknown, only from KnownModels)
- `supports_thinking`: whether model supports extended thinking (only from KnownModels)
- `cost_tier`: "cheap" / "moderate" / "expensive" (only from KnownModels; for `/v1/models`, inferred from `summarize_model_hint` keywords)

**`summarize_model_hint`**: Backend auto-selects a recommended summarize model. Strategy varies by source:
- **KnownModels**: pick the first model with `cost_tier == "cheap"`, fallback to first model
- **`/v1/models`**: scan model IDs for cheap-model keywords in priority order:

| Priority | Keywords | Rationale |
|----------|----------|-----------|
| 1 | `mini` | gpt-4o-mini, etc. |
| 2 | `flash` | gemini-2.0-flash, gemini-2.5-flash |
| 3 | `haiku` | claude-3-5-haiku (if listed via /v1/models) |
| 4 | `lite` | various lite models |
| 5 | `small` | small model variants |

If no keyword matches, `summarize_model_hint` equals the first model in the list (same as chat model). Frontend can override — this is just a suggestion.

**Custom URL models:** When `custom_url` is provided, the backend attempts GET on the custom URL with the last path segment replaced by `/models`. If this fails (404, timeout, etc.), the response includes `"models": []` with an `"error"` field, and the frontend must show manual model ID input fields for both chat and summarize models.

### `POST /api/setup/verify`

Request:
```json
{
  "provider": "openai",
  "custom_url": "",
  "api_key": "sk-xxx",
  "model": "gpt-5.5"
}
```

Backend: sets env var → runs `{embedded_pi} -p --mode json --provider {provider} --model {model} --no-session --no-tools "ping"` → checks for valid response within 30s timeout.

**Note on verification strategy:** Uses `--no-tools` flag to prevent Pi from executing any tools (Read/Write/Bash etc.) during verification. The test prompt "ping" is intentionally minimal. If `--no-tools` is not supported by the Pi version, falls back to `--tools read` (read-only mode) as a safety measure. This avoids the cost and risk of a full inference cycle with tool execution.

Response (success):
```json
{ "success": true, "message": "配置验证成功！智能体工作正常。", "model": "gpt-5.5" }
```

Response (failure):
```json
{ "success": false, "message": "验证失败：API Key 无效或模型不可用。请检查后重试。" }
```

### `POST /api/setup/complete`

Request:
```json
{
  "provider": "openai",
  "custom_url": "",
  "api_key": "sk-xxx",
  "model": "gpt-5.5",
  "summarize_model": "gpt-4o-mini",
  "agent_name": "OpenAI",
  "agent_id": "openai"
}
```

Backend actions (all in a DB transaction for atomicity):
1. **Write `~/.pi/agent/auth.json`** — add provider API key (atomic: write tmp + rename)
2. **Write `~/.pi/agent/settings.json`** — set defaultProvider + defaultModel (atomic: write tmp + rename)
3. If custom_url: **write `~/.pi/agent/models.json`** providers config (atomic: write tmp + rename)
4. **Insert into `agents` table** with `source='setup'`, `command=EmbeddedAgentPath()`, `preferred_model=model`
5. **Insert into `agent_api_keys` table** (encrypted)
6. **Auto-configure as summarize backend** — see details below
7. **Reinitialize summarize backend** at runtime (without restart): create new `OpenAISummarizer`/`AnthropicSummarizer` from provider registry + decrypted API key, and update the global summarizer instances
8. Reload agents from DB + MergeDiscoveredData
9. Return refreshed agent list

**Pi config file atomicity:** All Pi JSON config files (`auth.json`, `settings.json`, `models.json`) are written using the same atomic pattern as `WriteAgentYAML`: write to `.tmp` file, then `os.Rename`. This prevents partial/corrupt writes on crash.

**Pi config file location:** Pi CLI reads config from `~/.pi/agent/` where `~` is the home directory of the user running the ClawBench process. In most deployments, ClawBench runs as the desktop user, so this matches. For system service deployments, the `PI_HOME` env var can override the config directory — set it on the Pi subprocess via `cmd.Env` in `ExecuteStream()`.

**Summarize backend auto-configuration:** When the wizard completes, the newly created agent is automatically set as the summarize backend so chat auto-summary works immediately. **All providers uniformly use `backend: "api"`** (direct HTTP calls via `OpenAISummarizer`/`AnthropicSummarizer`), not the `AIBackendSummarizer` (Pi CLI subprocess). This is faster, cheaper, and avoids spawning a subprocess for every summarization.

**API key is NEVER stored in `config.yaml`** (I2 fix). Instead, the summarize backend reads the API key from `agent_api_keys` table at runtime. The `config.yaml` only stores a reference to the agent, not the key itself.

The configuration is written to `config/config.yaml`:

```yaml
summarize:
  backend: "api"
  model: "gpt-4o-mini"         # summarize_model — smaller/cheaper model for summaries
  api:
    base_url: "{ChatEndpoint}"  # from provider registry (built-in) or user input (custom)
    key: ""                     # EMPTY — key is read from agent_api_keys at runtime
    format: "{APIFormat}"       # "openai" or "anthropic" — from provider registry
    agent_id: "{agent_id}"      # reference to the agent whose API key to use
```

**Summarize backend runtime initialization:** At startup and after wizard completion, the `initTaskSummarizer` function:
1. Reads `summarize.api.agent_id` from config
2. Calls `LoadAgentAPIKey(db, agentID, provider)` to decrypt the key
3. Creates `OpenAISummarizer(baseURL, decryptedKey, model)` or `AnthropicSummarizer(baseURL, decryptedKey, model)`
4. If decryption fails (e.g., password changed without key rotation), logs a warning and falls back to `SimpleSummarizer`

**Enterprise provider fallback:** When `ProviderSpec.ChatEndpoint == ""`, the setup/complete handler falls back to `AIBackendSummarizer` for the summarize backend:

```yaml
# Enterprise provider example (amazon-bedrock) — NOT available in wizard, manual config only
summarize:
  backend: "pi"           # fallback to Pi CLI subprocess
  model: "{selected_model}"
```

### Pi CLI Runtime API Key Injection (C3 fix)

When the user chats via the Pi agent, `CLIBackend.ExecuteStream()` spawns the Pi CLI as a subprocess. Pi CLI needs the API key to authenticate with the provider. The injection mechanism:

1. **At session start** (`buildChatRequest` or `preStart` callback): read `agent_api_keys` table → decrypt → get `(provider, apiKey, customURL)`
2. **Set environment variable** on the `exec.Cmd`:
   ```go
   cmd.Env = append(os.Environ(), fmt.Sprintf("%s=%s", spec.EnvVar, decryptedAPIKey))
   ```
   For custom URL providers, also set `PI_CUSTOM_URL={customURL}`.
3. **Add `--provider {provider}` flag** to Pi CLI args (so Pi knows which provider config to use)
4. **Pi CLI reads the env var** and uses it for API calls, even if `~/.pi/agent/auth.json` has a different key (env var takes priority)

**Why env var injection instead of relying on auth.json:**
- `auth.json` is a single global file per user — if multiple Pi agents exist with different providers/keys, they'd overwrite each other
- Env vars are per-process, so concurrent sessions with different providers work correctly
- Env vars are set at process spawn time and never written to disk

**`ExecuteStream()` code change** (in `internal/ai/cli.go` or backend-specific file):
```go
func (b *CLIBackend) ExecuteStream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error) {
    // ... existing arg building ...

    // Inject API key from agent_api_keys table if available
    if req.AgentID != "" {
        if provider, apiKey, err := loadAgentProviderKey(req.AgentID); err == nil && apiKey != "" {
            spec := model.ProviderRegistry[provider]
            if spec.EnvVar != "" {
                cmd.Env = append(os.Environ(), fmt.Sprintf("%s=%s", spec.EnvVar, apiKey))
            }
            args = append(args, "--provider", provider)
        }
    }

    // ... rest of ExecuteStream ...
}
```

Note: The **chat model** (e.g., `gpt-5.5`) is stored as `agent.PreferredModel` in the `agents` table. The **summarize model** (e.g., `gpt-4o-mini`) is stored in `config.yaml` under `summarize.model`. These are deliberately separate — users typically want a capable model for coding but a cheap model for summarization.

### Provider Registry: `internal/model/provider_registry.go`

A Go constant map resolves each built-in provider to its endpoints and API format. This data is sourced from Pi's `packages/ai/scripts/generate-models.ts`.

**Design: Two separate URL fields.** `ChatEndpoint` is the full URL passed directly to `OpenAISummarizer`/`AnthropicSummarizer` (they use it as-is in `http.NewRequestWithContext`). `ModelsEndpoint` is the full URL for the `/v1/models` (or equivalent) API call — these are NOT derived from each other via path manipulation, because different providers have inconsistent path structures.

```go
type KnownModel struct {
    ID               string `json:"id"`
    Name             string `json:"name"`
    ContextLength    int    `json:"context_length"`    // in tokens, 0 = unknown
    SupportsThinking bool   `json:"supports_thinking"` // whether model supports extended thinking
    CostTier         string `json:"cost_tier"`         // "cheap", "moderate", "expensive"
}

type ProviderSpec struct {
    ID             string       `json:"id"`
    Name           string       `json:"name"`
    EnvVar         string       `json:"envVar"`
    ChatEndpoint   string       `json:"-"` // full URL for summarize API calls (OpenAISummarizer/AnthropicSummarizer use as-is)
    ModelsEndpoint string       `json:"-"` // full URL for GET /v1/models or equivalent (may be "" for Anthropic-format providers)
    APIFormat      string       `json:"-"` // "openai" or "anthropic" — for summarize api.format
    KnownModels    []KnownModel `json:"-"` // fallback model list for providers without /v1/models (e.g., Anthropic)
    SupportsCLI    bool         `json:"-"` // true = Pi CLI can use this provider directly
    WizardReady    bool         `json:"-"` // true = can be configured via setup wizard (single API key field)
}

var ProviderRegistry = map[string]ProviderSpec{
    "openai":         {ID: "openai", Name: "OpenAI", EnvVar: "OPENAI_API_KEY",
        ChatEndpoint: "https://api.openai.com/v1/chat/completions", ModelsEndpoint: "https://api.openai.com/v1/models",
        APIFormat: "openai", SupportsCLI: true, WizardReady: true},
    "anthropic":      {ID: "anthropic", Name: "Anthropic", EnvVar: "ANTHROPIC_API_KEY",
        ChatEndpoint: "https://api.anthropic.com/v1/messages", ModelsEndpoint: "",
        APIFormat: "anthropic", KnownModels: []KnownModel{
            {ID: "claude-sonnet-4-20250514", Name: "Claude Sonnet 4", ContextLength: 200000, SupportsThinking: true, CostTier: "expensive"},
            {ID: "claude-3-7-sonnet-20250219", Name: "Claude 3.7 Sonnet", ContextLength: 200000, SupportsThinking: true, CostTier: "expensive"},
            {ID: "claude-3-5-haiku-20241022", Name: "Claude 3.5 Haiku", ContextLength: 200000, SupportsThinking: false, CostTier: "cheap"},
            {ID: "claude-3-5-sonnet-20241022", Name: "Claude 3.5 Sonnet", ContextLength: 200000, SupportsThinking: true, CostTier: "moderate"},
        }, SupportsCLI: true, WizardReady: true},
    "google":         {ID: "google", Name: "Google Gemini", EnvVar: "GEMINI_API_KEY",
        ChatEndpoint: "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions", ModelsEndpoint: "https://generativelanguage.googleapis.com/v1beta/openai/models",
        APIFormat: "openai", SupportsCLI: true, WizardReady: true},
    "deepseek":       {ID: "deepseek", Name: "DeepSeek", EnvVar: "DEEPSEEK_API_KEY",
        ChatEndpoint: "https://api.deepseek.com/v1/chat/completions", ModelsEndpoint: "https://api.deepseek.com/v1/models",
        APIFormat: "openai", SupportsCLI: true, WizardReady: true},
    "groq":           {ID: "groq", Name: "Groq", EnvVar: "GROQ_API_KEY",
        ChatEndpoint: "https://api.groq.com/openai/v1/chat/completions", ModelsEndpoint: "https://api.groq.com/openai/v1/models",
        APIFormat: "openai", SupportsCLI: true, WizardReady: true},
    "openrouter":     {ID: "openrouter", Name: "OpenRouter", EnvVar: "OPENROUTER_API_KEY",
        ChatEndpoint: "https://openrouter.ai/api/v1/chat/completions", ModelsEndpoint: "https://openrouter.ai/api/v1/models",
        APIFormat: "openai", SupportsCLI: true, WizardReady: true},
    "cerebras":       {ID: "cerebras", Name: "Cerebras", EnvVar: "CEREBRAS_API_KEY",
        ChatEndpoint: "https://api.cerebras.ai/v1/chat/completions", ModelsEndpoint: "https://api.cerebras.ai/v1/models",
        APIFormat: "openai", SupportsCLI: true, WizardReady: true},
    "xai":            {ID: "xai", Name: "xAI Grok", EnvVar: "XAI_API_KEY",
        ChatEndpoint: "https://api.x.ai/v1/chat/completions", ModelsEndpoint: "https://api.x.ai/v1/models",
        APIFormat: "openai", SupportsCLI: true, WizardReady: true},
    "mistral":        {ID: "mistral", Name: "Mistral", EnvVar: "MISTRAL_API_KEY",
        ChatEndpoint: "https://api.mistral.ai/v1/chat/completions", ModelsEndpoint: "https://api.mistral.ai/v1/models",
        APIFormat: "openai", SupportsCLI: true, WizardReady: true},
    "fireworks":      {ID: "fireworks", Name: "Fireworks", EnvVar: "FIREWORKS_API_KEY",
        ChatEndpoint: "https://api.fireworks.ai/inference/v1/messages", ModelsEndpoint: "",
        APIFormat: "anthropic", KnownModels: []KnownModel{
            {ID: "claude-sonnet-4-20250514", Name: "Claude Sonnet 4", ContextLength: 200000, SupportsThinking: true, CostTier: "expensive"},
            {ID: "claude-3-5-haiku-20241022", Name: "Claude 3.5 Haiku", ContextLength: 200000, SupportsThinking: false, CostTier: "cheap"},
        }, SupportsCLI: true, WizardReady: true},
    "minimax":        {ID: "minimax", Name: "MiniMax", EnvVar: "MINIMAX_API_KEY",
        ChatEndpoint: "https://api.minimax.io/anthropic/v1/messages", ModelsEndpoint: "",
        APIFormat: "anthropic", KnownModels: []KnownModel{
            {ID: "MiniMax-Text-01", Name: "MiniMax-Text-01", ContextLength: 0, SupportsThinking: false, CostTier: "cheap"},
        }, SupportsCLI: true, WizardReady: true},
    "minimax-cn":     {ID: "minimax-cn", Name: "MiniMax (China)", EnvVar: "MINIMAX_API_KEY",
        ChatEndpoint: "https://api.minimaxi.com/anthropic/v1/messages", ModelsEndpoint: "",
        APIFormat: "anthropic", KnownModels: []KnownModel{
            {ID: "MiniMax-Text-01", Name: "MiniMax-Text-01", ContextLength: 0, SupportsThinking: false, CostTier: "cheap"},
        }, SupportsCLI: true, WizardReady: true},
    "kimi-coding":    {ID: "kimi-coding", Name: "Kimi For Coding", EnvVar: "KIMI_API_KEY",
        ChatEndpoint: "https://api.kimi.com/coding/v1/messages", ModelsEndpoint: "",
        APIFormat: "anthropic", KnownModels: []KnownModel{
            {ID: "kimi-k2-0711-preview", Name: "Kimi K2", ContextLength: 131072, SupportsThinking: false, CostTier: "moderate"},
        }, SupportsCLI: true, WizardReady: true},
    "moonshotai":     {ID: "moonshotai", Name: "Moonshot AI", EnvVar: "MOONSHOT_API_KEY",
        ChatEndpoint: "https://api.moonshot.ai/v1/chat/completions", ModelsEndpoint: "https://api.moonshot.ai/v1/models",
        APIFormat: "openai", SupportsCLI: true, WizardReady: true},
    "moonshotai-cn":  {ID: "moonshotai-cn", Name: "Moonshot AI (China)", EnvVar: "MOONSHOT_API_KEY",
        ChatEndpoint: "https://api.moonshot.cn/v1/chat/completions", ModelsEndpoint: "https://api.moonshot.cn/v1/models",
        APIFormat: "openai", SupportsCLI: true, WizardReady: true},
    "xiaomi":         {ID: "xiaomi", Name: "Xiaomi MiMo", EnvVar: "XIAOMI_API_KEY",
        ChatEndpoint: "https://api.xiaomimimo.com/v1/chat/completions", ModelsEndpoint: "https://api.xiaomimimo.com/v1/models",
        APIFormat: "openai", SupportsCLI: true, WizardReady: true},
    "xiaomi-token-plan-cn":  {ID: "xiaomi-token-plan-cn", Name: "Xiaomi MiMo Token Plan (China)", EnvVar: "XIAOMI_TOKEN_PLAN_CN_API_KEY",
        ChatEndpoint: "https://token-plan-cn.xiaomimimo.com/v1/chat/completions", ModelsEndpoint: "https://token-plan-cn.xiaomimimo.com/v1/models",
        APIFormat: "openai", SupportsCLI: true, WizardReady: true},
    "xiaomi-token-plan-ams": {ID: "xiaomi-token-plan-ams", Name: "Xiaomi MiMo Token Plan (Amsterdam)", EnvVar: "XIAOMI_TOKEN_PLAN_AMS_API_KEY",
        ChatEndpoint: "https://token-plan-ams.xiaomimimo.com/v1/chat/completions", ModelsEndpoint: "https://token-plan-ams.xiaomimimo.com/v1/models",
        APIFormat: "openai", SupportsCLI: true, WizardReady: true},
    "xiaomi-token-plan-sgp": {ID: "xiaomi-token-plan-sgp", Name: "Xiaomi MiMo Token Plan (Singapore)", EnvVar: "XIAOMI_TOKEN_PLAN_SGP_API_KEY",
        ChatEndpoint: "https://token-plan-sgp.xiaomimimo.com/v1/chat/completions", ModelsEndpoint: "https://token-plan-sgp.xiaomimimo.com/v1/models",
        APIFormat: "openai", SupportsCLI: true, WizardReady: true},
    "zai":            {ID: "zai", Name: "ZAI", EnvVar: "ZAI_API_KEY",
        ChatEndpoint: "https://api.z.ai/api/coding/paas/v4/chat/completions", ModelsEndpoint: "https://api.z.ai/api/coding/paas/v4/models",
        APIFormat: "openai", SupportsCLI: true, WizardReady: true},
    "huggingface":    {ID: "huggingface", Name: "Hugging Face", EnvVar: "HF_API_KEY",
        ChatEndpoint: "https://router.huggingface.co/v1/chat/completions", ModelsEndpoint: "https://router.huggingface.co/v1/models",
        APIFormat: "openai", SupportsCLI: true, WizardReady: true},
    "opencode":       {ID: "opencode", Name: "OpenCode Zen", EnvVar: "OPENCODE_API_KEY",
        ChatEndpoint: "https://opencode.ai/zen/chat/completions", ModelsEndpoint: "https://opencode.ai/zen/models",
        APIFormat: "openai", SupportsCLI: true, WizardReady: true},
    "vercel-ai-gateway": {ID: "vercel-ai-gateway", Name: "Vercel AI Gateway", EnvVar: "AI_GATEWAY_API_KEY",
        ChatEndpoint: "https://ai-gateway.vercel.sh/v1/messages", ModelsEndpoint: "",
        APIFormat: "anthropic", KnownModels: []KnownModel{
            {ID: "claude-sonnet-4-20250514", Name: "Claude Sonnet 4", ContextLength: 200000, SupportsThinking: true, CostTier: "expensive"},
            {ID: "claude-3-5-haiku-20241022", Name: "Claude 3.5 Haiku", ContextLength: 200000, SupportsThinking: false, CostTier: "cheap"},
        }, SupportsCLI: true, WizardReady: true},
    // Enterprise providers — cannot be configured via wizard (multi-field auth); Pi CLI handles them natively
    "amazon-bedrock":           {ID: "amazon-bedrock", Name: "Amazon Bedrock", EnvVar: "",
        ChatEndpoint: "", ModelsEndpoint: "", APIFormat: "", SupportsCLI: true, WizardReady: false},
    "azure-openai-responses":   {ID: "azure-openai-responses", Name: "Azure OpenAI Responses", EnvVar: "AZURE_OPENAI_API_KEY",
        ChatEndpoint: "", ModelsEndpoint: "", APIFormat: "", SupportsCLI: true, WizardReady: false},
    "cloudflare-ai-gateway":    {ID: "cloudflare-ai-gateway", Name: "Cloudflare AI Gateway", EnvVar: "CLOUDFLARE_API_KEY",
        ChatEndpoint: "", ModelsEndpoint: "", APIFormat: "", SupportsCLI: true, WizardReady: false},
    "cloudflare-workers-ai":    {ID: "cloudflare-workers-ai", Name: "Cloudflare Workers AI", EnvVar: "CLOUDFLARE_API_KEY",
        ChatEndpoint: "", ModelsEndpoint: "", APIFormat: "", SupportsCLI: true, WizardReady: false},
    "google-vertex":            {ID: "google-vertex", Name: "Google Vertex AI", EnvVar: "",
        ChatEndpoint: "", ModelsEndpoint: "", APIFormat: "", SupportsCLI: true, WizardReady: false},
}
```

**Provider categories for summarize auto-configuration:**

| Category | Providers | Summarize path |
|----------|-----------|----------------|
| **OpenAI-compatible** | openai, deepseek, groq, cerebras, openrouter, xai, mistral, moonshotai, moonshotai-cn, xiaomi, zai, huggingface, opencode, google, xiaomi-token-plan-* | `OpenAISummarizer` — POST to `{ChatEndpoint}` |
| **Anthropic-compatible** | anthropic, fireworks, minimax, minimax-cn, kimi-coding, vercel-ai-gateway | `AnthropicSummarizer` — POST to `{ChatEndpoint}` |
| **Enterprise** | amazon-bedrock, azure-openai-responses, cloudflare-*, google-vertex | `ChatEndpoint=""` → **fallback to Pi CLI** (`AIBackendSummarizer`) for summarize; these providers require complex auth (AWS SigV4, Azure tokens) that doesn't fit simple HTTP. **WizardReady=false** — excluded from provider selection in wizard. |
| **Custom URL** | (user input) | `OpenAISummarizer` by default; user can switch format to `anthropic` in settings |

**Google provider note:** Uses Gemini's OpenAI-compatible endpoint (`/v1beta/openai/chat/completions`) so `OpenAISummarizer` works without a new implementation.

Response:
```json
{
  "success": true,
  "agent": { "id": "openai", "name": "OpenAI", ... },
  "default_agent_id": "openai"
}
```

## Database Schema

### New Table: `agents`

```sql
CREATE TABLE IF NOT EXISTS agents (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    icon TEXT NOT NULL DEFAULT '',
    specialty TEXT NOT NULL DEFAULT '',
    backend TEXT NOT NULL,
    command TEXT NOT NULL DEFAULT '',
    thinking_effort TEXT NOT NULL DEFAULT '',
    thinking_effort_levels TEXT NOT NULL DEFAULT '[]',
    preferred_model TEXT NOT NULL DEFAULT '',
    preferred_thinking_effort TEXT NOT NULL DEFAULT '',
    system_prompt TEXT NOT NULL DEFAULT '',
    models TEXT NOT NULL DEFAULT '[]',
    models_auto_detected INTEGER NOT NULL DEFAULT 0,
    source TEXT NOT NULL DEFAULT 'auto',
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_agents_backend ON agents(backend);
CREATE INDEX IF NOT EXISTS idx_agents_source ON agents(source);
CREATE INDEX IF NOT EXISTS idx_agents_sort ON agents(sort_order);
```

**Field notes:**
- `models`: JSON array `[{"id":"...","name":"...","default":true}]` — always read/written atomically with agent
- `thinking_effort_levels`: JSON array `["low","medium","high"]` — from BackendRegistry, not user-editable
- `source`: `"auto"` (CLI detected), `"setup"` (wizard created), `"manual"` (manually added)
- `command`: embedded Pi binary path (absolute), or custom CLI path
- `models_auto_detected`: 0=user-defined models, 1=auto-discovered (used by AsyncRefreshCache)
- `sort_order`: determines display order in agent list; auto-detected agents get 0, wizard-created get 0; user can reorder later
- SQLite booleans: `INTEGER` where 0=false, 1=true (standard SQLite convention)

**No unique constraint on `backend`**: Multiple agents can share the same backend (e.g., two Pi agents with different models/providers). Uniqueness is on `id` (PK).

### New Table: `agent_api_keys`

```sql
CREATE TABLE IF NOT EXISTS agent_api_keys (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_id TEXT NOT NULL,
    provider TEXT NOT NULL,
    custom_url TEXT NOT NULL DEFAULT '',
    encrypted_key TEXT NOT NULL,           -- AES-256-GCM encrypted, NOT plaintext
    key_nonce TEXT NOT NULL,               -- Random nonce for decryption
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_api_keys_agent_provider
    ON agent_api_keys(agent_id, provider);
```

**API Key Encryption:**
- API keys are **encrypted at rest** using AES-256-GCM with a server-side encryption key
- Encryption key source: derived from the ClawBench auto-password (stored in `.clawbench/auto-password`) via HKDF-SHA256
- Rationale: the auto-password is already the authentication secret for the web UI; deriving the encryption key from it means API keys are only as secure as the login password (which is the existing threat model). If a user sets a custom password, the encryption key is re-derived.
- `encrypted_key`: base64-encoded ciphertext
- `key_nonce`: base64-encoded random nonce (generated per encryption operation)
- Decryption happens at runtime when Pi needs the API key passed via `--api-key` flag or env var

**Fallback for headless/CLI mode:** When no password is set (dev mode), encryption uses a hardcoded derivation key. This is acceptable because dev mode already implies localhost-only access.

**Encryption key rotation on password change (I4 fix):** When a user changes their password via `POST /api/settings/password`, the encryption key changes (it's derived from the auto-password). All existing encrypted API keys become undecryptable. The password change handler must:

1. **Before** updating the password file: decrypt all API keys using the OLD key
2. **Update** the password file
3. **Re-encrypt** all API keys using the NEW key (derived from new password)
4. **Clear** `encryptionKeyCache` so the next operation uses the new key
5. If any step fails, rollback the password change and return an error

```go
func RotateAPIKeyEncryption(db *sql.DB, oldPassword, newPassword string) error {
    // 1. Load all keys with old encryption key
    keys, err := loadAllAPIKeys(db) // decrypts with current (old) key
    if err != nil { return err }

    // 2. Update password file
    if err := updatePasswordFile(newPassword); err != nil { return err }

    // 3. Re-encrypt with new key
    ResetEncryptionKeyCache() // force re-derivation
    for _, k := range keys {
        if err := SaveAgentAPIKey(db, k.AgentID, k.Provider, k.CustomURL, k.PlaintextKey); err != nil {
            // CRITICAL: password updated but keys re-encryption failed
            // Attempt rollback
            updatePasswordFile(oldPassword)
            ResetEncryptionKeyCache()
            return fmt.Errorf("key rotation failed: %w (password rolled back)", err)
        }
    }
    return nil
}
```

This function is called from the password change handler in `internal/handler/settings.go` after verifying the current password.

### CRUD Operations (`internal/service/agent_store.go`)

```go
func LoadAgentsFromDB(db *sql.DB) ([]*model.Agent, error)
func SaveAgent(db *sql.DB, agent *model.Agent) error         // upsert
func DeleteAgent(db *sql.DB, id string) error                // cascades to agent_api_keys
func PatchAgent(db *sql.DB, id, preferredModel, preferredThinkingEffort string) error
func SaveAgentAPIKey(db *sql.DB, agentID, provider, customURL, apiKey string) error  // encrypts before storing
func LoadAgentAPIKey(db *sql.DB, agentID, provider string) (customURL, apiKey string, err error)  // decrypts on read
func RotateAPIKeyEncryption(db *sql.DB, oldPassword, newPassword string) error       // re-encrypt all keys on password change
func loadAllAPIKeys(db *sql.DB) ([]DecryptedAPIKey, error)                           // internal: for key rotation
```

Encryption helpers (`internal/service/crypto.go`):
```go
func EncryptAPIKey(plaintext string) (encrypted, nonce string, err error)
func DecryptAPIKey(encrypted, nonce string) (string, error)
func deriveEncryptionKey() []byte  // HKDF-SHA256 from auto-password
```

## YAML → Database Migration

One-time migration in `initDB()`, after table creation. **Wrapped in a database transaction** to ensure atomicity — either all YAML agents migrate or none do.

```go
func migrateAgentsFromYAML(db *sql.DB, agentsDir string) {
    var count int
    db.QueryRow("SELECT COUNT(*) FROM agents").Scan(&count)
    if count > 0 {
        return // already migrated
    }

    entries, err := os.ReadDir(agentsDir)
    if err != nil {
        return // directory doesn't exist, nothing to migrate
    }

    // Collect agents from YAML files first
    var agents []*model.Agent
    for _, entry := range entries {
        if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
            continue
        }
        data, err := os.ReadFile(filepath.Join(agentsDir, entry.Name()))
        if err != nil {
            slog.Warn("YAML migration: failed to read file", "file", entry.Name(), "error", err)
            continue
        }
        var agent model.Agent
        if err := yaml.Unmarshal(data, &agent); err != nil || agent.ID == "" {
            slog.Warn("YAML migration: invalid YAML", "file", entry.Name(), "error", err)
            continue
        }
        agent.Source = "auto"
        agents = append(agents, &agent)
    }

    if len(agents) == 0 {
        return // nothing to migrate
    }

    // Transactional write: all-or-nothing
    tx, err := db.BeginTx(context.Background(), nil)
    if err != nil {
        slog.Error("YAML migration: failed to begin transaction", "error", err)
        return
    }
    defer tx.Rollback()

    for _, agent := range agents {
        if err := saveAgentTx(tx, agent); err != nil {
            slog.Error("YAML migration: failed to save agent", "id", agent.ID, "error", err)
            return // tx.Rollback() via defer
        }
    }

    if err := tx.Commit(); err != nil {
        slog.Error("YAML migration: failed to commit", "error", err)
        return
    }

    slog.Info("YAML migration completed", "agents", len(agents))
}
```

After migration, YAML loading code is permanently removed. YAML files remain on disk but are never read again.

### BuildCommonPrompt After YAML Removal

Currently `BuildCommonPrompt()` calls `loadRules(agentsDir)` which reads `config/rules.md` from the parent of `agentsDir`. After removing YAML-based agent loading, `agentsDir` no longer exists as a variable. Fix: `BuildCommonPrompt()` accepts an explicit `configDir` parameter (or reads from `filepath.Dir(exePath) + "/config/"`), decoupled from agent storage.

```go
// Before: loadRules(agentsDir) → reads config/rules.md from parent of agentsDir
// After:  loadRules(configDir)  → reads config/rules.md directly from configDir
var ConfigDir string  // set once at startup from filepath.Dir(exePath) + "/config"
```

## Startup Flow

```
1. initDB(dbPath)                          — Create tables + YAML migration
2. model.LoadAgentsFromDB(db)              — Load from database
3. present := model.SyncDiscoverAgents(db) — Detect PATH CLIs + embedded binary → write new agents to DB
4. model.LoadAgentsFromDB(db)              — Reload (step 3 may have added agents)
5. model.SyncDiscoverModels(cacheDir)      — First-run model discovery (if no cache)
6. model.MergeDiscoveredData(db, cacheDir, present) — Fill models/levels, soft-delete missing CLIs
7. model.AsyncRefreshModelCache(cacheDir, db)       — Background model refresh
```

### SyncDiscoverAgents (revised)

```
For each BackendRegistry entry:
  1. CheckCLIExists(spec.DefaultCmd) → PATH detection
  2. If backend="pi": also check EmbeddedAgentPath()
  3. If CLI or embedded exists AND no DB record for this backend → SaveAgent() with source="auto"
  4. If DB record already exists → skip (preserve user customizations)
  5. Return present map (PATH + embedded)
```

### MergeDiscoveredData (revised)

```
1. Soft-delete: DELETE FROM agents WHERE backend NOT IN (present) AND source = 'auto'
   - source='setup' and source='manual' agents are never deleted
   - Wizard-created agents survive even if CLI is uninstalled

2. Fill ThinkingEffortLevels from BackendRegistry → SaveAgent()

3. Fill Models from cache for agents with models_auto_detected=1 → SaveAgent()

4. Set CanRefreshModels from BackendRegistry (runtime only, not persisted)

5. LoadAgentsFromDB() to refresh in-memory state
```

### Embedded Pi Detection

```go
func EmbeddedAgentPath() string {
    exePath, err := os.Executable()
    if err != nil {
        slog.Error("failed to get executable path", "error", err)
        return ""
    }
    baseDir := filepath.Dir(exePath)
    for _, name := range []string{"pi", "pi.exe"} {
        p := filepath.Join(baseDir, ".clawbench", "pi", name)
        if info, err := os.Stat(p); err == nil && !info.IsDir() {
            return p
        }
    }
    return ""
}

// EmbeddedAgentVersion extracts the version from the embedded Pi binary.
func EmbeddedAgentVersion() string {
    piPath := EmbeddedAgentPath()
    if piPath == "" {
        return ""
    }
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    out, err := exec.CommandContext(ctx, piPath, "--version").Output()
    if err != nil {
        return ""
    }
    return strings.TrimSpace(string(out))
}
```

## Build Integration

### build.sh Changes

New step 1.5 between Go build and Vue build:

```bash
PI_VERSION="${PI_VERSION:-0.78.0}"
PI_DIR=".clawbench/pi"

# Determine platform
if [ -n "$TARGET_OS" ] && [ -n "$TARGET_ARCH" ]; then
    case "$TARGET_OS" in
        linux)   PI_PLATFORM="linux-$TARGET_ARCH" ;;
        darwin)  PI_PLATFORM="darwin-$TARGET_ARCH" ;;
        windows) PI_PLATFORM="windows-$TARGET_ARCH" ;;
    esac
else
    PI_PLATFORM="$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m)"
    PI_PLATFORM="${PI_PLATFORM/x86_64/x64}"
    PI_PLATFORM="${PI_PLATFORM/aarch64/arm64}"
fi

PI_EXT="tar.gz"
[ "$TARGET_OS" = "windows" ] && PI_EXT="zip"

PI_ARCHIVE="pi-${PI_PLATFORM}.${PI_EXT}"
PI_URL="https://github.com/earendil-works/pi/releases/download/v${PI_VERSION}/${PI_ARCHIVE}"

mkdir -p "$PI_DIR"
curl -sL "$PI_URL" | tar xzf - -C "$PI_DIR" --strip-components=1
chmod +x "$PI_DIR/pi" 2>/dev/null || true

# Record embedded version for runtime detection
echo -n "$PI_VERSION" > "$PI_DIR/VERSION"
```

### Pi Version at Runtime

`EmbeddedAgentVersion()` first checks `.clawbench/pi/VERSION` file (fast, no subprocess). Falls back to `pi --version` (subprocess) if VERSION file missing (e.g., older builds).

### Release Package Structure

```
clawbench                     # Go binary
public/                       # Vue frontend
config/                       # rules.md (agents/ dir kept for migration only)
.clawbench/pi/pi              # Embedded Pi binary (~46MB)
.clawbench/pi/VERSION         # Pi version string (e.g., "0.78.0")
.clawbench/pi/export-html/    # Pi's export-html module
```

## Frontend Components

### New Files

```
web/src/components/
  SetupWizard.vue           — Fullscreen wizard container (step management + progress)
  SetupWelcome.vue          — Step 1: Welcome page
  SetupProvider.vue         — Step 2: Provider selection
  SetupApiKey.vue           — Step 3: API key input
  SetupModelVerify.vue      — Step 4: Model selection + verification
  SetupAgentName.vue        — Step 5: Agent name & ID confirmation

web/src/composables/
  useSetup.ts               — Setup API calls (status, providers, models, verify, complete)
```

### App.vue Changes

```vue
<!-- After login, before main UI -->
<SetupWizard v-if="needsSetup && embeddedAgent" @complete="handleSetupComplete" />

<!-- No agents and no embedded binary -->
<div v-else-if="needsSetup" class="setup-required">
  未检测到智能体。请安装智能体后重新启动服务。
</div>

<!-- Normal main UI -->
<div v-else class="app-container" ...>
```

### `useSetup.ts` Composable

```typescript
export function useSetup() {
  const setupStatus = ref<{ needs_setup: boolean; embedded_agent: boolean; agent_version: string }>()

  async function checkStatus() { ... }
  async function getProviders() { ... }
  async function scanModels(provider: string, customUrl: string, apiKey: string) { ... }
  async function verify(provider: string, customUrl: string, apiKey: string, model: string) { ... }
  async function complete(config: SetupCompleteRequest) { ... }

  return { setupStatus, checkStatus, getProviders, scanModels, verify, complete }
}
```

### Provider → Agent Name Auto-generation

```typescript
const providerAgentNames: Record<string, { name: string; id: string }> = {
  'anthropic':                { name: 'Anthropic Claude', id: 'anthropic-claude' },
  'openai':                   { name: 'OpenAI',           id: 'openai' },
  'google':                   { name: 'Google Gemini',    id: 'google-gemini' },
  'deepseek':                 { name: 'DeepSeek',         id: 'deepseek' },
  'minimax':                  { name: 'MiniMax',          id: 'minimax' },
  'minimax-cn':               { name: 'MiniMax (中国)',    id: 'minimax-cn' },
  'groq':                     { name: 'Groq',             id: 'groq' },
  'openrouter':               { name: 'OpenRouter',       id: 'openrouter' },
  'mistral':                  { name: 'Mistral',          id: 'mistral' },
  'xai':                      { name: 'xAI Grok',         id: 'xai-grok' },
  'cerebras':                 { name: 'Cerebras',         id: 'cerebras' },
  'fireworks':                { name: 'Fireworks',        id: 'fireworks' },
  'moonshotai':               { name: 'Moonshot AI',      id: 'moonshot-ai' },
  'moonshotai-cn':            { name: 'Moonshot AI (中国)', id: 'moonshot-ai-cn' },
  'opencode':                 { name: 'OpenCode Zen',     id: 'opencode-zen' },
  'kimi-coding':              { name: 'Kimi For Coding',  id: 'kimi-coding' },
  'zai':                      { name: 'ZAI',              id: 'zai' },
  'huggingface':              { name: 'Hugging Face',     id: 'huggingface' },
  'vercel-ai-gateway':        { name: 'Vercel AI GW',     id: 'vercel-ai-gw' },
  'xiaomi':                   { name: 'Xiaomi MiMo',      id: 'xiaomi-mimo' },
  'xiaomi-token-plan-cn':     { name: 'Xiaomi MiMo (CN)', id: 'xiaomi-mimo-cn' },
  'xiaomi-token-plan-ams':    { name: 'Xiaomi MiMo (AMS)',id: 'xiaomi-mimo-ams' },
  'xiaomi-token-plan-sgp':    { name: 'Xiaomi MiMo (SGP)',id: 'xiaomi-mimo-sgp' },
  '_custom':                  { name: '自定义智能体',      id: 'custom-agent' },
}
```

## `GET /api/agents` Response Change

```json
{
  "agents": [...],
  "default_agent_id": "pi",
  "needs_setup": false,
  "embedded_agent": true
}
```

## Code to Remove

| File | Function/Variable | Reason |
|------|-------------------|--------|
| `internal/model/agent.go` | `LoadAgents(dir)` | Replaced by `LoadAgentsFromDB(db)` |
| `internal/model/agent.go` | `WriteAgentYAML(agent)` | Replaced by `PatchAgent(db, ...)` |
| `internal/model/agent.go` | `agentsDir` variable | Replaced by `ConfigDir` |
| `internal/model/agent.go` | `loadRules(agentsDir)` | Changed to `loadRules(ConfigDir)` |
| `internal/model/discovery.go` | `GenerateAgentYAML(spec)` | Replaced by `SaveAgent(db, agent)` |
| `internal/model/discovery.go` | `DiscoverAgents(dir)` | Replaced by DB-based `SyncDiscoverAgents(db)` |
| `internal/model/discovery.go` | YAML write logic in `SyncDiscoverAgents` | Replaced by DB writes |

**Note:** `BuildCommonPrompt()` and `loadRules()` are NOT removed — they are refactored to use `ConfigDir` instead of `agentsDir` parent. `config/rules.md` is still read from the config directory.

## Error Handling

| Scenario | Handling |
|----------|----------|
| Database unavailable | Startup failure, log error and exit |
| YAML migration failure (single file) | Skip that YAML, continue with rest, log warning. Transaction ensures all-or-nothing for the batch. |
| YAML migration failure (transaction commit) | Roll back, log error. Agents table stays empty. On next startup, migration retries. |
| Embedded Pi missing + no PATH | `needs_setup=true, embedded_agent=false` → restart prompt only |
| `--list-models` scan failure | Return empty model list + error message, prompt to check API key |
| Verify message timeout (30s) | Return `success: false` + timeout message, allow retry |
| Verify API key invalid | Return `success: false` + specific error (401/403), prompt to go back |
| Setup complete DB write failure | Transaction rollback, return 500, prompt retry |
| Setup complete duplicate request | `sync.Mutex` guard, return 409 Conflict |
| Pi auth.json write failure | Log warning, don't block (Pi can also use `--api-key` flag or env var) |
| API key encryption failure | Log error, return 500 (never store plaintext as fallback) |
| API key decryption failure | Log error, treat as missing key (user must re-enter) |
| API key rotation failure on password change | Rollback password update, return error to user, log critical warning |
| `/v1/models` endpoint failure | Return empty model list + error message; for KnownModels providers, always succeeds (hardcoded) |
| Custom URL `/models` derivation failure | Return `{"models": [], "error": "..."}`, frontend shows manual model ID input |
| Summarize backend re-init after wizard | If decryption fails during re-init, fall back to `SimpleSummarizer` with logged warning |
| Pi CLI env var injection failure | If `LoadAgentAPIKey` returns error, Pi CLI falls back to `auth.json` config (degraded but functional) |
| Page refresh during wizard | State restored from sessionStorage (step, provider, model). API key NOT persisted — user must re-enter. |

## Upgrade Path (Existing Users)

1. User upgrades to new version → `initDB()` creates `agents` + `agent_api_keys` tables
2. Migration detects empty `agents` table → reads `config/agents/*.yaml` → writes to DB in transaction
3. YAML files remain on disk but are never read again (can be manually cleaned up later)
4. Existing `chat_sessions.agent_id` references continue to work — agent IDs are preserved in migration
5. If user had a `pi.yaml` with custom settings (preferred_model, system_prompt), those are migrated faithfully
6. After first run with new version, `config/agents/` directory is no longer needed

## Pi Provider List (Complete, from source)

Source: Pi `provider-display-names.js` + `--help` env vars

| ID | Display Name | Env Var |
|----|-------------|---------|
| anthropic | Anthropic | ANTHROPIC_API_KEY |
| amazon-bedrock | Amazon Bedrock | AWS_PROFILE / AWS_ACCESS_KEY_ID |
| azure-openai-responses | Azure OpenAI Responses | AZURE_OPENAI_API_KEY |
| cerebras | Cerebras | CEREBRAS_API_KEY |
| cloudflare-ai-gateway | Cloudflare AI Gateway | CLOUDFLARE_API_KEY + CLOUDFLARE_ACCOUNT_ID + CLOUDFLARE_GATEWAY_ID |
| cloudflare-workers-ai | Cloudflare Workers AI | CLOUDFLARE_API_KEY + CLOUDFLARE_ACCOUNT_ID |
| deepseek | DeepSeek | DEEPSEEK_API_KEY |
| fireworks | Fireworks | FIREWORKS_API_KEY |
| google | Google Gemini | GEMINI_API_KEY |
| google-vertex | Google Vertex AI | (service account) |
| groq | Groq | GROQ_API_KEY |
| huggingface | Hugging Face | (HF token) |
| kimi-coding | Kimi For Coding | KIMI_API_KEY |
| minimax | MiniMax | MINIMAX_API_KEY |
| minimax-cn | MiniMax (China) | MINIMAX_API_KEY |
| mistral | Mistral | MISTRAL_API_KEY |
| moonshotai | Moonshot AI | MOONSHOT_API_KEY |
| moonshotai-cn | Moonshot AI (China) | MOONSHOT_API_KEY |
| opencode | OpenCode Zen | OPENCODE_API_KEY |
| openai | OpenAI | OPENAI_API_KEY |
| openrouter | OpenRouter | OPENROUTER_API_KEY |
| vercel-ai-gateway | Vercel AI Gateway | AI_GATEWAY_API_KEY |
| xai | xAI Grok | XAI_API_KEY |
| zai | ZAI | ZAI_API_KEY |
| xiaomi | Xiaomi MiMo | XIAOMI_API_KEY |
| xiaomi-token-plan-cn | Xiaomi MiMo Token Plan (China) | XIAOMI_TOKEN_PLAN_CN_API_KEY |
| xiaomi-token-plan-ams | Xiaomi MiMo Token Plan (Amsterdam) | XIAOMI_TOKEN_PLAN_AMS_API_KEY |
| xiaomi-token-plan-sgp | Xiaomi MiMo Token Plan (Singapore) | XIAOMI_TOKEN_PLAN_SGP_API_KEY |

## Pi Binary Releases

| Platform | File | Size |
|----------|------|------|
| Linux x64 | `pi-linux-x64.tar.gz` | ~46MB |
| Linux arm64 | `pi-linux-arm64.tar.gz` | ~46MB |
| macOS arm64 | `pi-darwin-arm64.tar.gz` | ~29MB |
| macOS x64 | `pi-darwin-x64.tar.gz` | ~31MB |
| Windows x64 | `pi-windows-x64.zip` | ~48MB |
| Windows arm64 | `pi-windows-arm64.zip` | ~46MB |

Archive structure: `pi/pi` (binary) + `pi/export-html/` (HTML export module)
