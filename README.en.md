[中文](README.md) | [English](README.en.md)

# ClawBench — AI Workstation Built for Mobile

<p>
  <img src="assets/logo.svg" alt="ClawBench" width="96" height="96" align="left" style="margin-right:16px;">
</p>

**From Terminal to Palm** — An AI workstation built for mobile.

Brings the full power of AI coding agents to browsers and mobile apps, creating a true mobile development environment. File browsing, code editing, AI conversation, Git operations, scheduled tasks — one app does it all.

Core Advantage: Native passthrough of AI capabilities (tool calls, extended thinking, Skills, MCP) with zero adaptation cost, fully preserving the power of coding agents. Unlike other mobile AI tools that are merely "remote controllers," ClawBench is a full-featured mobile workstation — files, code, Git, AI, scheduled tasks, TTS, get real work done on your phone without needing a PC online. ([Similar Projects Comparison](docs/COMPARISON.en.md))

- **Supported Platforms**: Browser (PC / Tablet / Phone), Android App, PWA
- **AI Backends**: CodeBuddy, Claude Code, OpenCode, Gemini CLI, Codex, Qoder CLI, VeCLI

---

## Screenshots

### Login & Navigation

| Login | Home | Select Project |
|-------|------|----------------|
| ![Login](docs/screenshots/login.png) | ![Home](docs/screenshots/home.png) | ![Select Project](docs/screenshots/project-select.png) |

### File Browsing & Code Editing

| File Browser | Search & Filter | Code Editor | Quote & Ask |
|-------------|----------------|-------------|-------------|
| ![File Browser](docs/screenshots/file-browser.png) | ![Search & Filter](docs/screenshots/file-search.png) | ![Code Editor](docs/screenshots/code-editor.png) | ![Quote & Ask](docs/screenshots/quote-question.png) |

### Markdown & Document Preview

| Markdown Render | LaTeX Formulas | Mermaid Diagrams | Table of Contents |
|-----------------|----------------|------------------|-------------------|
| ![Markdown Render](docs/screenshots/markdown-preview.png) | ![LaTeX Formulas](docs/screenshots/latex-formula.png) | ![Mermaid Diagrams](docs/screenshots/mermaid-diagram.png) | ![Table of Contents](docs/screenshots/toc-drawer.png) |

### AI Agents

| Agent Selection | AI Conversation | Structured Question | Session Manager |
|-----------------|-----------------|---------------------|-----------------|
| ![Agent Selection](docs/screenshots/agent-selector.png) | ![AI Conversation](docs/screenshots/chat-panel.png) | ![Structured Question](docs/screenshots/ask-question.png) | ![Session Manager](docs/screenshots/session-manager.png) |

| Scheduled Tasks | Create Task | Task Card |
|-----------------|-------------|-----------|
| ![Scheduled Tasks](docs/screenshots/scheduled-tasks.png) | ![Create Task](docs/screenshots/task-create.png) | ![Task Card](docs/screenshots/schedule-proposal.png) |

### Git Integration

| Commit History & Branch Graph | Commit Detail | Comparison Report |
|-------------------------------|---------------|-------------------|
| ![Commit History & Branch Graph](docs/screenshots/git-history.png) | ![Commit Detail](docs/screenshots/git-commit-detail.png) | ![Comparison Report](docs/screenshots/git-comparison-report.png) |

### Media Preview

| Image Viewer | Video Player | Audio Player | PDF Preview |
|-------------|-------------|-------------|------------|
| ![Image Viewer](docs/screenshots/image-viewer.png) | ![Video Player](docs/screenshots/video-player.png) | ![Audio Player](docs/screenshots/audio-player.png) | ![PDF Preview](docs/screenshots/pdf-preview.png) |

### SSH Tunnel & Web Terminal

| Port Forwarding | Interactive Terminal |
|----------------|---------------------|
| ![Port Forwarding](docs/screenshots/port-forwarding.png) | ![Interactive Terminal](docs/screenshots/terminal.png) |

---

## Technical Architecture

ClawBench's core philosophy:

- **Zero-Adaptation Passthrough**: Instead of reimplementing AI capabilities, ClawBench uses AI coding agent CLIs as backend engines, wrapping them as HTTP API + SSE streaming interfaces via a web server. This fully preserves tool calls, extended thinking, Skills, MCP, and all other capabilities with zero adaptation cost. The frontend only handles rendering and interaction — all intelligent logic is natively provided by the CLI.
- **AI Handles Changes, I Handle Review**: The project does not provide direct file editing capabilities — all modifications are done through AI. The focus is on building an excellent Markdown and code preview experience, along with interaction with AI during preview — select code or text to ask AI questions or request modifications for rapid iteration.

