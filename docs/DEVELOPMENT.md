[中文](DEVELOPMENT.md) | [English](DEVELOPMENT.en.md)

# 编译与开发指南

## 快速开始

### 方式一：使用发布包（推荐）

从 [GitHub Releases](https://github.com/xulongzhe/clawbench/releases) 下载最新版 ZIP 包，解压即可部署。所有配置项均有默认值，无需配置文件即可启动。

```bash
# 1. 下载并解压
wget https://github.com/xulongzhe/clawbench/releases/latest/download/clawbench-linux-amd64.zip
unzip clawbench-linux-amd64.zip

# 2. 启动服务（无需配置文件）
cd clawbench
./server.sh
```

> 首次启动会自动生成随机密码并保存到 `.clawbench/auto-password`，启动脚本会自动显示。如需自定义配置，可复制 `config/config.example.yaml` 为 `config.yaml` 并修改。

发布包内容（Linux）：

| 文件 | 说明 |
|------|------|
| `clawbench-linux-amd64` | 后端二进制 |
| `public/` | 前端静态资源（已构建） |
| `config/config.example.yaml` | 配置模板（可选） |
| `config/agents/` | 智能体配置 |
| `dev-server.sh` | 开发调试启动脚本 |
| `server.sh` | 正式版启动脚本 |

发布包内容（Windows）：

| 文件 | 说明 |
|------|------|
| `clawbench-windows-amd64.exe` | 后端二进制 |
| `public/` | 前端静态资源（已构建） |
| `config/config.example.yaml` | 配置模板（可选） |
| `config/agents/` | 智能体配置 |
| `server.ps1` | 启动/停止脚本 |

### 方式二：从源码构建

**Linux/macOS：**

```bash
# 1. 克隆项目
git clone https://github.com/xulongzhe/clawbench.git
cd clawbench

# 2. 一键构建并启动（无需配置文件，所有项均有默认值）
./build.sh && ./server.sh
```

**开发调试模式**：

```bash
# 后台启动（Go dev 后端 + Vite 热更新）
./dev-server.sh

# 前台启动，查看实时日志
./dev-server.sh --fg

# 停止后台进程
./dev-server.sh --stop

# 重启
./dev-server.sh --restart
```

**Windows (PowerShell)：**

```powershell
# 1. 克隆项目
git clone https://github.com/xulongzhe/clawbench.git
cd clawbench

# 2. 一键构建并启动（无需配置文件）
.\build.ps1; .\server.ps1
```

**交叉编译**（在 Linux 上构建 Windows 二进制）：

```bash
./build.sh --windows
```

### 系统要求

| 平台 | 支持 |
|------|------|
| Linux x86_64 / ARM64 | ✅ |
| Windows x86_64 | ✅ |

| 依赖 | 说明 |
|------|------|
| **CodeBuddy CLI** 或 **Claude Code CLI** | AI 后端（需提前安装并完成认证，可选 OpenCode / Gemini / Codex） |

### 配置文件

`config.yaml` 完全可选，所有配置项均有默认值。如需自定义，复制 `config/config.example.yaml` 为 `config.yaml` 并修改。

**默认值**：

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| `port` | 20000 | 服务端口 |
| `watch_dir` | 用户家目录 | Linux: `/home/用户名`, Windows: `C:\Users\用户名` |
| `password` | 自动生成 UUID | 首次生成后保存到 `.clawbench/auto-password`，重启复用 |
| `log_dir` | `<BinDir>/.clawbench/logs` | 二进制同级目录下 |
| `log_max_days` | 7 | 日志保留天数 |
| `upload.max_size_mb` | 100 | 上传大小上限 MB |
| `upload.max_files` | 20 | 单次上传文件数上限 |
| `chat.initial_messages` | 20 | 初始加载消息条数 |
| `chat.page_size` | 20 | 懒加载每页消息条数 |
| `chat.collapsed_height` | 150 | 历史消息折叠高度 px |
| `session.max_count` | 10 | 每项目会话上限 |
| `proxy.enabled` | true | 端口转发默认启用 |
| `proxy.allowed_ports` | `1024-65535` | 允许转发的端口范围 |
| `ssh.enabled` | true | SSH 隧道默认启用 |
| `tts.engine` | edge | Edge TTS 免费无限制 |
| `tts.summarize_backend` | simple | 纯文本清洗，零延迟 |
| `tts.speed` | 1.0 | 正常语速 |
| `tts.inline_code_max_len` | 100 | 行内代码保留最大字符数（rune），超出则删除 |
| `tts.max_summarize_runes` | 10000 | 总结输入最大字符数，超出截取尾部 |

**自动密码机制**：未配置 `password` 时，系统自动生成随机 UUID 作为密码，保存到 `.clawbench/auto-password`（权限 0600）。重启时复用已保存的密码，不会重新生成。配置 `password` 后自动删除该文件。启动脚本会从文件读取并显示密码。

**示例配置**：

```yaml
# 以下均为默认值，仅在需要覆盖时才需配置
# port: 20000
# watch_dir: "/home/user"       # Linux/macOS 默认为用户家目录
# watch_dir: "C:\\Users\\user"  # Windows 默认为用户家目录
# password: "your_password"     # 不配置则自动生成
# default_agent: "assistant"   # 默认智能体，留空则使用第一个智能体
```

### 启动命令

#### 正式版

| 命令 | 说明 |
|------|------|
| `./clawbench-linux-amd64` | 直接运行（前台） |
| `./server.sh` | 后台启动（端口 20000） |
| `./server.sh --fg` | 前台启动（查看实时日志） |
| `./server.sh --stop` | 停止服务 |
| `./server.sh --restart` | 重启服务 |
| `./server.sh --port 8080` | 指定端口 |

#### 开发调试模式

| 命令 | 说明 |
|------|------|
| `./dev-server.sh` | 后台启动（dev 后端 + Vite，端口 20002/20001） |
| `./dev-server.sh --fg` | 前台启动 |
| `./dev-server.sh --stop` | 停止进程 |
| `./dev-server.sh --restart` | 重启 |

> **注意**：开发调试与正式版使用独立端口和数据库，可同时运行，互不干扰。

**Windows**：

| 命令 | 说明 |
|------|------|
| `.\clawbench-windows-amd64.exe` | 直接运行（前台） |
| `.\server.ps1` | 后台启动 |
| `.\server.ps1 -Foreground` | 前台启动 |
| `.\server.ps1 -Stop` | 停止服务 |
| `.\server.ps1 -Restart` | 重启服务 |
| `.\server.ps1 -Port 8080` | 指定端口 |

---

## 高级配置

完整配置参考 `config/config.example.yaml`。所有项均可选，以下为覆盖默认值的示例：

```yaml
# port: 20000                        # 发布版服务端口（默认 20000）
# watch_dir: "/home/user"            # 项目监控目录（默认用户家目录）
# password: "your_password"          # 访问密码（不配置则自动生成 UUID 并保存）

# 默认智能体（可选）
default_agent: "assistant"      # 默认使用的智能体 ID，留空则使用第一个智能体
                                 # 可用智能体：assistant（全能助手）、coder（编码专家）、
                                 # gemini（Gemini CLI）、handyman（勤杂工）、codebuddy2（Gemini）、gpt54（GPT）

# 上传限制（默认 max_size_mb: 100, max_files: 20）
upload:
  max_size_mb: 10
  max_files: 20

# 日志配置（默认 .clawbench/logs, 7 天保留）
log_dir: ".clawbench/logs"
log_max_days: 7

# TLS (HTTPS) 配置（可选）
tls:
  enabled: false                # 启用 HTTPS
  cert_file: "/path/to/fullchain.pem"   # 证书文件
  key_file: "/path/to/privkey.pem"      # 私钥文件

# 端口转发配置（默认启用，端口范围 1024-65535）
proxy:
  enabled: true
  allowed_ports: "1024-65535"

# SSH 隧道配置（默认启用）
ssh:
  enabled: true
  port: 0                       # SSH 端口（0 = 自动 = 主端口+1）
  host_key: ""                  # Host key 文件路径（空 = 自动生成）

# Chat UI 配置（默认 initial_messages: 20, page_size: 20, collapsed_height: 150）
chat:
  initial_messages: 20
  page_size: 20
  collapsed_height: 150
  quick_send:                   # 快捷发送预设（输入框为空时，点击发送按钮弹出菜单选择）
    "▶️ 继续": "继续"             # Key: 菜单显示标签（可带 emoji），Value: 实际发送的文本
    "👌 OK": "OK"
    "🔨 编译": "帮我编译当前项目，优先使用项目中已有的脚本。"
    "🔄 重启调试": "帮我重启当前项目的调试版本服务，优先使用项目中已经有的脚本。"
    "🚀 重启服务": "帮我重启当前项目的发布版本服务，优先使用项目中已经有的脚本。"
    "📦 提交": "提交"
    "👀 浏览变更": "工作区改了什么"
    "🗑️ 丢弃变更": "放弃工作区修改"
```

### AI 后端配置

ClawBench 通过调用本地 CLI 实现与 AI 编程工具的交互，无需额外 API Key 配置。

**CodeBuddy 后端**：安装 CodeBuddy CLI 并完成登录认证，确保 `codebuddy` 命令在 PATH 中可用。

**Claude Code 后端**：安装 Claude Code CLI 并完成认证，确保 `claude` 命令在 PATH 中可用。

**OpenCode 后端**：安装 OpenCode CLI 并完成认证，确保 `opencode` 命令在 PATH 中可用。

**Gemini CLI 后端**：安装 Gemini CLI 并完成认证，确保 `gemini` 命令在 PATH 中可用。

**Codex 后端**：安装 OpenAI Codex CLI 并完成认证，确保 `codex` 命令在 PATH 中可用。

五种后端可在 ClawBench Web UI 中实时切换，会话数据隔离。

### TTS 语音合成配置

ClawBench 支持 TTS 语音合成，自动将 AI 回复总结后朗读。支持 5 种 TTS 引擎和 8 种总结后端。

| TTS 引擎 | 说明 | 网络要求 |
|----------|------|---------|
| `edge` | 微软 Edge TTS，免费无限制（默认） | 需要网络 |
| `minimax` | 云端合成，音质最佳 | 需要 mmx CLI + API 配额 |
| `piper` | 本地离线，速度极快 | 无需网络 |
| `kokoro` | 本地离线，高质量中文 | 无需网络 |
| `moss-nano` | 本地离线，多语言，48kHz 音色克隆 | 首次需下载模型 |

各引擎的安装部署、配置示例、可用音色等详细说明请参阅 **[TTS 语音合成部署指南](TTS.md)**。

---

## 部署说明

### HTTPS 配置（公网部署）

生产环境建议启用 HTTPS：

1. **获取证书**：使用 Let's Encrypt 或其他 CA 签发证书
2. **配置 TLS**：在 `config.yaml` 中启用
   ```yaml
   tls:
     enabled: true
     cert_file: "/etc/letsencrypt/live/your-domain.com/fullchain.pem"
     key_file: "/etc/letsencrypt/live/your-domain.com/privkey.pem"
   ```
3. **重启服务**：`./server.sh --restart`

### 数据存储

| 数据 | 路径 | 说明 |
|------|------|------|
| 数据库 | `二进制同级/.clawbench/ClawBench.db` | SQLite，会话/历史/项目/定时任务 |
| 日志 | `二进制同级/.clawbench/logs/` | 按天轮转，自动清理 |
| 自动密码 | `二进制同级/.clawbench/auto-password` | 未配置 password 时自动生成，重启复用 |
| 上传文件 | `项目目录/.clawbench/uploads/` | 用户上传的文件，属于具体项目 |

> 所有运行时数据存放在二进制同级目录下的 `.clawbench/`，实现绿色便携部署，删除程序目录即可彻底卸载。当项目目录与二进制目录相同时，上传文件也在同一个 `.clawbench/` 下。

### 开发调试模式

使用 `./dev-server.sh` 启动独立开发环境：

- 后端：`http://localhost:20002`
- 前端（Vite HMR）：`http://localhost:20001`
- 数据库：使用 `ClawBench-dev.db`，与正式版数据隔离

```bash
./dev-server.sh              # 后台启动
./dev-server.sh --fg         # 前台启动
./dev-server.sh --stop       # 停止
./dev-server.sh --restart    # 重启
```

---

## 架构设计

### 智能体架构

ClawBench 不只是一个"聊天壳"——它是一个完整的智能体运行平台：

```
config/agents/
├── assistant.yaml     # 全能助手 — 通用问答、代码、文档、运维
├── codebuddy2.yaml    # Gemini（通过 CodeBuddy 调用）
├── coder.yaml         # 编码专家 — 复杂编码、架构设计、代码重构
├── codex.yaml         # Codex — OpenAI Codex CLI 编码助手
├── gemini.yaml        # Gemini CLI — Google Gemini 驱动的通用助手
├── gpt54.yaml         # GPT — 通过 CodeBuddy 调用 GPT 模型
└── handyman.yaml      # 勤杂工 — 定时任务、简单编码、日常操作
```

- **Agent 配置化**：每个智能体通过 YAML 定义专属 system prompt、模型、后端，无需改代码
- **共享提示词**：`config/agent_common_prompt.md` 定义所有智能体的公共行为（网络搜索、多模态、媒体处理），避免重复配置
- **模板占位符**：`{{AVAILABLE_AGENTS}}` 自动替换为可用智能体列表，方便智能体间互相调度
- **多 Agent 调度**：不同任务匹配不同智能体，全能助手负责对话，专业 Agent 执行定时任务
- **工具调用透传**：AI 的工具调用（文件读写、Bash 命令、代码编辑）实时可视化展示
- **Cron 定时执行**：AI 生成 `<schedule-proposal>` 提案，确认后由 Cron 调度自动执行
- **Cron 管控**：Claude 后端通过 `--disallowedTools` 禁用内置定时工具，统一走 ClawBench 调度
- **多后端可切换**：同一平台同时支持 CodeBuddy、Claude Code、OpenCode、Gemini CLI、Codex 后端，会话数据隔离

### 项目结构

```
clawbench/
├── cmd/server/main.go           # 应用入口
├── internal/
│   ├── handler/                 # HTTP 处理器
│   │   ├── handler.go           # 路由注册
│   │   ├── auth.go              # 认证
│   │   ├── chat.go              # AI 聊天（SSE 流式推送）
│   │   ├── agent.go             # Agent 管理
│   │   ├── scheduler.go         # 定时任务
│   │   ├── file.go              # 文件读取
│   │   ├── file_ops.go          # 文件操作
│   │   ├── upload.go            # 文件上传
│   │   ├── git.go               # Git 操作
│   │   ├── project.go           # 项目管理
│   │   ├── ssh_info.go          # SSH 隧道信息接口
│   │   └── static.go            # 静态文件
│   ├── middleware/              # 中间件（认证/日志/恢复/请求ID）
│   ├── platform/                # 平台适配（Windows 路径等）
│   ├── service/                 # 业务逻辑
│   │   ├── database.go          # SQLite 初始化
│   │   ├── chat.go              # 聊天历史管理
│   │   ├── scheduler.go         # 定时任务调度
│   │   ├── uuid.go              # UUID 工具
│   │   └── logger.go            # 文件日志（按天轮转）
│   ├── model/                   # 数据模型
│   │   ├── config.go / defaults.go / chat.go / file.go / agent.go / scheduler.go / path.go / ssh.go
│   │   └── errors.go
│   ├── ssh/                     # SSH 隧道服务器
│   │   ├── server.go            # SSH 服务器（direct-tcpip 端口转发）
│   │   └── server_test.go       # 测试
│   ├── ai/                      # AI 后端抽象
│       ├── interface.go         # AIBackend 接口
│       ├── factory.go           # 后端工厂
│       ├── cli_backend.go       # 共享 CLI 后端抽象
│       ├── stream_parser.go     # 共享流解析工具
│       ├── claude.go / claude_stream.go
│       ├── codebuddy.go / codebuddy_stream.go
│       ├── opencode.go / opencode_stream.go
│       ├── gemini.go / gemini_stream.go
│       └── codex.go / codex_stream.go
│   └── speech/                  # TTS 语音合成 & 总结
│       ├── summarizer.go        # Summarizer 接口 + genericSummarizer 共享管线
│       ├── mmx_summarizer.go    # MMXSummarizer（mmx-cli text chat）
│       ├── ollama_summarizer.go # OllamaSummarizer（HTTP /api/chat）
│       ├── ai_backend_summarizer.go # AIBackendSummarizer（CLI 后端总结）
│       ├── minimax.go / edge.go / piper.go / kokoro.go / moss_tts_nano.go  # TTS 引擎实现
├── config/                      # 配置目录
│   ├── agent_common_prompt.md   # 智能体共享提示词
│   ├── agents/                  # Agent 配置
│   │   ├── assistant.yaml       # 全能助手
│   │   ├── codebuddy2.yaml      # Gemini（CodeBuddy 调用）
│   │   ├── coder.yaml           # 编码专家
│   │   ├── codex.yaml           # Codex CLI
│   │   ├── gemini.yaml          # Gemini CLI
│   │   ├── gpt54.yaml           # GPT（CodeBuddy 调用）
│   │   └── handyman.yaml        # 勤杂工
│   └── config.example.yaml      # 配置模板
├── web/                         # Vue 3 前端源码
│   └── src/
│       ├── components/          # 41 个 Vue 组件
│       ├── composables/         # 13 个组合式函数
│       ├── stores/              # 状态管理
│       └── utils/               # 工具函数
├── build.sh                     # 编译脚本 (Linux/macOS)
├── build.ps1                    # 编译脚本 (Windows)
├── dev-server.sh                # 开发调试启动脚本 (Linux/macOS)
├── server.sh                    # 正式版启动脚本 (Linux/macOS)
├── server.ps1                   # 正式版启动脚本 (Windows)
└── vite.config.ts               # Vite 配置
```

---

## 技术栈

| 层级 | 技术 |
|------|------|
| 后端 | Go 1.21+ (net/http + SQLite) |
| 前端 | Vue 3 + Vite + TypeScript |
| 语法高亮 | highlight.js |
| Markdown | marked.js |
| 图表渲染 | Mermaid.js |
| 数学公式 | KaTeX |
| HTML 净化 | DOMPurify |
| AI 后端 | CodeBuddy CLI / Claude Code CLI / OpenCode CLI / Gemini CLI / Codex CLI（流式 JSON 输出 → SSE 推送） |
| TTS 总结 | Ollama HTTP API（本地推理，gemma3:270m 等小型模型，零外部 Go 依赖） |
| SSH 隧道 | golang.org/x/crypto/ssh（内嵌 SSH 服务器，direct-tcpip 端口转发） |
| 定时调度 | robfig/cron |
