# Golang Lint 机制实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**目标：** 为 ClawBench 建立 golangci-lint v2 全量严格的 lint 机制，通过 CI 门禁保障代码质量

**架构：** 使用 golangci-lint v2.12 作为统一 lint 工具，通过 `.golangci.yaml` 配置 25 个 linters + gofumpt 格式化。CI 中新增独立 lint 作业与 test/coverage 并行执行，全部通过才允许合入。

**技术栈：** golangci-lint v2.12.2, golangci-lint-action v7, GitHub Actions, Go 1.25

---

## 文件结构

| 文件 | 操作 | 职责 |
|------|------|------|
| `.golangci.yaml` | 新建 | golangci-lint v2 完整配置（25 linters + gofumpt） |
| `.golangci-lint-version` | 新建 | 版本锁定文件，确保本地和 CI 版本一致 |
| `scripts/lint-go.sh` | 新建 | 本地 lint 运行脚本，支持 --fix/--diff 参数 |
| `build.sh` | 修改 | 在 Go build 前插入 lint 检查步骤 |
| `CONTRIBUTING.md` | 修改 | 更新代码风格说明，增加 lint 使用规范 |
| `.github/workflows/ci.yml` | 修改 | 新增 golangci-lint 作业，移除 go vet 步骤 |
| `.github/workflows/release.yml` | 修改 | 新增 golangci-lint 作业，移除 go vet 步骤 |

---

### Task 1: 添加 .golangci.yaml 配置文件

**Files:**
- Create: `.golangci.yaml`

- [ ] **Step 1: 创建 .golangci.yaml 文件**

```yaml
# golangci-lint v2 配置
# 全量严格模式：所有代码必须通过所有 linters

run:
  timeout: 5m
  tests: true
  go: "1.25"

output:
  sorts-results: true

linters:
  disable-all: true
  enable:
    # --- 默认集（Go 官方基线）---
    - errcheck       # 未检查的错误返回值
    - gosimple       # 代码简化建议
    - govet          # 可疑代码结构
    - ineffassign    # 无效赋值
    - staticcheck    # 综合静态分析
    - typecheck      # 类型检查
    - unused         # 未使用的代码

    # --- 正确性 ---
    - bodyclose      # HTTP 响应体未关闭
    - copyloopvar    # 循环变量捕获问题
    - errorlint      # 错误包装（Go 1.13+）
    - nilerr         # 错误检查后返回 nil
    - nilnesserr     # nil 错误返回或缺少 nil 检查
    - noctx          # HTTP 请求缺少 context
    - rowserrcheck   # SQL rows.Err 未检查
    - sqlclosecheck  # SQL rows/stmt 未关闭

    # --- 安全 ---
    - gosec          # 安全问题扫描

    # --- 代码质量 ---
    - gocritic       # 综合诊断（bug/性能/风格）
    - goconst        # 重复字符串应提取为常量
    - unconvert      # 不必要的类型转换
    - unparam        # 未使用的函数参数
    - revive         # golint 替代品

    # --- 复杂度 ---
    - gocyclo        # 圈复杂度
    - gocognit       # 认知复杂度

    # --- 风格 ---
    - misspell       # 拼写错误
    - nakedret       # 长函数中的裸返回
    - intrange       # 使用 range over int（Go 1.22+）
    - whitespace     # 多余的空白行

    # --- 性能 ---
    - prealloc       # 切片预分配提示

    # --- 指令强制 ---
    - nolintlint     # 要求 //nolint 指令附带原因

formatters:
  enable:
    - gofumpt        # 比 gofmt 更严格的格式化

linters-settings:
  errcheck:
    check-type-assertions: true
    check-blank: false

  gocognit:
    min-complexity: 30

  gocritic:
    enabled-tags:
      - diagnostic
      - style
      - performance
    disabled-checks:
      - hugeParam      # 大结构体参数提示，噪声较大
      - rangeValCopy   # range 值拷贝提示，常见模式噪声

  goconst:
    min-occurrences: 3
    ignore-tests: true

  gocyclo:
    min-complexity: 15

  gosec:
    excludes:
      - G104  # 已由 errcheck 覆盖

  govet:
    enable-all: true
    disable:
      - fieldalignment  # 过于严格，按需单独使用

  misspell:
    locale: US

  nakedret:
    max-func-lines: 30

  nolintlint:
    require-explanation: true
    require-specific: true

  prealloc:
    simple: true
    range-loops: true
    for-loops: false

  revive:
    rules:
      - name: blank-imports
      - name: context-as-argument
      - name: context-keys-type
      - name: dot-imports
      - name: error-return
      - name: error-strings
      - name: error-naming
      - name: exported
      - name: increment-decrement
      - name: indent-error-flow
      - name: package-comments
      - name: range
      - name: receiver-naming
      - name: time-naming
      - name: unexported-return
      - name: var-declaration
      - name: var-naming

  unparam:
    check-exported: false

issues:
  max-issues-per-linter: 50
  max-same-issues: 10

  exclude-rules:
    # 测试文件豁免复杂度和安全检查
    - path: _test\.go
      linters:
        - gocyclo
        - gocognit
        - gosec
        - errcheck

    # cmd/ 和 main.go 豁免 package-comments
    - path: (cmd/|main\.go)
      linters:
        - revive
      text: "package-comments"

severity:
  default-severity: warning
```

