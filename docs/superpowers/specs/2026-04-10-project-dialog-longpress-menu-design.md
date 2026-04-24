# 设计：ProjectDialog 长按菜单

## 概述

为「选择项目」界面的「浏览」Tab 添加长按/右键菜单，替代被移除的顶部「新建文件夹」按钮，支持在目录项上直接进行新建、重命名、删除操作。

## 交互行为

| 触发方式 | 行为 |
|---------|------|
| PC 鼠标右键 | 在目录项上右键 → 弹出菜单 |
| 手机长按 | 按住 450ms → 弹出菜单 |
| 点击菜单外 | 关闭菜单 |
| 长按空白区域 | 无响应 |

## 菜单操作

长按目录项时显示以下操作：

| 选项 | 说明 |
|------|------|
| 新建文件夹 | 在该目录**同级**目录下创建新文件夹 |
| 重命名 | 弹出 prompt 修改目录名，调用 `/api/file/rename` |
| 删除 | 调用 `/api/file/delete`，成功后刷新列表 |

## 菜单 UI

- **样式复用**：`FileManager.vue` 的 `.context-menu` CSS（ProjectDialog 和 FileManager 在同一页面渲染，CSS 类全局可用）
- **定位**：使用 `clientX/clientY`，超出视口时 clamp
- **危险操作**：「删除」使用 `.context-menu-item.danger` 红色样式
- **点击后**：关闭菜单（`ctxMenu.visible = false`）

## 改动点

### 1. 删除顶部按钮

移除 `ProjectDialog.vue` 第 28-30 行的「新建文件夹」toolbar 按钮：

```html
<!-- 删除 -->
<button class="toolbar-btn" @click="createDir" title="新建目录">
  <svg ...><line x1="12" .../><line x1="5" .../></svg>
</button>
```

### 2. 添加状态和菜单模板

```js
// 状态
const ctxMenu = reactive({ visible: false, x: 0, y: 0, entry: null })

// 长按检测（仅处理 item 长按，不处理空白区域）
let pressTimer = null
let pressMoved = false
let pressPos = { x: 0, y: 0 }

function onItemTouchStart(e) {
    pressMoved = false
    const touch = e.touches[0]
    pressPos = { x: touch.clientX, y: touch.clientY }
    pressTimer = setTimeout(() => {
        if (!pressMoved) {
            ctxMenu.x = touch.clientX
            ctxMenu.y = touch.clientY + 10
            ctxMenu.entry = e.currentTarget.__vueParentData
            ctxMenu.visible = true
            nextTick(() => clampCtxMenu())
        }
        pressTimer = null
    }, 450)
}
function onItemTouchMove() { pressMoved = true }
function onItemTouchEnd() { if (pressTimer) { clearTimeout(pressTimer); pressTimer = null } }
```

### 3. 模板改动

在 `.dialog-item` 上绑定事件：

```html
<div
  class="dialog-item"
  @contextmenu.prevent="showCtx($event, item)"
  @touchstart.passive="onItemTouchStart"
  @touchmove="onItemTouchMove"
  @touchend="onItemTouchEnd"
  @touchcancel="onItemTouchEnd"
>
```

添加菜单模板（放在 `.dialog-content` 之后）：

```html
<div v-if="ctxMenu.visible" class="context-menu visible"
     :style="{ left: ctxMenu.x + 'px', top: ctxMenu.y + 'px' }" @click.stop>
  <div class="context-menu-item" @click.stop="doNewFolder">新建文件夹</div>
  <div class="context-menu-item" @click.stop="doRename">重命名</div>
  <div class="context-menu-item danger" @click.stop="doDelete">删除</div>
</div>
<div v-if="ctxMenu.visible" class="ctx-overlay"
     @click="ctxMenu.visible = false" @touchstart="ctxMenu.visible = false" />
```

### 4. 实现操作方法

```js
async function doNewFolder() {
    ctxMenu.visible = false
    const name = prompt('输入文件夹名：')
    if (!name || !name.trim()) return
    const dir = ctxMenu.entry ? ctxMenu.entry.path : browsePath.value
    try {
        const resp = await fetch('/api/projects', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path: dir, name: name.trim() })
        })
        if (resp.ok) await loadBrowse()
        else alert('创建失败')
    } catch (_) { alert('创建失败') }
}

function doRename() {
    if (!ctxMenu.entry) return
    ctxMenu.visible = false
    const newName = prompt('输入新名称：', ctxMenu.entry.name)
    if (!newName || newName === ctxMenu.entry.name) return
    // 调用重命名接口后刷新
    rename({ path: ctxMenu.entry.path, name: newName })
}

async function doDelete() {
    if (!ctxMenu.entry) return
    ctxMenu.visible = false
    if (!confirm('确认删除目录 "' + ctxMenu.entry.name + '" 及其所有内容？')) return
    try {
        const resp = await fetch('/api/file/delete', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ path: ctxMenu.entry.path })
        })
        if (resp.ok) await loadBrowse()
        else {
            const err = await resp.json()
            alert('删除失败: ' + (err.error || ''))
        }
    } catch (_) { alert('删除失败') }
}

function showCtx(e, item) {
    const relPath = browsePath.value === '/' ? item.name : browsePath.value + '/' + item.name
    ctxMenu.x = e.clientX
    ctxMenu.y = e.clientY
    ctxMenu.entry = { ...item, path: relPath }
    ctxMenu.visible = true
    nextTick(() => clampCtxMenu())
}

function clampCtxMenu() {
    const menu = document.querySelector('.context-menu.visible')
    if (!menu) return
    const w = menu.offsetWidth, h = menu.offsetHeight
    const vw = window.innerWidth, vh = window.innerHeight
    const pad = 8
    ctxMenu.x = Math.max(pad, Math.min(ctxMenu.x, vw - w - pad))
    ctxMenu.y = Math.max(pad, Math.min(ctxMenu.y, vh - h - pad))
}
```

`rename` 方法需通过 `emit` 向上层组件传递，或直接在当前组件调用 `/api/file/rename` 接口。

## API 调用

| 操作 | 接口 | 请求体 |
|------|------|--------|
| 新建文件夹 | `POST /api/projects` | `{ path, name }` |
| 重命名 | `POST /api/file/rename` | `{ path, newName }` |
| 删除 | `POST /api/file/delete` | `{ path }` |

## 边界情况

- **目录为空**：长按无菜单（无 item 可操作）
- **长按后拖动**：识别为滑动操作，不触发菜单（`pressMoved` 标志）
- **删除根目录项目**：后端应已做路径限制，前端不做额外校验
- **重命名冲突**：后端返回错误，前端用 `alert` 提示
