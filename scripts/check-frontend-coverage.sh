#!/usr/bin/env bash
# check-frontend-coverage.sh — Run frontend tests with coverage and enforce coverage gate.
# Gate rule: per-directory statements coverage >= min(baseline, 80%).
# Excluded: src/i18n/locales (translation dictionaries, not testable)
#
# Usage:
#   ./scripts/check-frontend-coverage.sh              # run tests + check
#   ./scripts/check-frontend-coverage.sh --skip-test  # skip tests, use existing coverage
#   ./scripts/check-frontend-coverage.sh --update      # update baseline with current coverage
#
# Exit code: 0 = pass, 1 = fail

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
WEB_DIR="$ROOT_DIR/web"
BASELINE_FILE="$ROOT_DIR/coverage-baseline.json"

SKIP_TEST=false
UPDATE_BASELINE=false

for arg in "$@"; do
  case "$arg" in
    --skip-test) SKIP_TEST=true ;;
    --update) UPDATE_BASELINE=true ;;
    --help|-h)
      echo "Usage: $0 [--skip-test] [--update] [--help]"
      exit 0 ;;
  esac
done

cd "$ROOT_DIR"

# Run Python to execute vitest, parse coverage, and check gate
python3 - "$BASELINE_FILE" "$WEB_DIR" "$SKIP_TEST" "$UPDATE_BASELINE" << 'PYTHON_SCRIPT'
import json, sys, subprocess, re

baseline_file = sys.argv[1]
web_dir = sys.argv[2]
skip_test = sys.argv[3] == "true"
update_baseline = sys.argv[4] == "true"
MIN_FLOOR = 80.0
EXCLUDED = {"src/i18n/locales"}

# Load baseline
with open(baseline_file) as f:
    baseline_data = json.load(f)
fe_baseline = baseline_data.get("frontend", {})

# Run vitest with coverage and capture output
if skip_test:
    print("==> Skipping test run, re-running vitest to capture coverage...")
else:
    print("==> Running frontend tests with coverage...")

result = subprocess.run(
    ["npx", "vitest", "run", "--coverage"],
    cwd=web_dir, capture_output=True, text=True
)
output = result.stdout + result.stderr

# Check if tests passed
if result.returncode != 0:
    # Print test output so failures are visible
    print(output)
    print("ERROR: Frontend tests failed. Fix test failures before checking coverage.")
    sys.exit(1)

# Parse directory-level coverage from output
# Format: " src/components    |     100 |      100 |     100 |     100 |"
current = {}
for line in output.split("\n"):
    line = line.strip()
    if not line.startswith("src/"):
        continue
    parts = [p.strip() for p in line.split("|")]
    if len(parts) < 5:
        continue
    dir_name = parts[0]
    try:
        stmts = float(parts[1])
        branches = float(parts[2])
        funcs = float(parts[3])
        lines = float(parts[4])
        current[dir_name] = {
            "statements": stmts,
            "branches": branches,
            "functions": funcs,
            "lines": lines
        }
    except (ValueError, IndexError):
        continue

# Report
BOLD = "\033[1m"
RED = "\033[0;31m"
GREEN = "\033[0;32m"
YELLOW = "\033[0;33m"
RESET = "\033[0m"

print()
print(f"{BOLD}╔══════════════════════════════════════════════════════════════════╗{RESET}")
print(f"{BOLD}║              Frontend Coverage Gate Check                     ║{RESET}")
print(f"{BOLD}║  Rule: statements >= min(baseline, {MIN_FLOOR}%)                      ║{RESET}")
print(f"{BOLD}╚══════════════════════════════════════════════════════════════════╝{RESET}")
print()

all_pass = True
updated_dirs = {}

print(f"{BOLD}{'Directory':<25} {'Base%':>8} {'Curr%':>8} {'Floor':>8}  {'Status':<8}{RESET}")
print(f"{'─'*25} {'─'*8} {'─'*8} {'─'*8}  {'─'*8}")

# Check all baseline dirs + any new ones
all_dirs = sorted(set(list(fe_baseline.keys()) + list(current.keys())))

for dir_name in all_dirs:
    if dir_name in EXCLUDED:
        baseline_pct = fe_baseline.get(dir_name, {}).get("statements", 0)
        curr_pct = current.get(dir_name, {}).get("statements", 0)
        print(f"{dir_name:<25} {baseline_pct:>7.1f}% {curr_pct:>7.1f}% {'N/A':>8}  {YELLOW}SKIP{RESET}")
        continue

    baseline_pct = fe_baseline.get(dir_name, {}).get("statements")
    curr_data = current.get(dir_name)
    curr_pct = curr_data["statements"] if curr_data else None

    # Skip dirs with no current data
    if curr_pct is None and baseline_pct is not None:
        print(f"{dir_name:<25} {baseline_pct:>7.1f}% {'N/A':>8} {'N/A':>8}  {YELLOW}REMOVED{RESET}")
        continue

    # New dir not in baseline
    if baseline_pct is None:
        baseline_pct = 0.0
        fe_baseline[dir_name] = {"statements": 0.0, "branches": 0.0, "functions": 0.0, "lines": 0.0}

    # Floor = min(baseline, 80)
    floor = min(baseline_pct, MIN_FLOOR)
    passed = curr_pct >= floor

    if passed:
        status = f"{GREEN}PASS{RESET}"
    else:
        status = f"{RED}FAIL{RESET}"
        all_pass = False

    # Track improvements
    if curr_pct > baseline_pct + 0.1:
        updated_dirs[dir_name] = curr_data

    print(f"{dir_name:<25} {baseline_pct:>7.1f}% {curr_pct:>7.1f}% {floor:>7.1f}%  {status}")

print()

if all_pass:
    print(f"{GREEN}{BOLD}✓ Frontend coverage gate PASSED{RESET}")

    if updated_dirs and update_baseline:
        print(f"\n{YELLOW}Updating baseline with improved coverage...{RESET}")
        for dir_name, data in updated_dirs.items():
            fe_baseline[dir_name] = data
        baseline_data["frontend"] = fe_baseline
        with open(baseline_file, "w") as f:
            json.dump(baseline_data, f, indent=2)
            f.write("\n")
        print(f"Baseline updated: {list(updated_dirs.keys())}")

    sys.exit(0)
else:
    print(f"{RED}{BOLD}✗ Frontend coverage gate FAILED{RESET}")
    print(f"\n{YELLOW}Tips:{RESET}")
    print("  - Add tests to improve coverage for failing directories")
    print("  - If baseline needs updating: ./scripts/check-frontend-coverage.sh --update")
    print("  - To skip test run: ./scripts/check-frontend-coverage.sh --skip-test")
    sys.exit(1)
PYTHON_SCRIPT