```mermaid
graph LR
    Client["📱 Phone / PWA / Pad"] -->|HTTP / SSE| Server["🏗️ ClawBench\nGo Web Server"]
    Server -->|CLI Invocation · Stream Output| CB["🤖 CodeBuddy CLI"]
    Server -->|CLI Invocation · Stream Output| CC["🤖 Claude Code CLI"]
    Server -->|CLI Invocation · Stream Output| OC["🤖 OpenCode CLI"]
    Server -->|CLI Invocation · Stream Output| GC["🤖 Gemini CLI"]
    Server -->|CLI Invocation · Stream Output| CX["🤖 Codex CLI"]
    Server -->|CLI Invocation · Stream Output| QR["🤖 Qoder CLI"]
    Server -->|CLI Invocation · Stream Output| VC["🤖 VeCLI"]
    Server -->|Read/Write| DB[("💾 SQLite\nSessions · History · Scheduled Tasks")]
    CB -->|Native Support| Tools["🔧 Tool Calls"]
    CB -->|Native Support| Think["🧠 Extended Thinking"]
    CB -->|Native Support| Skills["🎯 Skill"]
    CB -->|Native Support| MCP["🔌 MCP"]
    CC -->|Native Support| Tools
    CC -->|Native Support| Think
    CC -->|Native Support| Skills
    CC -->|Native Support| MCP
    OC -->|Native Support| Tools
    GC -->|Native Support| Tools
    GC -->|Native Support| Think
    CX -->|Native Support| Tools
    QR -->|Native Support| Tools
    VC -->|Native Support| Tools
```

---

## Quick Start

### Prerequisites

