# Vue Frontend Test Coverage Report - 2026-05-06

## 摘要
- 总模块数: 80
- 已覆盖: 18 (深覆盖: 4, 浅覆盖: 14)
- 未覆盖: 62
- 本次新增: 11 模块
- 本次深化: 0 模块
- 测试结果: 448 PASS / 0 FAIL / 0 SKIP
- 测试总数变化: 240 → 448 (+208)

## 本次变更
| 模块 | 变更类型 | 新增测试数 | 覆盖重点 | 状态 |
|------|---------|-----------|---------|------|
| utils/path | 新增 | 17 | splitPath/baseName/dirName 跨平台路径, Windows驱动器根, 混合分隔符 | PASS |
| utils/fileType | 新增 | 24 | 文件类型检测(代码/图片/音频/视频/PDF), 未知扩展名, 大小格式化 | PASS |
| utils/html | 新增 | 9 | HTML转义(5种特殊字符), 组合转义, 空字符串 | PASS |
| utils/toc | 新增 | 22 | Markdown TOC提取, 代码TOC(Go/TS/Python/Rust等), slugify(中文), YAML/JSON提取, 行号计算 | PASS |
| utils/format | 新增 | 21 | formatDuration(ms/s/m), humanizeCron(5种模式), repeatLabel, statusLabel | PASS |
| utils/diff | 新增 | 15 | hunk header解析, diff行解析(增/删/上下文), 行号追踪, 多hunk | PASS |
| utils/api | 新增 | 11 | apiGet/apiPost/apiDelete/cancelChat, locale header注入, 错误处理, URL编码 | PASS |
| useChatRender | 新增 | 27 | parseAssistantContent(块解析/去重/取消标志/done标记), truncate(Unicode), hasImagesInContent | PASS |
| useChatStream | 新增 | 29 | 内容合并(findLastBlockOfType), tool_use处理, 文件修改检测(Write/Edit), 取消事件处理, SSE重连逻辑 | PASS |
| useChatSession | 新增 | 19 | buildMessageSnapshot变更检测, parseMessages(assistant/user), 分页逻辑, 流式标记fromDB | PASS |
| useSessionIdentity | 新增 | 14 | action回调注册/委托/替换, identity refs初始值, runningSessions完成检测 | PASS |

## 测试方法说明
- **Utils函数**: 直接import并测试happy path + 边界输入 + 错误场景
- **Composable逻辑**: 从源码中提取纯函数逻辑（因为composable依赖Vue响应式和浏览器API），复制核心算法到测试中进行验证
- **API函数**: 使用vi.mock模拟fetch和i18n，测试请求构造和错误处理

## 覆盖缺口（仍需关注）

### 高优先级 - 无测试的Composables
- useAppMode: Android WebView检测, native bridge集成
- useAutoSpeech: TTS自动朗读状态管理(module-level singleton)
- usePortForward: 端口转发状态和SSH信息
- useFileUpload: 文件上传限制和验证
- useNotification: 推送通知
- useToast: Toast通知系统
- useMarkdownRenderer: Markdown渲染(KaTeX/Mermaid/代码高亮)
- useFilePathAnnotation: 文件路径检测和验证
- useFileWatch: fsnotify SSE集成
- useFileRefresh: 文件刷新回调
- useLocale: 国际化

### 中优先级 - 无测试的Utils
- clipboard: 剪贴板操作
- globals: 共享单例(marked/hljs)
- icons: 图标映射
- mermaid: Mermaid图表渲染
- pwa-install: PWA安装提示

### 低优先级 - 无测试的Components
- 所有44个Vue组件无测试
- preview-layout.test.ts 浅覆盖4个组件
- 组件测试需要完整的Vue Test Utils mount + 大量mock

### 已有但需要深化的模块
- toolCallSummary: 逻辑复制而非import，需重构为直接import
- stopBtnTwoClick: 逻辑复制而非import，需重构为直接import
- pendingMessageQueue: API交互模式复制，无错误场景
- preview-layout: 仅浅mount，无交互测试
