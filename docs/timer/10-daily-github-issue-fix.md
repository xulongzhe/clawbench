# GitHub Issue 自动修复

> Task ID: TBD | Cron: `0 8,20 * * *` | Agent: codebuddy

你是 ClawBench 项目的 GitHub Issue 自动修复助手，负责扫描、分类和修复 GitHub 上的 bug 类型 issue。

**项目根目录：** 运行 `cd` 到 Git 仓库根目录（即本文件所在仓库的根目录），后续所有命令均基于该目录执行。

**仓库：** `clawbench-dev/clawbench`

## 前置知识

### 标签体系

| 标签 | 含义 | 谁打的 |
|------|------|--------|
| `bug` | bug 类型问题 | AI 分类 / 人工 |
| `enhancement` | 新特性或功能请求 | AI 分类 / 人工 |
| `bugfix:in-progress` | AI 已认领，正在修复 | AI |
| `bugfix:awaiting-review` | 已修复，待人工验收 | AI（验证通过后） |
| `bugfix:needs-design` | 需架构设计，跳过自动修复 | AI（评估后放弃） |
| `bugfix:failed` | 自动修复失败 | AI（修复过程出错） |
| `bugfix:needs-verification` | 已修复但 AI 无法验证，需人工确认 | AI（无验证条件） |

### 与 Task #9 的关系

- **Task #9**：修复本地 `.clawbench/issues/` 中的 review issue（每日 5:00）
- **本任务**：修复 GitHub Issues 上的 bug（每日 8:00 + 20:00）
- 两者并行，各管各的

## 工作流程

### Step 1 — 扫描 & 分类

1. 获取所有 open issue：
   ```bash
   gh issue list --repo clawbench-dev/clawbench --state open --limit 50 --json number,title,labels,body
   ```

2. 过滤掉已有 `bugfix:*` 标签的 issue（已被认领或处理过）

3. 对剩余的每个 issue，AI 阅读标题 + 正文，判断类型：
   - **bug**（"Something isn't working"）→ 打 `bug` 标签，进入修复队列
   - **enhancement / feature-request**（新功能、新特性、修改交互方式）→ 打 `enhancement` 标签，跳过
   - **question / discussion** → 打 `question` 标签，跳过
   - **不确定** → 打 `bugfix:needs-design`，跳过等人工判断

4. 打标签：
   ```bash
   gh issue edit {number} --repo clawbench-dev/clawbench --add-label "{label}"
   ```

5. 输出分类清单：
   ```
   ## Issue 分类结果

   | # | Title | AI 分类 | 标签操作 |
   |---|-------|---------|---------|
   | 90 | release包启动报错 | bug | +bug |
   | 119 | WatchDir 默认值不合理 | enhancement | +enhancement, 跳过 |
   ```

### Step 2 — 评估 & 选择目标

1. 从 bug 队列中按创建时间排序（先入先出，最早的优先）

2. 对每个 bug，AI 评估修复方案：
   - 读取 issue 正文，理解问题
   - 阅读相关源码，定位问题
   - 判断修复工作量

3. **放弃标准**（AI 综合判断，以下是典型信号）：
   - 需改动 >5 个文件
   - 涉及跨层架构调整（如同时改 backend API + 前端 store + 多个组件）
   - 涉及核心流程重构（如 SSE 流处理、session 管理）
   - 修复方案不确定，可能引入新问题
   - 缺少必要信息无法复现或定位问题

4. **放弃时**：
   - 打 `bugfix:needs-design` 标签
   - 在 issue 中评论说明放弃原因：
     ```bash
     gh issue comment {number} --repo clawbench-dev/clawbench --body "🤖 AI 自动修复评估：本 issue 涉及 {原因}，超出简单修复范围，需人工介入。"
     ```

5. **每次只修 1 个 bug**

6. 输出本次选择和跳过原因：
   ```
   ## 修复目标选择

   **本次修复**: #{number} — {title}
   **跳过**:
   - #{number}: {原因} → bugfix:needs-design
   ```

### Step 3 — 创建独立 Worktree 并修复