- **A PC (Linux / macOS / Windows)**: To run the ClawBench server, with at least one AI coding agent CLI installed (CodeBuddy, Claude Code, OpenCode, Gemini CLI, Codex, Qoder CLI, or VeCLI)
- **A phone**: Install the [ClawBench Android App](https://github.com/xulongzhe/clawbench/releases), or use a mobile browser (Chrome recommended) to access the server address

### Download & Extract

Download the latest ZIP package from [GitHub Releases](https://github.com/xulongzhe/clawbench/releases), extract and deploy. All configuration items have default values — no config file needed to start.

```bash
wget https://github.com/xulongzhe/clawbench/releases/latest/download/clawbench-linux-amd64.zip
unzip clawbench-linux-amd64.zip
cd clawbench
```

### Configure Agents

YAML files in the `config/agents/` directory define available AI agents. Each backend provides a separate example file — copy and modify to create your own agent:

| Example File | Backend | Description |
|--------------|---------|-------------|
| `claude.yaml.example` | claude | Claude Code CLI |
| `codebuddy.yaml.example` | codebuddy | CodeBuddy CLI, multi-model support |
| `opencode.yaml.example` | opencode | OpenCode CLI |
| `gemini.yaml.example` | gemini | Gemini CLI |
| `codex.yaml.example` | codex | Codex CLI, profile support |
| `qoder.yaml.example` | qoder | Qoder CLI, auto model routing |
| `vecli.yaml.example` | vecli | VeCLI (Volcengine) |

```bash
# Example: create a Claude agent
cp config/agents/claude.yaml.example config/agents/my-claude.yaml
# Edit id, name, model, etc. and restart the server
```

Each example file contains complete configuration fields and descriptions for that backend. Files with the `.yaml.example` extension are not loaded — they serve as reference templates only.

### Start the Server

```bash
./server.sh
```

> A random password is auto-generated on first startup and printed to the console. Save it securely. To customize configuration, copy `config/config.example.yaml` to `config/config.yaml` and modify.

Once deployed, access `http://server-ip:20000` from your phone app or mobile browser:

- **Phone App**: Native integration, auto-connect, full feature support
- **Mobile Browser**: **Chrome** recommended — supports installing as a PWA app (Add to Home Screen) for a near-native experience

> For build instructions, advanced configuration, deployment, and architecture details, see **[Build & Development Guide](docs/DEVELOPMENT.en.md)**.

---

## Features

### 📁 File Browser
- Recursive directory browsing with 120+ file extension support
- Search filtering, sorting (name/time/extension)
- Context menu: rename, delete, copy, cut, paste, new file/folder, download, open as project
- File upload (image support, configurable size and count)
- Toggle hidden file visibility

### 🎨 Code Preview
- Syntax highlighting, sticky line numbers, word wrap toggle
- Double-click to copy code line content (flash animation feedback)
- **Quote & Ask**: Select a code snippet, one-click ask AI, auto-attaches file path and line number
- Swipe gestures: swipe left/right to switch files

### 📝 Markdown
- Toggle between rendered view / source view
- **Quote & Ask**: Select text, one-click ask AI
- Smart table of contents drawer (TOC), LaTeX math, Mermaid diagrams
- **Image Lightbox**: Images support zoom, swipe browsing
- **File Path Navigation**: Clickable file paths in Markdown

### 🤖 AI Agents
- **Streaming Response**: Real-time SSE push, thinking process and tool calls fully visible
- **Multi-Agent Support**: General assistant, coding expert, handyman, etc. — YAML config, plug-and-play
- **AI Backend Switching**: CodeBuddy, Claude Code, OpenCode, Gemini CLI, Codex, Qoder CLI, VeCLI — session-level isolation
- **Scheduled Tasks**: AI creates Cron schedules via CLI subcommands, executes automatically; task cards embedded in chat messages; frequency presets (hourly/daily/weekly/monthly) + custom cron expressions
- **Multi-Session Management**: Create, switch, delete independent sessions, swipe to switch
- **Image Upload**: Upload images for AI conversation (multimodal)
- **Disconnect Protection**: Messages persist immediately, no data loss on disconnect, 60s timeout auto-reconnect (3 attempts then fallback to polling)
- **Auto Resume**: Automatically sends "continue" after Claude/CodeBuddy exits Plan Mode
- **Message Queue**: Messages queue when AI is busy, sent sequentially

### 🤖 AI Conversation
- **Tool Call Visualization**: Name, parameters, execution results displayed in real time with success/error status
- **Extended Thinking**: Complex tasks auto-trigger extended thinking, reasoning visible in real time
- **File Path Navigation**: Clickable file paths in AI responses
- **Quick Send**: Preset common commands (continue, build, commit, etc.) with drag reorder, one-click send
- **Quote & Ask**: Select code or text, ask AI directly, auto-attaches context
- **Unread Badge**: Chat panel icon shows unread message count

### 🖼️ Media Preview
- In-app preview of images, audio, video
- Lightbox zoom, fullscreen view, support for pinch-zoom and drag

### 🔊 TTS Speech Synthesis
- Auto-summarize and read AI replies aloud, listen while reading
- **5 TTS Engines**: Edge TTS (free), MiniMax (best quality), Piper / Kokoro / MOSS-Nano (local offline)
- **10 Summarization Backends**: simple (text-only cleanup), mmx-cli, Claude, CodeBuddy, Gemini, OpenCode, Codex, Qoder, VeCLI, Ollama (local inference)
- See [TTS Deployment Guide](docs/TTS.en.md)

### 📂 Git Integration
- Project-level / file-level commit history browsing
- **Git Branch Graph**: Vertical branch topology, intuitive branch relationships
- **Git Diff View**: View changes relative to HEAD, character-level highlighting
- Commit detail view (author, time, commit message)
- Working tree changes view (staged / unstaged files)
- Git init (one-click `git init` from UI)

### 🔀 SSH Tunnel Port Forwarding
- **Remote Development**: Access server local ports directly from Android App
- **Protocol Transparent**: HTTP, HTTPS, WebSocket, SSE, gRPC — no URL rewriting needed

### 💻 Web Terminal
- **Interactive Terminal**: PTY + WebSocket + xterm.js, operate server terminal directly in browser
- **Concurrent Sessions**: Each client gets an independent PTY session, no interference
- **Virtual Key Toolbar**: Color-coded key groups (modifiers, shortcuts, navigation, arrows, actions), three-state modifier toggle
- **Symbol Bar**: Expandable symbol input row with 19 high-frequency terminal symbols, smart sorting using exponential decay (balances frequency and recency)
- **Touch Gestures**: Termius-style gestures (swipe→arrow keys, hold-to-repeat, double-tap→Tab, pinch-to-zoom), touch scroll when gestures disabled
- **Quick Commands**: CRUD management of common commands with drag reorder, hidden flag, auto-execute (auto-run on every connect/reconnect)
- **Android Volume Keys**: Volume up/down remapped to arrow keys when terminal is open in the app
- See [Web Terminal User Guide](docs/TERMINAL.en.md)

### 🌐 Internationalization
- Chinese / English bilingual UI, auto-detect system language

### 📱 Android App
- Native bridge integration: auto-login, file download, port forwarding management
- SSH password management, server dialog
- Terminal volume key mapping: volume keys act as arrow keys when terminal is open

### 🔔 Notifications
- Notification sound + haptic feedback (alerts when AI completes)
- Browser push notifications

### 🎨 Themes
- Light / Dark mode, follows system preference

### 📱 PWA Support
- Installable to home screen, runs in standalone window

### 🔒 Security
- Optional password protection (SHA-256 salted)
- Path traversal protection, all operations restricted to project directory
- Configurable file upload size and count (default 10MB / 20 files)
- XSS protection (DOMPurify sanitization)
- TLS support (manual certificate configuration required)

---

## FAQ

See **[FAQ](docs/FAQ.en.md)**.

---

## License

Copyright (c) 2026 xulongzhe

Licensed under the MIT License
