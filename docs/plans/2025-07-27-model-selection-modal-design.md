---
name: Model Selection Modal Design
date: 2025-07-27
---

# 模型选择模态框设计

## 需求

将模型选择从 ChatInputBar 的 PopupMenu 改为模态框，合并「会话级切换」和「设为默认模型/思考深度」两条路径。

### 功能点

| # | 功能 | 说明 |
|---|------|------|
| 1 | 双 Tab 切换 | 「模型」Tab + 「思考深度」Tab |
| 2 | 模型搜索 | 搜索框实时过滤模型名称 |
| 3 | 模型刷新 | 刷新按钮触发后端重新发现模型，完全覆盖当前列表 |
| 4 | 会话级切换 | 点击模型/深度 → 关闭模态框，切换当前会话 |
| 5 | 设为默认 | 长按模型/深度项 → PopupMenu → 「设为默认」→ PATCH 持久化 |
| 6 | 清理外部入口 | 移除 ChatInputBar 思考深度 PopupMenu + Settings > Agents 模型/深度选择 |

### 移除内容

- `ChatInputBar.vue` 中的模型 PopupMenu → 改为打开模态框
- `ChatInputBar.vue` 中的思考深度 PopupMenu → 移除
- `SettingsCategory.vue` 中 agents 类别下的模型/思考深度 select 行 → 移除

---

## 交互设计

### 入口

点击 ChatInputBar 的模型芯片（当前显示模型名的按钮），打开模态框。

### 模态框结构

```
┌─────────────────────────────────┐
│  [模型]  [思考深度]              │  ← Tab 切换
├─────────────────────────────────┤
│  🔍 搜索模型...        [🔄]    │  ← 仅模型 Tab 显示搜索框+刷新
├─────────────────────────────────┤
│  ● claude-sonnet-4-6   默认    │  ← 高亮=当前会话, 标签=默认
│    claude-opus-4-5              │
│    claude-haiku-3-5             │
│    ...                          │
└─────────────────────────────────┘
```

### 操作

| 操作 | 效果 |
|------|------|
| 点击模型/深度 | 切换当前会话，关闭模态框 |
| 长按模型/深度 | 弹出 PopupMenu，选项：「设为默认」 |
| 点击刷新按钮 | 触发后端重新发现模型，覆盖当前列表，按钮显示 loading |
| 输入搜索文本 | 实时过滤模型名称（大小写不敏感） |

### 视觉标记

- **当前会话选中**：左侧圆形实心标记 + 文字加粗
- **默认模型/深度**：右侧小标签 `默认`（灰色小字或 badge）
- 两者可以不同（会话选了 opus，默认还是 sonnet）

---

## 后端改动

### 新增 API: `POST /api/agents/{id}/refresh-models`

触发指定 Agent 的后端模型重新发现，覆盖内存中的模型列表和缓存文件。

逻辑：
1. 查找 Agent 对应的 BackendSpec
2. 调用 DiscoverModels()（同 SyncDiscoverModels 逻辑）
3. 更新 Agent.Models（无论 ModelsAutoDetected 与否，手动刷新都覆盖）
4. 写入 .clawbench/model-cache/{backend}.json
5. 返回新的模型列表

请求/响应：
```json
// POST /api/agents/claude/refresh-models
// Response: 200
{
  "models": [
    {"id": "claude-sonnet-4-6", "name": "Claude Sonnet 4.6", "default": true},
    ...
  ]
}
```

错误处理：
- Agent 不存在 → 404
- 后端不支持模型发现（无 ListModelsCmd/DiscoverModelsFunc）→ 400 + 提示
- 发现过程中 CLI 执行失败 → 500 + 错误信息

### 修改 API: `PATCH /api/agents`

现有接口已支持 `preferred_model` 和 `preferred_thinking_effort`，无需修改。

---

## 前端改动

### 新增组件: `ModelModal.vue`

位置：`web/src/components/chat/ModelModal.vue`

Props:
```ts
{
  show: boolean           // v-model:show
  agentId: string         // 当前智能体 ID
  activeTab?: 'model' | 'thinking'  // 初始激活 Tab，默认 'model'
}
```

内部数据:
```ts
{
  activeTab: ref<'model' | 'thinking'>('model')
  searchQuery: ref('')
  refreshing: ref(false)
  longPressedItem: ref(null)   // 当前长按的项
  showDefaultMenu: ref(false)  // 长按弹出的 PopupMenu
}
```

核心逻辑:
- 模型列表来自 `useAgents().getAgentModels(agentId)`
- 思考深度列表来自 `useAgents().getAgentThinkingLevels(agentId)`（需新增）
- 当前会话模型/深度来自 `useSessionIdentity()`
- 默认模型/深度来自 agent 对象的 `preferredModel` / `preferredThinkingEffort`
- 搜索过滤：computed，`model.name.toLowerCase().includes(query)`

### 修改组件: `ChatInputBar.vue`

- 移除模型 PopupMenu（`showModelMenu` 相关代码）
- 移除思考深度 PopupMenu（`showThinkingEffortMenu` 相关代码）
- 移除思考深度芯片按钮
- 新增 `showModelModal` ref，模型芯片点击 → `showModelModal = true`
- 引入 `ModelModal` 组件

### 修改组件: `SettingsCategory.vue`

- 移除 agents 类别下 `agent-model-{agentId}` 行
- 移除 agents 类别下 `agent-thinking-{agentId}` 行

### 修改 composable: `useAgents.ts`

- 新增 `getAgentThinkingLevels(agentId)` 辅助函数
- 新增 `refreshAgentModels(agentId)` → 调 `POST /api/agents/{id}/refresh-models`，更新内存

### 修改 composable: `useSessionIdentity.ts`

- `saveModelPref` 不再是 no-op，切换会话模型时可直接更新（保持 session-scoped）
- 或者保持 no-op，由模态框直接操作 `currentModelId`/`currentThinkingEffort`

---

## 实现步骤

1. **后端**: 新增 `POST /api/agents/{id}/refresh-models` 接口
2. **前端**: `useAgents.ts` 新增 `getAgentThinkingLevels` + `refreshAgentModels`
3. **前端**: 新建 `ModelModal.vue` 组件
4. **前端**: 修改 `ChatInputBar.vue` — 接入模态框，移除旧 PopupMenu
5. **前端**: 修改 `SettingsCategory.vue` — 移除 agents 模型/深度行
6. **测试**: 验证会话级切换、设为默认、刷新、搜索、思考深度
