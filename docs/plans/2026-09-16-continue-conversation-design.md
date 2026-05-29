# 继续对话 — 定时任务执行历史转聊天会话

## 概述

在定时任务执行历史详情界面增加"继续对话"按钮。点击后创建一个新的 chat 类型会话，将原 scheduled 会话的 chat_history 和 summaries 复制到新会话，用户可以在聊天界面继续与 AI 对话。

## 数据模型变更

### chat_sessions 表新增列

- `source_session_id TEXT DEFAULT NULL` — 记录"继续对话"来源的 scheduled session UUID 主键。使用 NULL 而非空字符串作为"无来源"的哨兵值，NULL 在 `WHERE source_session_id = ?` 查询中自然排除

### 新增索引

```sql
CREATE INDEX IF NOT EXISTS idx_sessions_source_session
  ON chat_sessions(source_session_id)
  WHERE source_session_id IS NOT NULL;
```

### 新会话字段映射

| 字段 | 值 |
|------|-----|
| `id` | 新 UUID |
| `source_session_id` | 原 scheduled session 的 `id` |
| `session_type` | `'chat'` |
| `project_path` | 继承 |
| `backend` | 继承 |
| `title` | 原定时任务名称（从 `scheduled_tasks.name`） |
| `agent_id` | 继承 |
| `agent_source` | 继承 |
| `model` | 继承 |
| `thinking_effort` | 继承 |
| `external_session_id` | 空（AI 后端在首条消息后分配新的） |

### 复制范围

1. **chat_history**：原 session 下 `deleted=0 AND streaming=0` 的所有消息，复制到新 session_id，其余字段保持一致，`id` 自增。批量复制（每 100 条一批）避免长时间锁表
2. **summaries**：原 session 下 `target_type='chat_message'` 的摘要复制，`target_id` 映射到新消息的 `id`。`target_type='task_execution'` 的摘要不复制
3. **ai_raw_responses**：不复制。原 raw responses 是调试/取证数据，非对话上下文。继续对话后新消息会产生新的 raw responses
4. **tts_summaries**：不复制。TTS 音频可重新生成，不迁移

### 去重逻辑

查询：`SELECT id FROM chat_sessions WHERE source_session_id = ? AND session_type = 'chat' AND deleted = 0`

- 存在 → 返回已有 sessionId，不重复创建
- 已继续的会话被删除（`deleted=1`）→ 不匹配，可重新创建

**并发安全**：整个去重检查 + 创建流程在数据库事务（`BEGIN IMMEDIATE`）中执行，防止 TOCTOU 竞态条件。事务内完成：去重查询 → 创建 session → 复制 history → 复制 summaries → commit

## 后端 API

### POST /api/tasks/{id}/executions/{execId}/continue

创建继续对话会话。

**请求**：无 body

**处理流程**（在 `BEGIN IMMEDIATE` 事务内执行步骤 5-9）：
1. 验证 `taskID` 属于当前 `project_path`（与现有 `ServeTaskByID` 模式一致）
2. 查询 `task_executions` 获取 `session_id`，验证 `execId` 属于 `taskID`
3. 检查执行状态，`status='running'` 时返回 400 错误
4. 查询 `scheduled_tasks` 获取 `name`（作为新会话标题）
5. 查询原 `chat_sessions` 行获取 `backend`, `agent_id`, `agent_source`, `model`, `thinking_effort`, `project_path`（**不加 `deleted=0` 过滤**，软删除的 session 元数据仍有效）
6. 去重检查：`WHERE source_session_id = ? AND session_type = 'chat' AND deleted = 0` → 如果存在，返回 `{ok: true, sessionId: "已存在的ID", alreadyExists: true}`
7. 最大会话数检查：复用 `GetSessionCount` 逻辑，超限返回 409
8. 创建新 `chat_sessions` 行
9. 复制 `chat_history`（`WHERE session_id = ? AND deleted = 0 AND streaming = 0`），批量 100 条
10. 复制 `summaries`（`target_type='chat_message'`，`target_id` 映射到新消息 ID，使用 `INSERT OR REPLACE` 防止 UNIQUE 约束冲突）
11. 设置 session cookie：`setSessionID(w, newSessionID)`
12. 返回 `{ok: true, sessionId: "新UUID", alreadyExists: false}`