- [ ] **Step 2: 创建 .golangci-lint-version 版本锁定文件**

```
v2.12.2
```

- [ ] **Step 3: 提交配置文件**

```bash
git add .golangci.yaml .golangci-lint-version
git commit -m "feat: 添加 golangci-lint v2 配置和版本锁定文件

启用 25 个 linters + gofumpt 格式化，全量严格模式。
包含正确性、安全、代码质量、复杂度、风格、性能、指令强制等维度。"
```

---

### Task 2: 创建本地 lint 脚本

**Files:**
- Create: `scripts/lint-go.sh`

- [ ] **Step 1: 创建 scripts/lint-go.sh 脚本**

```bash
#!/usr/bin/env bash
# lint-go.sh — 本地运行 golangci-lint 检查
#
# 自动检测/安装 golangci-lint，读取 .golangci-lint-version 锁定版本。
#
# 用法：
#   ./scripts/lint-go.sh              # 运行全量 lint
#   ./scripts/lint-go.sh --fix        # 自动修复可修复的问题
#   ./scripts/lint-go.sh --diff       # 仅检查暂存区变更
#
# 退出码：0 = 通过，1 = 有问题

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
VERSION_FILE="$ROOT_DIR/.golangci-lint-version"

cd "$ROOT_DIR"

# 读取锁定版本
LINT_VERSION=""
if [ -f "$VERSION_FILE" ]; then
    LINT_VERSION="$(cat "$VERSION_FILE" | tr -d '[:space:]')"
fi

# 解析参数
FIX_MODE=false
DIFF_MODE=false
EXTRA_ARGS=""

for arg in "$@"; do
    case "$arg" in
        --fix)
            FIX_MODE=true
            EXTRA_ARGS="$EXTRA_ARGS --fix"
            ;;
        --diff)
            DIFF_MODE=true
            ;;
        --help|-h)
            echo "用法: $0 [--fix] [--diff] [--help]"
            echo ""
            echo "  --fix    自动修复可修复的问题"
            echo "  --diff   仅检查暂存区变更（与 master/main 的差异）"
            echo "  --help   显示帮助"
            exit 0
            ;;
        *)
            echo "未知参数: $arg"
            exit 1
            ;;
    esac
done

# 检测/安装 golangci-lint
ensure_golangci_lint() {
    if command -v golangci-lint >/dev/null 2>&1; then
        LOCAL_VERSION="$(golangci-lint version --format short 2>/dev/null || echo "unknown")"
        if [ -n "$LINT_VERSION" ] && [ "$LOCAL_VERSION" != "$LINT_VERSION" ]; then
            echo "⚠️  golangci-lint 版本不匹配: 本地=$LOCAL_VERSION, 要求=$LINT_VERSION"
            echo "   正在安装正确版本..."
            install_golangci_lint
        fi
    else
        echo "⚠️  未找到 golangci-lint，正在安装..."
        install_golangci_lint
    fi
}

install_golangci_lint() {
    if [ -n "$LINT_VERSION" ]; then
        curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | \
            sh -s -- -b "$(go env GOPATH)/bin" "$LINT_VERSION"
    else
        curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | \
            sh -s -- -b "$(go env GOPATH)/bin"
    fi
}

ensure_golangci_lint

# 构建 lint 参数
LINT_ARGS="--timeout=5m"

if [ "$FIX_MODE" = true ]; then
    LINT_ARGS="$LINT_ARGS --fix"
fi

if [ "$DIFF_MODE" = true ]; then
    # 检测 merge-base
    MERGE_BASE=""
    if git rev-parse --verify origin/main &>/dev/null; then
        MERGE_BASE="$(git merge-base origin/main HEAD 2>/dev/null || true)"
    elif git rev-parse --verify origin/master &>/dev/null; then
        MERGE_BASE="$(git merge-base origin/master HEAD 2>/dev/null || true)"
    fi

    if [ -n "$MERGE_BASE" ]; then
        LINT_ARGS="$LINT_ARGS --new-from-rev=$MERGE_BASE"
        echo "🔍 仅检查自 $MERGE_BASE 以来的变更"
    else
        echo "⚠️  无法确定 merge-base，将运行全量检查"
    fi
fi

# 运行 lint
echo "🔍 运行 golangci-lint ($(golangci-lint version --format short 2>/dev/null || echo "unknown"))..."
echo ""

if golangci-lint run $LINT_ARGS ./...; then
    echo ""
    echo "✅ Lint 检查通过"
else
    echo ""
    echo "❌ Lint 检查未通过"
    exit 1
fi
```

