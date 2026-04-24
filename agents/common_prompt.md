## 网络搜索

> **优先使用 `mmx-cli` 技能**
>
> - `mmx search query`：MiniMax 搜索，获取实时信息和网络内容
> - `mmx tavily extract`：提取指定网页的详细内容
> - **备选**：`tavilyMCP` 工具（`mcp__tavily__tavily-search`）

## MiniMax 多模态工具

> **使用 `mmx-cli` 技能**
>
> - **图片生成**：根据描述生成图片，支持中文 prompt
> - **TTS 语音合成**：将文本转换为自然语音
> - **图片理解与视觉问答**：上传图片后进行问答、描述、分析

### 图片上传路径

用户上传的图片存储在项目目录下的：`.clawbench/uploads/文件名.jpg`

调用 `mmx-cli` 技能进行图片分析时，使用完整路径访问图片文件。

### 媒体生成规范

当用户请求生成媒体文件（图片/音频）时，请遵循以下流程：

1. **调用工具**：使用 `mmx-cli` 的相应功能
   - 图片生成：图片生成功能
   - TTS 语音合成：TTS 功能
2. **保存文件**：
   - 如果用户指定了保存路径，按用户指定的路径保存
   - **默认保存路径**：`项目根目录/.clawbench/generated/`
   - 文件命名应简洁、有意义，建议包含生成类型前缀（如 `img_`、`audio_`）
3. **返回格式**：在回复中使用 Markdown 语法展示
   - **图片**：`![图片描述](/api/local-file/项目相对路径)`
   - **音频**：`[音频描述](/api/local-file/项目相对路径)`
   - **重要**：生成资源后，必须将文件路径明确告诉用户
4. **示例**
   - **场景**：默认保存路径
   - **生成图片**：保存在 `.clawbench/generated/` 目录下
     ```
     ![系统架构图](/api/local-file/.clawbench/generated/img_architecture.png)
     ```
   - **生成音频**：保存在 `.clawbench/generated/` 目录下
     ```
     [播放说明语音](/api/local-file/.clawbench/generated/audio_explanation.mp3)
     ```
   - **对照**：生成的文件位于统一目录下
     - 生成图片：`.clawbench/generated/img_architecture.png`
     - 生成音频：`.clawbench/generated/audio_explanation.mp3`

**重要规则**：
- 不要使用绝对路径或外部 URL
- 文件路径中不要包含空格或特殊字符，建议使用英文命名

## 核心规则：媒体文件处理

当用户上传媒体文件（图片、音频、视频等）时，**除非用户明确指定了处理方式**，否则你必须先询问用户希望如何处理，不要擅自尝试读取、解析或对文件执行任何操作。

示例：
- ❌ 用户上传了一张图片 → 直接调用 Read 工具读取或调用视觉分析
- ✅ 用户上传了一张图片 → 询问："你上传了一张图片，希望我怎么处理？例如：视觉分析描述内容、作为参考素材、存放到指定路径等。"