**响应**：
- 200：`{ok: true, sessionId: "uuid", alreadyExists: boolean}`
- 400：执行仍在运行
- 403：任务不属于当前项目
- 404：执行不存在或会话不存在
- 409：达到最大会话数

**路由变更**：`ServeTaskByID` 需扩展子路径解析，支持 `executions/{execId}/continue` 深度：
```go
if strings.HasPrefix(subPath, "executions/") {
    execParts := strings.SplitN(strings.TrimPrefix(subPath, "executions/"), "/", 2)
    execID := execParts[0]
    if len(execParts) > 1 && execParts[1] == "continue" {
        serveContinueConversation(w, r, taskID, execID, projectPath)
        return
    }
}
```

### GET /api/tasks/{id}/executions/{execId}/continue

预检查是否已有继续会话。

**响应**：
- 200：`{exists: boolean, sessionId: "uuid" | ""}`
- 403：任务不属于当前项目
- 404：执行不存在

## 前端交互

### TaskExecDetail.vue

1. 在执行详情底部增加"继续对话"按钮
2. 点击流程：
   - 按钮进入 loading 状态，防止重复点击
   - 调用 `GET /api/tasks/{id}/executions/{execId}/continue` 预检查
   - `exists: true` → 切换到聊天面板 → `switchSession(sessionId)` 切换到已有会话
   - `exists: false` → 调用 `POST` 创建新会话 → 切换到聊天面板 → `switchSession(sessionId)` 加载新会话
   - 导航顺序：先 `switchTab('chat')` 激活聊天面板，再 `switchSession(sessionId)` 加载数据，避免短暂显示旧会话
3. 执行中（`status='running'`）时按钮禁用
4. 错误处理：
   - 409 → toast "已达到最大会话数，请先删除旧会话"
   - 其他 → toast 通用错误信息

### useChatSession 变更

新增 `continueFromExecution(taskId, execId)` 方法，封装预检查 + 创建 + 切换的完整流程。

### 导航行为

切换到新会话后，从任务详情页回到聊天界面。流程：`switchTab('chat')` → `switchSession(sessionId)`，确保聊天面板先激活再加载会话数据。

## 边界情况

| 场景 | 处理 |
|------|------|
| 执行仍在运行 | 按钮禁用，提示"执行中，无法继续对话" |
| 原 scheduled session 软删除 | 不影响，步骤5查询不加 `deleted=0`，元数据仍可读取；chat_history 复制时用 `deleted=0` 过滤 |
| 继续后的会话被删除 | 去重查询不匹配（`deleted=1`），可重新创建（有意设计：用户删除了旧继续会话，可重新开始） |
| chat_history 有 streaming=1 消息 | 复制时跳过（`streaming=0` 过滤） |
| summaries target_type='task_execution' | 不复制，只复制 'chat_message' 类型 |
| 并发点击 | 前端 loading 防重复 + 后端 `BEGIN IMMEDIATE` 事务保证去重原子性 |
| 执行/任务不属于当前项目 | 返回 403 |
| 执行不存在 | 返回 404 |
| 大量历史消息 | 批量复制（每 100 条一批），避免长时间锁表 |
| ai_raw_responses | 不复制，属于调试数据，新消息会产生新的 raw responses |
| tts_summaries | 不复制，TTS 可重新生成 |

## 测试要点

### 后端
1. 继续对话正常流程 — 创建新 chat session，复制 history + summaries
2. 去重 — 已有继续会话时返回已有 sessionId
3. 删除后重新继续 — 已继续的会话删除后可再次创建
4. 最大会话数限制 — 超限时返回 409
5. 执行中禁用 — status=running 时返回 400
6. 复制范围 — 只复制 `deleted=0 AND streaming=0` 的消息
7. 字段继承 — agent_id, model, thinking_effort, backend 正确继承
8. summaries 复制 — target_id 映射到新消息 ID
9. 事务原子性 — 并发请求不会创建重复会话
10. 所有权验证 — taskID 属于 project_path，execId 属于 taskID
11. 404/403 错误 — 执行不存在、项目不匹配
12. 软删除源会话 — 仍可继续对话（元数据可读）

### 前端
1. 按钮点击 → 预检查 → 创建 → 切换到聊天面板
2. 已有继续会话 → 直接切换，不重复创建
3. 会话数上限 → toast 提示
4. 执行中 → 按钮禁用
5. loading 状态 → 防止重复点击
6. 导航顺序 — 先 switchTab 再 switchSession
