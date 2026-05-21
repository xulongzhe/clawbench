# Contribution Guidelines Design

**Date**: 2026-05-22
**Issue**: #23

## Overview

Establish contribution guidelines for ClawBench, covering commit conventions, PR/Issue templates, CONTRIBUTING.md, and CI enforcement.

## Deliverables

| File | Purpose |
|------|---------|
| `.github/ISSUE_TEMPLATE/bug_report.yml` | Bug report template (YAML forms) |
| `.github/ISSUE_TEMPLATE/feature_request.yml` | Feature request template |
| `.github/ISSUE_TEMPLATE/question.yml` | Question/discussion template |
| `.github/ISSUE_TEMPLATE/config.yml` | Disable blank issues, link to docs/discussions |
| `.github/PULL_REQUEST_TEMPLATE.md` | PR description template |
| `.github/workflows/lint-pr.yml` | PR title format lint (CI enforcement) |
| `CONTRIBUTING.md` | Chinese contributing guide |
| `CONTRIBUTING.en.md` | English contributing guide |
| GitHub Settings (manual) | Branch protection rules |

## Design Decisions

### 1. Commit Message Convention

**Decision**: Conventional Commits with Chinese descriptions.

- **type**: English only (tooling dependency — changelog, release notes, semantic versioning)
- **scope**: English module name, optional (not enforced — only 7.5% of existing commits use scope)
- **description**: Chinese preferred (matches 34% existing commits and release.yml Chinese section headers)
- **Spacing**: Space between CJK and Latin characters (Naive UI convention)
- **Punctuation**: No period for Chinese descriptions

**Types**: feat, fix, docs, style, refactor, perf, test, chore, ci, build, revert

**Rationale**: Aligns with existing `release.yml` which parses these types to generate Chinese release notes. The type list matches what release.yml already recognizes plus `test`/`docs`/`revert` for completeness.

### 2. PR Convention

**Decision**: PR title = commit message format (enabled by squash merge).

- Squash merge means PR title becomes the commit message on main
- `amannn/action-semantic-pull-request` validates PR title format in CI
- `subjectPattern: ^.{1,200}$` allows Chinese descriptions

### 3. Issue Templates

**Decision**: Three YAML form templates + config.

- Bug report: description, reproduction steps, expected/actual behavior, platform, environment, logs
- Feature request: problem statement, proposed solution, alternatives, module area
- Question: question text, context
- Config: `blank_issues_enabled: false` to force template usage

### 4. CI Enforcement Strategy

**Decision**: CI-only enforcement, local hooks recommended but not required.

- **P0**: PR title lint via GitHub Action (the only gate into main)
- **P1**: Branch protection requiring status checks
- **P2**: Local commitlint + husky (optional, documented in CONTRIBUTING.md)

**Rationale**: Avoid discouraging new contributors with mandatory local tooling setup. CI is the single source of truth.

### 5. Branch Protection (Manual GitHub Settings)

- Require PR before merging: ✅
- Require status checks: ✅ (test, coverage-go, coverage-frontend, coverage-android)
- Require conversation resolution: ✅
- Require approvals: ⏳ (defer until external contributors join)
- No force pushes, no deletions

### 6. CONTRIBUTING.md Language

**Decision**: Bilingual — `CONTRIBUTING.md` (Chinese) + `CONTRIBUTING.en.md` (English).

GitHub displays `CONTRIBUTING.md` automatically when creating PRs. Both files link to each other via header navigation.

## Alignment with Existing Project

| Existing | Contribution Guidelines |
|----------|------------------------|
| `release.yml` recognizes feat/fix/perf/refactor/chore/style/ci/build + BREAKING CHANGE | Type list matches exactly |
| `auto-merge.yml` squash-merges owner PRs with `auto-merge` label | PR convention supports squash merge model |
| Coverage gate scripts (Go/Frontend/Android) | Documented in CONTRIBUTING.md testing section |
| 34% of commits already use Chinese descriptions | Chinese-first convention matches existing practice |
| AGENTS.md has architecture/build details | CONTRIBUTING.md links to AGENTS.md instead of duplicating |