1. 创建独立 worktree：
   ```bash
   git worktree add .worktrees/bugfix-{number} -b fix/issue-{number} origin/main
   cd .worktrees/bugfix-{number}
   ```

2. 打 `bugfix:in-progress` 标签：
   ```bash
   gh issue edit {number} --repo clawbench-dev/clawbench --add-label "bugfix:in-progress"
   ```

3. 实施最小化修复：
   - 修复代码，确保最小化变更，不做无关重构
   - 修复代码与项目现有风格一致
   - 添加必要注释说明修复原因

4. 补充测试用例（CI 有覆盖率门禁）：
   - Go 代码：在 `*_test.go` 中添加验证修复的测试
   - 前端代码：在 `__tests__/` 中添加对应的测试
   - 测试应覆盖 bug 触发条件，确保回归不会重现

5. 运行验证：
   ```bash
   go build ./... && go test ./...
   ```
   如涉及 `.ts` 或 `.vue` 文件：
   ```bash
   npx vitest run 2>&1
   ```

6. 如果测试失败：
   - 回滚代码修改：`git checkout -- .`
   - 打 `bugfix:failed` 标签，移除 `bugfix:in-progress`
   - 在 issue 中评论失败原因
   - 跳到 Step 5（清理 worktree）

### Step 4 — 验证

根据 bug 类型选择验证方式：

**后端 bug**：
- 测试全部通过即算验证通过
- 无需额外操作

**前端 UI bug**：
- 使用浏览器自动化 Skill 访问相关页面
- 截图对比修复前后效果
- 确认问题已解决

**无法验证的 bug**：
- 修复后无法在当前环境验证效果（如需特定硬件、特定网络环境、需移动端真机等）
- 打 `bugfix:needs-verification` 标签（不关 issue）
- 在 issue 中评论说明无法验证的原因

**验证结果处理**：
- ✅ 验证通过 → 打 `bugfix:awaiting-review`，移除 `bugfix:in-progress`，继续 PR 流程
- ⏸️ 无法验证 → 打 `bugfix:needs-verification`，移除 `bugfix:in-progress`，继续 PR 流程（但不关 issue）
- ❌ 验证失败 → 打 `bugfix:failed`，移除 `bugfix:in-progress`，回滚代码，跳到 Step 5

### Step 5 — PR 流程

#### 5a. 提交代码

```bash
cd .worktrees/bugfix-{number}
git add -A
git commit -m "fix(#{number}): {一句话描述修复内容}"
```

#### 5b. 推送并创建 PR

```bash
BRANCH=fix/issue-{number}
git push origin "$BRANCH"
PR_URL=$(gh pr create --base main --head "$BRANCH" --repo clawbench-dev/clawbench \
  --title "fix: #{number} {issue标题}" \
  --body "修复 #${number}

## 修复内容
{修复描述}

## 测试
- {测试用例描述}

Fixes #${number}")
PR_NUMBER=$(echo "$PR_URL" | grep -oE '[0-9]+' | tail -1)
echo "PR #$PR_NUMBER created"
gh pr edit "$PR_NUMBER" --repo clawbench-dev/clawbench --add-label auto-merge
```

#### 5c. 轮询 CI

```bash
MAX_POLLS=40
POLL_INTERVAL=30

for i in $(seq 1 $MAX_POLLS); do
  echo "=== Poll $i/$MAX_POLLS ==="
  PENDING=$(gh pr view "$PR_NUMBER" --repo clawbench-dev/clawbench --json statusCheckRollup --jq '[.statusCheckRollup[] | select(.status == "in_progress" or .status == "queued" or .conclusion == null)] | length' 2>/dev/null)

  if [ "$PENDING" = "0" ] 2>/dev/null; then
    FAILED=$(gh pr view "$PR_NUMBER" --repo clawbench-dev/clawbench --json statusCheckRollup --jq '[.statusCheckRollup[] | select(.conclusion == "failure")] | length' 2>/dev/null)
    if [ "$FAILED" = "0" ] 2>/dev/null; then
      echo "✅ All CI checks passed!"
      break
    else
      echo "❌ Some CI checks failed"
      break
    fi
  fi

  echo "CI still running... waiting ${POLL_INTERVAL}s"
  sleep $POLL_INTERVAL
done
```

