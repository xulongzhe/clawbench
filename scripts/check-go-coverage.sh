#!/usr/bin/env bash
# check-go-coverage.sh — Run Go tests with coverage and enforce coverage gate.
# Gate rule: per-package coverage >= min(baseline, 80%) - tolerance.
# Tolerance (default 1.0%) accounts for cross-environment coverage fluctuations.
#
# Usage:
#   ./scripts/check-go-coverage.sh              # run tests + check
#   ./scripts/check-go-coverage.sh --skip-test   # skip running tests, use existing coverage.out
#   ./scripts/check-go-coverage.sh --update      # update baseline with current coverage (only raises, never lowers)
#
# Exit code: 0 = pass, 1 = fail

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
BASELINE_FILE="$ROOT_DIR/coverage-baseline.json"
COVERAGE_PROFILE="$ROOT_DIR/coverage.out"

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

# Step 1: Run tests with coverage (if not skipped)
if [ "$SKIP_TEST" = false ]; then
  echo "==> Running Go tests with coverage..."
  go test -coverprofile="$COVERAGE_PROFILE" ./... 2>&1
  echo ""
fi

if [ ! -f "$COVERAGE_PROFILE" ]; then
  echo "ERROR: coverage.out not found. Run without --skip-test first."
  exit 1
fi

# Step 2: Use Python to parse coverage and check gate
# We run `go test -cover ./...` separately just to get the per-package percentage text
# (go tool cover -func uses a different algorithm — function-weighted vs statement-weighted)
python3 - "$BASELINE_FILE" "$COVERAGE_PROFILE" "$UPDATE_BASELINE" << 'PYTHON_SCRIPT'
import json, sys, re, subprocess

baseline_file = sys.argv[1]
coverage_profile = sys.argv[2]
update_baseline = sys.argv[3] == "true"
MIN_FLOOR = 80.0
TOLERANCE = 1.0  # Allow ±1% fluctuation across environments

# Load baseline
with open(baseline_file) as f:
    baseline_data = json.load(f)
go_baseline = baseline_data.get("go", {})

# Get per-package coverage from `go test -cover`
# This gives statement-weighted coverage (the same metric Go reports in test output)
# We also parse the cover profile to discover all packages (including 0% ones)
result = subprocess.run(
    ["go", "test", "-cover", "./..."],
    capture_output=True, text=True
)
output = result.stdout + result.stderr

# Parse output:
#   ok   clawbench/internal/ai  3.092s  coverage: 77.0% of statements
#   ?    clawbench/cmd/server   [no test files]
current = {}
for line in output.split("\n"):
    m = re.search(r'^[ok?]{2}\s+(\S+)\s+.*?coverage:\s+([\d.]+)%', line)
    if m:
        pkg = m.group(1)
        if "node_modules" in pkg or "vendor" in pkg:
            continue
        current[pkg] = float(m.group(2))

# Also discover packages in the profile that have 0% coverage
# (they don't appear in `go test -cover` output)
with open(coverage_profile) as f:
    profile_pkgs = set()
    for line in f:
        if line.startswith("mode:"):
            continue
        parts = line.split()
        if parts:
            # File path like: clawbench/internal/ai/agent.go
            file_path = parts[0]
            pkg = "/".join(file_path.split("/")[:-1])
            if pkg.startswith("clawbench/") and "node_modules" not in pkg and "vendor" not in pkg:
                profile_pkgs.add(pkg)

# Add packages from baseline that exist in profile but weren't reported by go test
for pkg in go_baseline:
    if pkg in profile_pkgs and pkg not in current:
        current[pkg] = 0.0

# Report
BOLD = "\033[1m"
RED = "\033[0;31m"
GREEN = "\033[0;32m"
YELLOW = "\033[0;33m"
RESET = "\033[0m"

print(f"{BOLD}╔══════════════════════════════════════════════════════════════════╗{RESET}")
print(f"{BOLD}║                  Go Coverage Gate Check                        ║{RESET}")
print(f"{BOLD}║  Rule: coverage >= min(baseline, {MIN_FLOOR}%) - {TOLERANCE}%              ║{RESET}")
print(f"{BOLD}╚══════════════════════════════════════════════════════════════════╝{RESET}")
print()

all_pass = True
updated_pkgs = {}

# Header
print(f"{BOLD}{'Package':<40} {'Base%':>8} {'Curr%':>8} {'Floor':>8}  {'Status':<8}{RESET}")
print(f"{'─'*40} {'─'*8} {'─'*8} {'─'*8}  {'─'*8}")

# Check all baseline packages + any new ones
all_pkgs = sorted(set(list(go_baseline.keys()) + list(current.keys())))

for pkg in all_pkgs:
    baseline_pct = go_baseline.get(pkg)
    curr_pct = current.get(pkg)

    # Skip packages with no current data (removed from codebase)
    if curr_pct is None and baseline_pct is not None:
        print(f"{pkg:<40} {baseline_pct:>7.1f}% {'N/A':>8} {'N/A':>8}  {YELLOW}REMOVED{RESET}")
        continue

    # New package not in baseline
    if baseline_pct is None:
        baseline_pct = 0.0
        go_baseline[pkg] = 0.0

    # Floor = min(baseline, 80) - tolerance
    floor = max(min(baseline_pct, MIN_FLOOR) - TOLERANCE, 0.0)
    passed = curr_pct >= floor

    if passed:
        status = f"{GREEN}PASS{RESET}"
    else:
        status = f"{RED}FAIL{RESET}"
        all_pass = False

    # Track improvements (only upward — never lower baseline)
    if curr_pct > baseline_pct + 0.1:
        updated_pkgs[pkg] = curr_pct

    print(f"{pkg:<40} {baseline_pct:>7.1f}% {curr_pct:>7.1f}% {floor:>7.1f}%  {status}")

print()

if all_pass:
    print(f"{GREEN}{BOLD}✓ Go coverage gate PASSED{RESET}")

    if updated_pkgs and update_baseline:
        print(f"\n{YELLOW}Updating baseline with improved coverage...{RESET}")
        for pkg, pct in updated_pkgs.items():
            go_baseline[pkg] = pct
        baseline_data["go"] = go_baseline
        with open(baseline_file, "w") as f:
            json.dump(baseline_data, f, indent=2)
            f.write("\n")
        print(f"Baseline updated: {list(updated_pkgs.keys())}")

    sys.exit(0)
else:
    print(f"{RED}{BOLD}✗ Go coverage gate FAILED{RESET}")
    print(f"\n{YELLOW}Tips:{RESET}")
    print("  - Add tests to improve coverage for failing packages")
    print("  - If baseline needs updating: ./scripts/check-go-coverage.sh --update")
    print("  - To skip test run (use existing coverage.out): ./scripts/check-go-coverage.sh --skip-test")
    sys.exit(1)
PYTHON_SCRIPT
