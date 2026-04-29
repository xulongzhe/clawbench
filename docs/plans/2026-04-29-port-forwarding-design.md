# 端口转发功能设计

> 日期：2026-04-29
> 状态：草案

## 1. 问题陈述

ClawBench Android 客户端是一个 WebView 壳，连接远程 ClawBench 服务器。当 AI Agent 在服务器上启动 HTTP/WebSocket 服务（如 `npm run dev` 在 `:5173`），Android 设备无法直接访问这些端口：

- `localhost:5173` 是服务器的 localhost，不是手机的
- 即使通过网络 IP 访问，可能存在防火墙、CORS、HTTPS 混合内容等问题
- WebView 的 `shouldOverrideUrlLoading` 会将非同源链接踢到系统浏览器

需要类似 VS Code Remote 的端口转发能力：通过 ClawBench 服务器作为代理，将服务器本地端口映射到 ClawBench 的 HTTP 路径下，Android WebView 通过同源访问即可打开这些服务。

## 2. 核心设计

### 2.1 反向代理方案

使用 Go 标准库 `net/http/httputil.ReverseProxy`，将特定路径前缀的请求代理到服务器本地端口。

**路由规则：**

```
/proxy/{port}/{path}  →  http://127.0.0.1:{port}/{path}
```

示例：
- `/proxy/5173/` → `http://127.0.0.1:5173/`
- `/proxy/5173/index.html` → `http://127.0.0.1:5173/index.html`
- `/proxy/3000/api/users` → `http://127.0.0.1:3000/api/users`

**WebSocket 支持：** `httputil.ReverseProxy` 默认不处理 WebSocket。需要检测 `Upgrade: websocket` 请求头，使用 `gorilla/websocket` 或 Go 1.22+ 的标准库手动完成 WebSocket 代理（客户端 ↔ 后端双向转发）。

### 2.2 端口白名单 + 动态注册

**不允许代理任意端口**（安全风险太大），而是采用白名单 + 动态注册机制：

1. **配置白名单**（`config.yaml`）：管理员预先声明允许转发的端口
2. **API 动态注册**：AI Agent 通过 tool_use 事件触发端口注册，或用户手动添加
3. **自动检测**：后端可选地监听服务器上新启动的 TCP 端口（类似 VS Code 的自动检测行为）

### 2.3 APP 模式限定

端口转发 API 和路由仅在检测到 APP 模式时启用。检测方式：

- **后端**：新增 `--app-mode` 启动参数，或通过 config 中 `app_mode: true` 配置
- **前端**：检查 `window.AndroidNative?.isNativeApp()` 或 User-Agent 中的 `ClawBench-Android`
- **Android 壳**：启动时在 server URL 后附加查询参数 `?app=1`，或通过 JS bridge 通知前端

实际实现中，后端路由始终注册（因为后端本身无法区分请求来源是 APP 还是浏览器），但前端 UI 只在 APP 模式下显示端口转发面板。API 需要认证（`middleware.Auth`），未认证请求无法利用代理。

## 3. 架构设计

### 3.1 后端新增文件

```
internal/
  handler/
    proxy.go              # 代理 handler（HTTP + WebSocket）
    proxy_api.go          # 端口注册/查询/删除 API
  service/
    proxy.go              # 端口转发业务逻辑（白名单管理、连接跟踪）
  model/
    proxy.go              # ForwardedPort 数据模型
```

### 3.2 数据模型

```go
// model/proxy.go
type ForwardedPort struct {
    Port       int       `json:"port"`        // 服务器本地端口号
    Name       string    `json:"name"`        // 显示名称，如 "Vite Dev Server"
    Source     string    `json:"source"`      // 来源："config" | "auto" | "manual"
    RegisteredAt time.Time `json:"registered_at"`
    Active     bool      `json:"active"`      // 端口是否有服务在监听
}
```

### 3.3 API 设计

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/proxy/ports` | 获取已注册的转发端口列表（含活跃状态） |
| POST | `/api/proxy/ports` | 手动注册端口 `{port, name}` |
| DELETE | `/api/proxy/ports/{port}` | 删除端口注册 |
| GET | `/proxy/{port}/{path}` | 代理请求到 `http://127.0.0.1:{port}/{path}` |

所有 `/api/proxy/*` 接口需要 `middleware.Auth` 认证。`/proxy/*` 路由同样需要认证。

### 3.4 配置扩展