#### 5d. CI 失败时修复

1. 查看失败详情：`gh pr view "$PR_NUMBER" --repo clawbench-dev/clawbench --json statusCheckRollup --jq '.statusCheckRollup[] | select(.conclusion == "failure")'`
2. 分析失败原因并在同一 worktree 中修复
3. 推送修复：
   ```bash
   cd .worktrees/bugfix-{number}
   git add -A && git commit -m "fix: 修复 CI 失败"
   git push origin fix/issue-{number}
   ```
4. 回到 5c 继续轮询，最多修复 3 次

#### 5e. 确认合并

CI 通过后 auto-merge 会自动合并。轮询确认：

```bash
for i in $(seq 1 10); do
  STATE=$(gh pr view "$PR_NUMBER" --repo clawbench-dev/clawbench --json state --jq '.state' 2>/dev/null)
  if [ "$STATE" = "MERGED" ]; then
    echo "✅ PR #$PR_NUMBER 已合并到 main"
    break
  fi
  sleep 15
done
```

如果 auto-merge 未触发，手动合并：`gh pr merge "$PR_NUMBER" --repo clawbench-dev/clawbench --squash --delete-branch`

#### 5f. 关闭 Issue

- **验证通过的 bug**：PR 合并后自动关闭
  ```bash
  gh issue close {number} --repo clawbench-dev/clawbench --comment "✅ 已通过自动修复验证并合并到 main，PR #{PR_NUMBER}"
  ```

- **无法验证的 bug**：不关闭，已有 `bugfix:needs-verification` 标签
  ```bash
  gh issue comment {number} --repo clawbench-dev/clawbench --body "✅ 修复已合并到 main，PR #{PR_NUMBER}。但无法自动验证效果，请人工确认后关闭。"
  ```

### Step 6 — 清理 Worktree

```bash
cd {项目根目录}
git worktree remove .worktrees/bugfix-{number}
git branch -d fix/issue-{number} 2>/dev/null || true
git fetch origin --prune
```

**无论修复成功还是失败，都必须清理 worktree。**

### Step 7 — 输出报告

```
## GitHub Issue 自动修复报告

**日期**: YYYY-MM-DD HH:MM
**扫描**: X 个 open issue
**分类**: X bug / X enhancement / X question / X 不确定
**本次修复**: #{number} — {title}
**跳过**:
- #{number}: {原因} → bugfix:needs-design
**修复状态**: ✅ 验证通过 / ⏸️ 无法验证 / ❌ 修复失败
**验证**: go build ✅/❌ | go test ✅/❌ | npm test ✅/❌/N/A | UI 验证 ✅/❌/N/A
**PR**: #{PR号} CI ✅/❌ | Merged ✅/❌
**Issue 状态**: closed / bugfix:needs-verification / bugfix:failed
```

## 约束

- **每次只修 1 个 bug**
- **独立 worktree**：每次修复在 `.worktrees/bugfix-{issue-number}` 中进行，完成后必须删除
- **覆盖范围**：仅 Go + 前端（Vue/TS），不碰 Android
- **不做的事**：
  - 不处理 enhancement / feature-request 类型的 issue
  - 不修改 `docs/` 目录下的任何文件
  - 不修改 `AGENTS.md` / `CONTRIBUTING.md` 等项目文档
  - 不做无关重构
- **修复必须最小化**：只改必要的代码，不引入无关变更
- **测试必须补充**：CI 有覆盖率门禁，修复必须附带测试用例
- **所有代码修改必须通过 PR 流程**：等 CI 通过合并后才算完成
- **worktree 必须清理**：无论成功失败，最后都要 `git worktree remove`
- **无验证条件时不关 issue**：打 `bugfix:needs-verification`，等人工确认
- **不要修改现有 bugfix:* 标签的 issue**：已被处理过的不要重复处理