- [ ] **Step 2: 设置脚本可执行权限**

```bash
chmod +x scripts/lint-go.sh
```

- [ ] **Step 3: 提交**

```bash
git add scripts/lint-go.sh
git commit -m "feat: 添加本地 Go lint 脚本

支持 --fix（自动修复）和 --diff（仅检查变更），
自动检测/安装指定版本的 golangci-lint。"
```

---

### Task 3: 更新 build.sh 集成 lint

**Files:**
- Modify: `build.sh` (在 Go build 前插入 lint 步骤，步骤编号从 [1/3] 改为 [1/4])

- [ ] **Step 1: 修改 build.sh，在 Go build 前插入 lint 步骤**

将原 `[1/3] Building Go backend...` 改为 `[2/4]`，在其前面插入 lint 步骤，其余步骤编号同步更新。

具体改动：

1. 在第 62 行（`# 1. Build Go backend` 之前）插入 lint 步骤：

```bash
# 1. Lint Go code
echo "[1/4] Linting Go code..."
if command -v golangci-lint >/dev/null 2>&1; then
    ./scripts/lint-go.sh
else
    echo "  golangci-lint not found, skipping lint"
fi
```

2. 将 `# 1. Build Go backend` 的步骤编号从 `[1/3]` 改为 `[2/4]`：
   - 第 64 行: `echo "[1/3] Building Go backend..."` → `echo "[2/4] Building Go backend..."`

3. 将 `# 2. Build Vue frontend` 的步骤编号从 `[2/3]` 改为 `[3/4]`：
   - 第 82 行: `echo "[2/3] Building Vue frontend..."` → `echo "[3/4] Building Vue frontend..."`

4. 将 `# 3. Build Android APK` 的步骤编号从 `[3/3]` 改为 `[4/4]`：
   - 第 98 行: `echo "[3/3] Building Android APK..."` → `echo "[4/4] Building Android APK..."`
   - 第 107 行: `echo "[3/3] Android APK skipped..."` → `echo "[4/4] Android APK skipped..."`

- [ ] **Step 2: 提交**

```bash
git add build.sh
git commit -m "feat: build.sh 集成 golangci-lint 检查步骤

在 Go build 前新增 lint 步骤，构建步骤编号更新为 [1/4]-[4/4]。"
```

