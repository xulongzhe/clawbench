#!/usr/bin/env bash
# E2E Coverage Collection and Reporting (Phase 1: report only, no gate)
#
# Usage:
#   ./scripts/check-e2e-coverage.sh              # Run nyc report and display summary
#   ./scripts/check-e2e-coverage.sh --skip-report # Skip nyc report (just display if exists)
#
# Prerequisites:
#   - E2E tests have been run (creates .nyc_output/*.json files)
#   - npx is available (npm installed)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT_DIR"

COVERAGE_DIR=".nyc_output"
REPORT_DIR="coverage/e2e"

echo "==> E2E Coverage Report (Phase 1: report only)"

if [ -z "$(ls -A "$COVERAGE_DIR" 2>/dev/null)" ]; then
  echo "WARNING: No E2E coverage data found in $COVERAGE_DIR/"
  echo "  Run E2E tests first: npm run test:e2e"
  exit 0
fi

if [ "${1:-}" != "--skip-report" ]; then
  echo "==> Generating E2E coverage report from $COVERAGE_DIR/..."
  npx nyc report \
    --reporter=text \
    --reporter=json-summary \
    --report-dir="$REPORT_DIR" \
    --temp-dir="$COVERAGE_DIR" \
    2>/dev/null || true
fi

COVERAGE_JSON="$ROOT_DIR/$REPORT_DIR/coverage-summary.json"

if [ -f "$COVERAGE_JSON" ]; then
  echo ""
  echo "E2E Coverage Summary:"
  python3 - "$COVERAGE_JSON" << 'PYTHON'
import json, sys
try:
    with open(sys.argv[1]) as f:
        data = json.load(f)
    total = data.get("total", {})
    for metric in ["lines", "statements", "functions", "branches"]:
        m = total.get(metric, {})
        pct = (m.get("covered", 0) / m["total"] * 100) if m.get("total", 0) > 0 else 0
        print(f"  {metric:>12}: {m.get('covered', 0):>6}/{m.get('total', 0):<6} ({pct:.1f}%)")
except Exception as e:
    print(f"  Error reading coverage: {e}")
PYTHON
else
  echo "WARNING: No E2E coverage summary generated at $REPORT_DIR/coverage-summary.json"
  echo "  Run 'npx nyc report' manually to generate the report"
fi
