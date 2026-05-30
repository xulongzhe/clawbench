[中文](FAQ.md) | [English](FAQ.en.md)

# Frequently Asked Questions (FAQ)

**Q: Which operating systems does ClawBench support?**

A: Linux (x86_64 / ARM64) and Windows (x86_64) are supported. The backend is written in Go and the frontend is a standard web application, enabling cross-platform operation.

**Q: Which AI backends are supported?**

A: Nine CLI backends are supported: CodeBuddy, Claude Code, OpenCode, Gemini CLI, Codex, Qoder CLI, VeCLI, DeepSeek TUI, and Pi. You can switch between them in real time via the Web UI, with isolated session data. Just make sure the corresponding CLI is installed and available in your PATH.

**Q: How do I add a new agent?**

A: Create a YAML file in the `config/agents/` directory, defining id, name, icon, specialty, backend, model, and system_prompt. Common rules go in `config/rules.md`, which is automatically injected into all agents' system prompts. The `{{AVAILABLE_AGENTS}}` placeholder is automatically replaced with the list of available agents.

**Q: Do I need to configure an API Key?**

A: No. ClawBench implements AI functionality by calling local CLIs (CodeBuddy, Claude Code, OpenCode, Gemini CLI, Codex, Qoder CLI, VeCLI, DeepSeek TUI, or Pi), which already handle API Key configuration and management.

**Q: Can TTS speech synthesis use local models?**

A: Yes. Set `summarize.backend` to `"api"` and configure the Ollama OpenAI-compatible endpoint to use a local Ollama service for text summarization without any cloud API. Just install Ollama and pull a model (e.g., `ollama pull gemma3:270m`), then configure:

```yaml
summarize:
  backend: "api"
  model: "gemma3:270m"
  api:
    base_url: "http://localhost:11434/v1/chat/completions"
    format: "openai"
```

The TTS engine itself also supports local offline solutions (piper / kokoro / moss-nano). Combining both enables fully offline speech playback. Among these, moss-nano supports multiple languages and voice cloning with 48kHz high-quality output.

**Q: Can I run multiple ClawBench instances simultaneously?**

A: Yes. Copy the entire release directory to a different location — each copy gets its own `BinDir`, config, and `.clawbench/` data directory for complete isolation. Just configure different ports in each copy's `config/config.yaml`.

**Q: Do I need a config file to start?**

A: No. All configuration options have default values, so you can start without `config/config.yaml`. When `password` is not configured, a random password is auto-generated and saved to `.clawbench/auto-password`; the startup script will display it. To customize, copy `config/config.example.yaml` to `config/config.yaml` and modify as needed.

**Q: What if I forget the auto-generated password?**

A: Check the `.clawbench/auto-password` file to retrieve the password. You can also set `password` in `config/config.yaml` to use a fixed password.

**Q: Where is data stored?**

A: Data is stored in the `.clawbench/` directory alongside the binary, including the database file (`ClawBench.db`), log files (`logs/`), and auto-password (`auto-password`). Uploaded files are stored in `.clawbench/uploads/` within the project directory. It's a green portable deployment — deleting the program directory completely uninstalls everything.

**Q: How do I back up data?**

A: Back up the `.clawbench/ClawBench.db` database file in the directory alongside the binary.

**Q: What if I don't have any AI CLI installed on first launch?**

A: If the release package includes the embedded Pi agent (or you built with `./build.sh --with-pi`), a setup wizard appears automatically on first launch. The wizard guides you through selecting an LLM provider (23 supported including OpenAI, Anthropic, DeepSeek, etc.), entering your API key, verifying model connectivity, and naming your agent. API keys are encrypted with AES-256-GCM and encryption keys auto-rotate on password change.

**Q: What's the difference between wizard-created agents and YAML-configured agents?**

A: Wizard-created agents are stored in the database (`agents` table), while YAML-configured agents are stored in the `config/agents/` directory. When both exist, database agents take precedence on ID collision. Wizard agent API keys are encrypted and stored in the `agent_api_keys` table; YAML agent API keys are managed by the respective CLI tools.