---

### Task 4: 更新 CONTRIBUTING.md

**Files:**
- Modify: `CONTRIBUTING.md` (第 149 行代码风格说明 + 测试章节新增 lint 说明)

- [ ] **Step 1: 更新代码风格说明**

将第 149 行：
```
- **Go**：`gofmt` + `go vet`
```
替换为：
```
- **Go**：`gofumpt`（通过 golangci-lint）+ `golangci-lint v2` 全量严格
```

- [ ] **Step 2: 在测试章节（第 155-158 行之后）新增 lint 章节**

在 `npm test             # 前端全量测试` 之后添加：

```markdown

## Lint

```bash
./scripts/lint-go.sh              # Go 全量 lint
./scripts/lint-go.sh --fix        # 自动修复可修复的问题
./scripts/lint-go.sh --diff       # 仅检查暂存区变更
```

### nolint 使用规范

仅在真正合理的场景使用 `//nolint`，且必须附带原因说明：

```go
//nolint:errcheck // 有意忽略：只读响应的 Close() 错误
resp.Body.Close()
```

禁止：
- 不带原因的 `//nolint`
- 为绕过问题批量添加 `//nolint`
- 降低 linter 阈值来绕过问题
```

- [ ] **Step 3: 提交**

```bash
git add CONTRIBUTING.md
git commit -m "docs: 更新 CONTRIBUTING.md 代码风格和 lint 使用说明

Go 代码风格更新为 gofumpt + golangci-lint v2，
新增 lint 运行说明和 nolint 使用规范。"
```

---

### Task 5: 本地运行 lint 获取问题基线

**Files:**
- 无文件变更，仅运行分析

- [ ] **Step 1: 安装 golangci-lint v2.12.2**

```bash
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b "$(go env GOPATH)/bin" v2.12.2
```

- [ ] **Step 2: 运行全量 lint 获取问题清单**

```bash
./scripts/lint-go.sh 2>&1 | tee /tmp/lint-baseline.txt
```

- [ ] **Step 3: 统计各 linter 问题数量**

```bash
# 按错误类型分组统计
golangci-lint run --timeout=5m ./... 2>&1 | grep -oP '(?<= )\w+$' | sort | uniq -c | sort -rn
```

预期：获得完整问题清单，用于后续按优先级分批修复。

---

### Task 6: 修复 P0 — 正确性/安全问题

**Files:**
- Modify: 涉及 bodyclose、noctx、errorlint、gosec、nilerr、rowserrcheck、sqlclosecheck 报告的所有 Go 文件

**说明：** 此任务的具体步骤取决于 Task 5 的 lint 输出结果。每个修复遵循以下模式：

- [ ] **Step 1: 收集 P0 问题清单**

```bash
golangci-lint run --timeout=5m --no-config ./... 2>&1 | \
  grep -E '(bodyclose|noctx|errorlint|gosec|nilerr|rowserrcheck|sqlclosecheck)' | \
  sort -t: -k1,1 -k2,2n
```

- [ ] **Step 2: 逐文件修复 P0 问题**

修复模式（按 linter 类型）：

- **bodyclose**: 在 `resp.Body` 读取完成后添加 `defer resp.Body.Close()`，或确保已有关闭调用
- **noctx**: 将 `http.NewRequest` 改为 `http.NewRequestWithContext(ctx, ...)`
- **errorlint**: 将 `err.(type)` 改为 `errors.As`，将 `fmt.Errorf("...: %v", err)` 改为 `fmt.Errorf("...: %w", err)`
- **gosec**: 根据具体规则修复（如 G101 硬编码凭据、G401 弱加密等）
- **nilerr**: 修正 `if err != nil { return nil }` 为 `if err != nil { return err }` 或适当处理
- **rowserrcheck**: 在 `rows.Next()` 循环后添加 `return rows.Err()`
- **sqlclosecheck**: 确保Rows/Stmt有`defer rows.Close()`或`defer stmt.Close()`

- [ ] **Step 3: 验证 P0 修复**

```bash
golangci-lint run --timeout=5m --no-config ./... 2>&1 | \
  grep -E '(bodyclose|noctx|errorlint|gosec|nilerr|rowserrcheck|sqlclosecheck)' || echo "P0 全部修复"
