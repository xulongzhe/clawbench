# ClawBench 系统设计规格

ClawBench 是一个移动优先的 AI 工作站，将多种 AI CLI 工具（CodeBuddy、Claude Code、OpenCode、Gemini CLI、Codex、Qoder CLI、VeCLI、DeepSeek TUI、Pi）包装为 Web 可访问的平台。Go 后端通过 shell 调用 CLI 工具并经 SSE 流式输出 JSON；Vue 3 前端实时渲染流式事件。支持 SSH 隧道端口转发和定时任务系统。

## 模块地图

### core/ — 核心业务

| 模块 | 说明 |
|------|------|
| [聊天流程](core/chat-flow.md) | 用户发消息到 AI 回复的完整链路：handler → service → AI 后端 → SSE → 前端 |
| [AI 后端抽象](core/ai-backend.md) | 多 AI 后端的统一接口、CLI shell-out 模式、流式解析、AutoResume 机制 |
| [流式传输体系](core/streaming.md) | SSE 推送与重连策略、WebSocket 事件通道、HTTP 轮询降级 |
| [会话生命周期](core/session-lifecycle.md) | 聊天会话的创建、执行、排队、取消、归档全流程 |

### features/ — 功能特性

| 模块 | 说明 |
|------|------|
| [定时任务](features/scheduled-tasks.md) | cron 调度 → AI 执行 → 摘要推送，支持暂停/恢复/手动触发 |
| [语音合成](features/tts.md) | 多引擎 TTS（云/本地），文本清理，缓存策略 |
| [Web 终端](features/terminal.md) | PTY 会话管理，WebSocket 双向通信，手势与虚拟键盘 |
| [Git 管理](features/git-management.md) | 历史浏览、Worktree 隔离、分支/标签 CRUD、滑动手势删除 |
| [文件管理](features/file-management.md) | 浏览、编辑、上传、缩略图生成、归档打包 |
| [RAG 检索](features/rag.md) | 文档分块、向量化（BGE-M3）、DuckDB 向量存储、混合检索 |
| [推送通知](features/push-notifications.md) | JPush 集成、WebSocket 后备、推送感知的后台策略 |

### infra/ — 基础设施

| 模块 | 说明 |
|------|------|
| [认证与中间件](infra/auth-and-middleware.md) | 密码认证、localhost 旁路、请求链、panic 恢复 |
| [SSH 隧道](infra/ssh-tunnel.md) | direct-tcpip 端口转发、密码认证、自动 host key、暴力破解防护 |
| [Proxy 注册表](infra/proxy.md) | 反向代理、Host 头重写、特权端口映射、前端端口展示 |
| [配置与自动发现](infra/config-and-discovery.md) | 零配置启动、Agent/Model 自动发现与缓存 |
| [事件体系](infra/event-system.md) | EventBus、WebSocket 广播、JPush 后备、断线缓冲重放 |

### client/ — 客户端

| 模块 | 说明 |
|------|------|
| [前端架构](client/frontend-architecture.md) | 单页布局、reactive store、composable 模式、SSE/WebSocket 双通道 |
| [Android 集成](client/android-integration.md) | JS Bridge、后台服务、SSH 端口转发、推送感知生命周期 |

## 核心技术栈

| 层 | 技术 |
|----|------|
| 后端 | Go 1.23+、SQLite（WAL）、DuckDB（向量）、robfig/cron |
| 前端 | Vue 3 + TypeScript、Vite、xterm.js、marked + hljs |
| AI 集成 | Shell-out 到 CLI 工具、stream-json 解析 |
| 实时通信 | SSE（聊天流）、WebSocket（系统事件）、SSH（端口转发） |
| 移动端 | Android WebView、JPush 推送、原生后台服务 |
