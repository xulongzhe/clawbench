# Golang Lint 机制设计

**日期：** 2025-07-27
**状态：** 已批准
**策略：** 全量严格 — 所有存量代码必须通过所有 linter 后 CI 门禁才生效

## 背景

ClawBench 拥有 247 个 Go 文件（约 85K 行），目前仅有 `go vet` 作为静态分析工具。无 golangci-lint、无格式化强制、无安全扫描。现有 CI 有完善的两级覆盖率门禁，但缺少 lint 门禁。

## 决策

采用 golangci-lint v2 作为统一 lint 工具，启用完整 linter 集合，通过 CI 强制执行作为与覆盖率门禁并列的必需质量门禁。

## Linter 配置

### 启用的 Linters（25 个）

| 分类 | Linters | 用途 |
|------|---------|------|
| **默认集（6）** | errcheck, gosimple, govet, ineffassign, staticcheck, unused | Go 官方基线 |
| **正确性** | bodyclose, copyloopvar, errorlint, nilerr, noctx, rowserrcheck, sqlclosecheck, nilnesserr | 运行时缺陷预防 |
| **安全** | gosec | SQL 注入、硬编码凭据、弱加密 |
| **代码质量** | gocritic, goconst, unconvert, unparam, revive | 代码改进建议 |
| **复杂度** | gocyclo（≤15）, gocognit（≤30） | 防止过度复杂的函数 |
| **风格** | misspell, nakedret, intrange, whitespace | 一致性 |
| **性能** | prealloc | 切片预分配提示 |
| **指令强制** | nolintlint | 要求所有 `//nolint` 指令附带原因说明 |

### 启用的 Formatters（1 个）

- `gofumpt` — 比 gofmt 更严格的格式化

### 关键配置

- `errcheck`: `check-type-assertions: true`，`check-blank: false`
- `gocritic`: 启用标签 diagnostic + style + performance；排除检查 hugeParam、rangeValCopy
- `goconst`: `min-occurrences: 3`，`ignore-tests: true`
- `gosec`: 排除 G104（已由 errcheck 覆盖）
- `govet`: `enable-all: true`，排除 fieldalignment
- `revive`: 17 条规则（blank-imports, context-as-argument, context-keys-type, dot-imports, error-return, error-strings, error-naming, exported, increment-decrement, indent-error-flow, package-comments, range, receiver-naming, time-naming, unexported-return, var-declaration, var-naming）
- `nakedret`: `max-func-lines: 30`
- `unparam`: `check-exported: false`
- `prealloc`: simple + range-loops，不包括 for-loops
- `misspell`: locale US

### 豁免规则

- `_test.go` 文件：豁免 gocyclo、gocognit、gosec、errcheck
- `cmd/` 和 `main.go`：豁免 revive 的 package-comments 规则

### 问题数量限制

- `max-issues-per-linter: 50`
- `max-same-issues: 10`
- `default-severity: warning`

### nolintlint 规则

所有 `//nolint` 指令必须附带原因说明：
```go
//nolint:errcheck // 有意忽略：只读响应的 Close() 错误
resp.Body.Close()
```

## CI 集成

### 新增 lint 作业（与 test 并行）

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

### CI 改动

1. 在 `ci.yml` 中新增 `golangci-lint` 作业（与 `test` 并行）
2. 从 `test` 作业中移除 `go vet` 步骤（govet 已包含在 golangci-lint 中）
3. 在 `release.yml` 中新增 `golangci-lint` 作业 + 移除 `go vet`
4. 所有质量门禁必须通过：Lint + Test + Coverage (Go) + Coverage (FE)

### 版本锁定

新增 `.golangci-lint-version` 文件，内容为 `v2.12`，确保本地和 CI 使用同一版本。

### 质量门禁全景

```
PR → Lint (Go) ─────┐
     Test (3 OS) ────┤
     Coverage (Go) ──┤── 全部通过 → 允许合入
     Coverage (FE) ──┤
     Build Frontend ─┘
```

## 本地开发

### scripts/lint-go.sh

- 自动检测/安装 golangci-lint（读取 `.golangci-lint-version`）
- 运行 `golangci-lint run --timeout=5m ./...`
- 支持参数：`--fix`（自动修复）、`--diff`（仅检查暂存区变更）

### build.sh 集成

在 `go build` 之前增加 lint 检查步骤，调用 `scripts/lint-go.sh`。

### CONTRIBUTING.md 更新

- 旧：`Go: gofmt + go vet`
- 新：`Go: gofumpt（通过 golangci-lint）+ golangci-lint v2`
- 增加 lint 运行说明和 `//nolint` 使用规范

## 实施阶段

| 阶段 | 内容 | 产出 |
|------|------|------|
| **阶段 1** | 添加 `.golangci.yaml` + `.golangci-lint-version`，本地运行获取完整问题清单 | 问题基线 |
| **阶段 2** | 新增 `scripts/lint-go.sh` + 更新 `build.sh` + 更新 `CONTRIBUTING.md` | 本地工具就绪 |
| **阶段 3** | 按优先级修复所有存量问题（P0→P1→P2→P3） | 全量通过 |
| **阶段 4** | 新增 CI lint 作业 + 移除 `go vet` + 更新 `release.yml` | CI 门禁生效 |

### 修复优先级

1. **P0 — 正确性/安全：** bodyclose, noctx, errorlint, gosec, nilerr, rowserrcheck, sqlclosecheck → 立即修复
2. **P1 — 质量：** errcheck, gocritic, goconst, unconvert, unparam → 功能正确但有改进空间
3. **P2 — 风格/复杂度：** revive, nakedret, misspell, gocyclo, gocognit, gofumpt → 格式化和重构
4. **P3 — 性能：** prealloc → 性能优化

### 修复原则

- 不降低 linter 阈值来绕过问题
- 不批量添加 `//nolint` — 仅用于真正合理的场景且必须附带原因
- 格式化问题（gofumpt、whitespace、misspell）可通过 `--fix` 批量自动修复