```

预期输出：`P0 全部修复`

- [ ] **Step 4: 运行测试确保无回归**

```bash
go test ./...
```

预期：所有测试通过

- [ ] **Step 5: 提交**

```bash
git add -A
git commit -m "fix: 修复 P0 lint 问题（正确性/安全）

修复 bodyclose、noctx、errorlint、gosec、nilerr、
rowserrcheck、sqlclosecheck 报告的所有问题。"
```

---

### Task 7: 修复 P1 — 代码质量问题

**Files:**
- Modify: 涉及 errcheck、gocritic、goconst、unconvert、unparam 报告的所有 Go 文件

- [ ] **Step 1: 收集 P1 问题清单**

```bash
golangci-lint run --timeout=5m --no-config ./... 2>&1 | \
  grep -E '(errcheck|gocritic|goconst|unconvert|unparam)' | \
  sort -t: -k1,1 -k2,2n
```

- [ ] **Step 2: 逐文件修复 P1 问题**

修复模式：

- **errcheck**: 对未检查的返回值添加错误处理（`if err != nil { ... }`），或在合理场景添加 `//nolint:errcheck // 原因说明`
- **gocritic**: 根据 gocritic 提示的 diagnostic/style/performance 建议逐一修改
- **goconst**: 将重复 3+ 次的字符串提取为常量
- **unconvert**: 移除不必要的类型转换（如 `int(x)` 中 x 本身就是 int）
- **unparam**: 移除未使用的函数参数或返回值

- [ ] **Step 3: 验证 P1 修复**

```bash
golangci-lint run --timeout=5m --no-config ./... 2>&1 | \
  grep -E '(errcheck|gocritic|goconst|unconvert|unparam)' || echo "P1 全部修复"
```

- [ ] **Step 4: 运行测试确保无回归**

```bash
go test ./...
```

- [ ] **Step 5: 提交**

```bash
git add -A
git commit -m "fix: 修复 P1 lint 问题（代码质量）

修复 errcheck、gocritic、goconst、unconvert、unparam 报告的所有问题。"
```

---

### Task 8: 修复 P2 — 风格/复杂度问题

**Files:**
- Modify: 涉及 revive、nakedret、misspell、gocyclo、gocognit、whitespace 报告的所有 Go 文件

- [ ] **Step 1: 先用 --fix 自动修复格式化问题**

```bash
golangci-lint run --timeout=5m --fix ./...
```

此步骤自动修复：misspell（拼写）、whitespace（空白行）、gofumpt（格式化）。

- [ ] **Step 2: 收集剩余 P2 问题清单**

```bash
golangci-lint run --timeout=5m --no-config ./... 2>&1 | \
  grep -E '(revive|nakedret|gocyclo|gocognit)' | \
  sort -t: -k1,1 -k2,2n
```

- [ ] **Step 3: 逐文件修复剩余 P2 问题**

修复模式：

- **revive**: 根据具体规则修改（如导出函数缺注释、receiver 命名不一致等）
- **nakedret**: 在超过 30 行的函数中，将裸返回改为显式返回值
- **gocyclo**: 圈复杂度 >15 的函数需要拆分为更小的子函数
- **gocognit**: 认知复杂度 >30 的函数需要拆分

- [ ] **Step 4: 验证 P2 修复**

```bash
golangci-lint run --timeout=5m --no-config ./... 2>&1 | \
  grep -E '(revive|nakedret|misspell|gocyclo|gocognit|whitespace)' || echo "P2 全部修复"
```

- [ ] **Step 5: 运行测试确保无回归**

```bash
go test ./...
```

- [ ] **Step 6: 提交**

```bash
git add -A
git commit -m "fix: 修复 P2 lint 问题（风格/复杂度）

修复 revive、nakedret、misspell、gocyclo、gocognit、
whitespace 报告的所有问题，gofumpt 格式化已自动应用。"
```

