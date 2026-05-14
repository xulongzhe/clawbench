[中文](FAQ.md) | [English](FAQ.en.md)

# Frequently Asked Questions (FAQ)

**Q: Which operating systems does ClawBench support?**

A: Linux (x86_64 / ARM64) and Windows (x86_64) are supported. The backend is written in Go and the frontend is a standard web application, enabling cross-platform operation.

**Q: Which AI backends are supported?**

A: Seven CLI backends are supported: CodeBuddy, Claude Code, OpenCode, Gemini CLI, Codex, Qoder CLI, and VeCLI. You can switch between them in real time via the Web UI, with isolated session data. Just make sure the corresponding CLI is installed and available in your PATH.

**Q: How do I add a new agent?**

A: Create a YAML file in the `config/agents/` directory, defining id, name, icon, specialty, backend, model, and system_prompt. Common rules go in `config/rules.md`, which is automatically injected into all agents' system prompts. The `{{AVAILABLE_AGENTS}}` placeholder is automatically replaced with the list of available agents.

**Q: Do I need to configure an API Key?**

A: No. ClawBench implements AI functionality by calling local CLIs (CodeBuddy, Claude Code, OpenCode, Gemini CLI, Codex, Qoder CLI, or VeCLI), which already handle API Key configuration and management.

**Q: Can TTS speech synthesis use local models?**

A: Yes. Set `summarize_backend` to `"api"` and configure the Ollama OpenAI-compatible endpoint to use a local Ollama service for text summarization without any cloud API. Just install Ollama and pull a model (e.g., `ollama pull gemma3:270m`), then configure:

```yaml
tts:
  summarize_backend: "api"
  api:
    base_url: "http://localhost:11434/v1/chat/completions"
    format: "openai"
    model: "gemma3:270m"
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