```yaml
# config.yaml
proxy:
  enabled: true                  # 是否启用端口转发（默认 true）
  allowed_ports: []              # 白名单，空数组=允许所有已注册端口
  auto_detect: true              # 自动检测新端口（默认 true）
  max_port: 65535                # 允许转发的最大端口号
  allow_localhost_only: true     # 只允许转发 127.0.0.1 的端口（默认 true）
```

### 3.5 HTTP 代理实现（核心）

```go
// handler/proxy.go

func ServeProxy(w http.ResponseWriter, r *http.Request) {
    // 1. 从 URL 解析端口号：/proxy/{port}/...
    // 2. 检查端口是否在注册列表中
    // 3. 如果是 WebSocket 升级请求，走 WebSocket 代理
    // 4. 否则使用 httputil.ReverseProxy
}

func serveHTTPProxy(w http.ResponseWriter, r *http.Request, targetPort int) {
    target := &url.URL{
        Scheme: "http",
        Host:   fmt.Sprintf("127.0.0.1:%d", targetPort),
    }
    proxy := httputil.NewSingleHostReverseProxy(target)

    // 重写请求路径：/proxy/5173/foo → /foo
    origDirector := proxy.Director
    proxy.Director = func(req *http.Request) {
        origDirector(req)
        // 从 /proxy/{port}/xxx 中提取 /xxx
        req.URL.Path = stripProxyPrefix(req.URL.Path, targetPort)
        req.Host = target.Host
    }

    // 修改响应：重写 HTML 中的绝对 URL
    proxy.ModifyResponse = rewriteResponseBody

    proxy.ServeHTTP(w, r)
}
```

### 3.6 WebSocket 代理实现

```go
func serveWebSocketProxy(w http.ResponseWriter, r *http.Request, targetPort int) {
    // 1. 拨号到后端 WebSocket
    targetURL := fmt.Sprintf("ws://127.0.0.1:%d%s", targetPort, backendPath)
    backendConn, _, err := websocket.DefaultDialer.Dial(targetURL, nil)

    // 2. 升级客户端连接
    upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
    clientConn, err := upgrader.Upgrade(w, r, nil)

    // 3. 双向转发
    go func() {
        // backend → client
        for {
            msgType, msg, err := backendConn.ReadMessage()
            if err != nil { break }
            clientConn.WriteMessage(msgType, msg)
        }
    }()
    // client → backend
    for {
        msgType, msg, err := clientConn.ReadMessage()
        if err != nil { break }
        backendConn.WriteMessage(msgType, msg)
    }
}
```

### 3.7 自动端口检测

```go
// service/proxy.go

// 定期扫描服务器上新增的 TCP 监听端口
func (s *ProxyService) startAutoDetect(ctx context.Context) {
    ticker := time.NewTicker(5 * time.Second)
    for {
        select {
        case <-ticker.C:
            s.detectNewPorts()
        case <-ctx.Done():
            return
        }
    }
}

func (s *ProxyService) detectNewPorts() {
    // 读取 /proc/net/tcp（Linux）或使用 netstat 解析
    // 对比已知端口列表，发现新端口则自动注册
    // 过滤掉系统端口（< 1024）和 ClawBench 自身端口
}
```

### 3.8 HTML 响应重写

代理的 HTML 响应中可能包含绝对 URL（如 `<script src="http://localhost:5173/...">`），需要在代理层重写为 `/proxy/5173/...`。

```go
func rewriteResponseBody(resp *http.Response) error {
    ct := resp.Header.Get("Content-Type")
    if !strings.Contains(ct, "text/html") && !strings.Contains(ct, "javascript") {
        return nil
    }
    // 读取响应体，替换 http://localhost:{port} → /proxy/{port}
    // 替换 ws://localhost:{port} → ws://{clawbench-host}/proxy/{port}
    // 写回修改后的响应体
}
```

## 4. 前端设计

### 4.1 端口转发面板

在 Chat 面板底部 dock 栏旁新增端口转发入口（仅在 APP 模式显示），或作为 Chat 面板内的一个子面板。

**UI 组件：**

```
web/src/components/proxy/
  ProxyPanel.vue          # 端口转发主面板（BottomSheet）
  ProxyPortList.vue       # 端口列表
  ProxyPortItem.vue       # 单个端口条目（端口、名称、状态、操作按钮）
  ProxyAddDialog.vue      # 手动添加端口对话框
```

**端口条目信息：**
- 端口号 + 名称
- 状态指示灯（绿色=活跃，灰色=离线）
- "打开" 按钮：在 WebView 中导航到 `/proxy/{port}/`
- "复制链接" 按钮：复制完整 URL 供分享
- "删除" 按钮：移除端口注册

### 4.2 AI 消息中的端口链接

