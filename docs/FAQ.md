[中文](FAQ.md) | [English](FAQ.en.md)

# 常见问题（FAQ）

**Q: ClawBench 支持哪些操作系统？**

A: 支持 Linux（x86_64 / ARM64）和 Windows（x86_64）。后端使用 Go 编写，前端为标准 Web 应用，可跨平台运行。

**Q: 支持哪些 AI 后端？**

A: 支持 CodeBuddy、Claude Code、OpenCode、Gemini CLI、Codex 五种 CLI 后端。可在 Web UI 中实时切换，会话数据隔离。只需确保对应 CLI 已安装并在 PATH 中可用。

**Q: 如何添加新的智能体？**

A: 在 `config/agents/` 目录下创建 YAML 文件，定义 id、name、icon、specialty、backend、model 和 system_prompt 即可。公共提示词放在 `config/agent_common_prompt.md`，会自动注入到所有智能体。`{{AVAILABLE_AGENTS}}` 占位符会自动替换为可用智能体列表。

**Q: 是否需要配置 API Key？**

A: 不需要。ClawBench 通过调用本地 CLI（CodeBuddy、Claude Code、OpenCode、Gemini CLI 或 Codex）实现 AI 功能，这些 CLI 工具已经完成了 API Key 的配置和管理。

**Q: TTS 语音合成可以使用本地模型吗？**

A: 可以。将 `summarize_backend` 设为 `"ollama"` 即可使用本地 Ollama 服务进行文本总结，无需任何云 API。只需安装 Ollama 并拉取模型（如 `ollama pull gemma3:270m`），然后在配置文件中设置 `summarize_backend: "ollama"`。TTS 引擎本身也支持本地离线方案（piper / kokoro / moss-nano），两者搭配可实现完全离线的语音朗读。其中 moss-nano 支持多语言和音色克隆，48kHz 高音质输出。

**Q: 可以同时运行多个 ClawBench 实例吗？**

A: 可以。发布版和开发版使用独立端口和数据库，可以同时运行。也可以通过 `--port` 参数指定不同端口运行多个实例。

**Q: 是否需要配置文件才能启动？**

A: 不需要。所有配置项均有默认值，无需 `config.yaml` 即可启动。未配置 `password` 时自动生成随机密码并保存到 `.clawbench/auto-password`，启动脚本会自动显示。如需自定义，复制 `config/config.example.yaml` 为 `config.yaml` 并修改。

**Q: 忘记自动生成的密码怎么办？**

A: 查看 `.clawbench/auto-password` 文件即可获取密码。也可以在 `config.yaml` 中设置 `password` 来使用固定密码。

**Q: 数据存储在哪里？**

A: 数据存储在二进制文件同级目录下的 `.clawbench/` 中，包括数据库文件（`ClawBench.db`）、日志文件（`logs/`）和自动密码（`auto-password`）。上传的文件存放在项目目录的 `.clawbench/uploads/` 中。绿色便携，删除程序目录即可彻底卸载。

**Q: 如何备份数据？**

A: 备份二进制同级目录下 `.clawbench/ClawBench.db` 数据库文件即可。