---

### Task 9: 修复 P3 — 性能问题

**Files:**
- Modify: 涉及 prealloc 报告的所有 Go 文件

- [ ] **Step 1: 收集 P3 问题清单**

```bash
golangci-lint run --timeout=5m --no-config ./... 2>&1 | \
  grep -E 'prealloc' | sort -t: -k1,1 -k2,2n
```

- [ ] **Step 2: 逐文件修复 prealloc 问题**

修复模式：在已知大小的切片赋值前添加 `make([]T, 0, len(source))` 预分配。

- [ ] **Step 3: 验证 P3 修复**

```bash
golangci-lint run --timeout=5m --no-config ./... 2>&1 | \
  grep -E 'prealloc' || echo "P3 全部修复"
```

- [ ] **Step 4: 运行测试确保无回归**

```bash
go test ./...
```

- [ ] **Step 5: 全量 lint 验证**

```bash
golangci-lint run --timeout=5m ./...
```

预期：零错误，lint 全量通过

- [ ] **Step 6: 提交**

```bash
git add -A
git commit -m "fix: 修复 P3 lint 问题（性能）

修复 prealloc 报告的所有问题。Lint 全量通过。"
```

---

### Task 10: 更新 CI — ci.yml

**Files:**
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: 在 ci.yml 中新增 golangci-lint 作业**

在 `test` 作业之前（第 9 行 `jobs:` 之后）插入新作业：

```yaml
  golangci-lint:
    name: Lint (Go)
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'

      - name: golangci-lint
        uses: golangci/golangci-lint-action@v7
        with:
          version: v2.12
          args: --timeout=5m
```

- [ ] **Step 2: 从 test 作业中移除 go vet 步骤**

删除第 23-24 行：
```yaml
      - name: Go vet
        run: go vet ./...
```

（govet 已包含在 golangci-lint 中，无需重复运行）

- [ ] **Step 3: 提交**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: 新增 golangci-lint 作业，移除 go vet

Lint 作业与 test/coverage 并行执行，govet 已包含在
golangci-lint 中，移除重复的 go vet 步骤。"
```

---

### Task 11: 更新 CI — release.yml

**Files:**
- Modify: `.github/workflows/release.yml`

- [ ] **Step 1: 在 release.yml 中新增 golangci-lint 作业**

在 `test` 作业之前（第 8 行 `jobs:` 之后）插入新作业：

```yaml
  golangci-lint:
    name: Lint (Go)
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'

      - name: golangci-lint
        uses: golangci/golangci-lint-action@v7
        with:
          version: v2.12
          args: --timeout=5m

  test:
    needs: golangci-lint
```

注意：在原有 `test` 作业上添加 `needs: golangci-lint`，确保 lint 通过后才执行测试和发布构建。

- [ ] **Step 2: 从 test 作业中移除 go vet 步骤**

删除第 19-20 行：
```yaml
      - name: Go vet
        run: go vet ./...
```

- [ ] **Step 3: 提交**

```bash
git add .github/workflows/release.yml
git commit -m "ci: release.yml 新增 golangci-lint 作业，移除 go vet

Lint 通过后才执行 test 和后续发布构建。"
```

---

### Task 12: 最终验证

**Files:**
- 无文件变更

- [ ] **Step 1: 本地全量 lint 验证**

```bash
./scripts/lint-go.sh
```

预期：✅ Lint 检查通过

- [ ] **Step 2: 全量测试验证**

```bash
go test ./...
```

预期：所有测试通过

- [ ] **Step 3: build.sh 完整构建验证**

```bash
./build.sh
```

预期：[1/4] Lint → [2/4] Go backend → [3/4] Vue frontend → [4/4] Android APK (skipped) → Build complete

- [ ] **Step 4: 覆盖率门禁验证**

```bash
./scripts/check-go-coverage.sh
```

预期：覆盖率门禁通过

- [ ] **Step 5: 确认所有文件已提交**

```bash
git status
```

预期：无未提交的变更