当 AI Agent 的 tool_use 输出中包含 `localhost:XXXX` 或 `127.0.0.1:XXXX` 模式时，前端自动：

1. 将文本中的 `http://localhost:XXXX/...` 转换为 `/proxy/XXXX/...` 链接
2. 在消息下方显示"已检测到端口 XXXX"提示条，一键打开或注册

### 4.3 APP 模式检测

```typescript
// composables/useAppMode.ts
export function useAppMode() {
  const isAppMode = ref(false)

  onMounted(() => {
    // 方法1: JS Bridge
    if (window.AndroidNative?.isNativeApp?.()) {
      isAppMode.value = true
      return
    }
    // 方法2: User-Agent
    if (navigator.userAgent.includes('ClawBench-Android')) {
      isAppMode.value = true
    }
  })

  return { isAppMode }
}
```

### 4.4 WebView 内打开转发页面

Android 端需要修改 `shouldOverrideUrlLoading`，允许 `/proxy/` 路径在 WebView 内加载（当前只有同源和 localhost 允许，`/proxy/` 路径本身就是同源的，所以应该已经可以工作）。

## 5. Android 端改动

### 5.1 修改 `shouldOverrideUrlLoading`

当前逻辑只允许同源 host 在 WebView 内加载。`/proxy/` 路径是同源的，无需修改此逻辑。

### 5.2 新增 JS Bridge 方法（可选）

```java
@JavascriptInterface
public void openProxyPort(int port) {
    // 在 WebView 中导航到 /proxy/{port}/
    runOnUiThread(() -> webView.loadUrl(serverUrl + "/proxy/" + port + "/"));
}
```

## 6. 安全考量

1. **认证**：所有代理请求必须经过 `middleware.Auth` 认证
2. **端口限制**：只代理已注册的端口，不允许代理任意端口
3. **仅 localhost**：默认只代理 `127.0.0.1` 的端口，不代理内网其他机器
4. **系统端口过滤**：自动检测时忽略 < 1024 的端口
5. **CSRF**：代理请求来源受 cookie 认证保护

## 7. 依赖

| 依赖 | 用途 | 是否需要新增 |
|------|------|------------|
| `net/http/httputil` | HTTP 反向代理 | 标准库，无需新增 |
| `gorilla/websocket` | WebSocket 代理 | **需新增** |
| Go 1.22+ | `http.NewServeMux` 路由参数 | 已满足 |

## 8. 实现步骤

### Phase 1：最小可用（HTTP 代理 + 手动注册）
1. 后端：`model/proxy.go` — 数据模型 + Config 扩展
2. 后端：`service/proxy.go` — 端口注册管理（内存存储，无需 SQLite）
3. 后端：`handler/proxy.go` — HTTP 反向代理 handler
4. 后端：`handler/proxy_api.go` — CRUD API
5. 后端：`handler/handler.go` — 注册新路由
6. 前端：`useAppMode.ts` — APP 模式检测
7. 前端：`ProxyPanel.vue` + 子组件 — 端口管理 UI
8. 前端：集成到 App.vue 的 dock 栏

### Phase 2：WebSocket 支持
9. 后端：WebSocket 代理实现
10. 验证 Vite HMR、Jupyter 等场景

### Phase 3：自动检测 + HTML 重写
11. 后端：自动端口检测（`/proc/net/tcp` 解析）
12. 后端：HTML 响应重写（localhost URL 替换）
13. 前端：AI 消息中的端口链接自动转换

### Phase 4：体验优化
14. 前端：端口活跃状态实时更新（SSE 或轮询）
15. Android：新增 `openProxyPort` JS Bridge
16. 端口转发链接在 iframe 中打开（避免离开主界面）

## 9. 备选方案

### 方案 B：SSH 隧道（不推荐）

在 Android 端建立 SSH 隧道到服务器，通过本地端口映射实现转发。

- **优点**：不修改后端，通用性强
- **缺点**：需要 SSH 服务端配置，Android 端需要 SSH 客户端库，用户体验差，连接管理复杂

### 方案 C：Android 本地代理服务（不推荐）

在 Android 端启动一个本地 HTTP 代理服务，通过 ClawBench 的 WebSocket/API 隧道转发请求。

- **优点**：可以处理任意协议
- **缺点**：实现极其复杂，需要自研隧道协议，延迟高，Android 后台服务管理困难

**推荐方案 A（反向代理）**，理由：
- Go 标准库直接支持，实现简单
- 请求路径同源，无需处理 CORS/混合内容
- 与现有认证体系无缝集成
- Android 端几乎无需改动
