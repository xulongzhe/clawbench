#!/usr/bin/env python3
"""Generate provider_models.json from models.dev API.

Fetches https://models.dev/api.json, extracts tool_call-capable models for
ClawBench providers, classifies cost tiers, and writes config/provider_models.json.

Usage:
    python3 scripts/generate-provider-models.py [--output PATH]

The output file is committed to git. Run this script periodically or before
releases to update model lists. The build.sh script runs this automatically.
"""

import json
import sys
import urllib.request
from datetime import datetime, timezone
from pathlib import Path

API_URL = "https://models.dev/api.json"

# Default output path (relative to project root)
# Placed inside internal/model/ so go:embed can access it
DEFAULT_OUTPUT = "internal/model/provider_models.json"

# ---------------------------------------------------------------------------
# Provider mapping: ClawBench provider ID → (models.dev provider ID, filter)
#
# filter: callable(model_dict) -> bool, or None for no extra filtering.
# Only models with tool_call=true are included by default.
# ---------------------------------------------------------------------------

def _vercel_filter(m):
    """Vercel AI Gateway proxies many providers; we only include Anthropic models
    since we use it via Anthropic-format API."""
    return m.get("id", "").startswith("anthropic/")

PROVIDER_MAPPING = {
    # ClawBench ID     → (models.dev ID,       extra filter)
    "openai":              ("openai",              None),
    "anthropic":           ("anthropic",           None),
    "google":              ("google",              None),
    "deepseek":            ("deepseek",            None),
    "groq":                ("groq",                None),
    "openrouter":          ("openrouter",          None),
    "cerebras":            ("cerebras",            None),
    "xai":                 ("xai",                 None),
    "mistral":             ("mistral",             None),
    "fireworks":           ("fireworks-ai",        None),
    "minimax":             ("minimax",             None),
    "minimax-cn":          ("minimax-cn",          None),
    "kimi-coding":         ("kimi-for-coding",     None),
    "moonshotai":          ("moonshotai",          None),
    "moonshotai-cn":       ("moonshotai-cn",       None),
    "xiaomi":              ("xiaomi",              None),
    "xiaomi-token-plan-cn": ("xiaomi-token-plan-cn", None),
    "xiaomi-token-plan-ams": ("xiaomi-token-plan-ams", None),
    "xiaomi-token-plan-sgp": ("xiaomi-token-plan-sgp", None),
    "zai":                 ("zai-coding-plan",     None),
    "huggingface":         ("huggingface",         None),
    "opencode":            ("opencode",            None),
    "vercel-ai-gateway":   ("vercel",              _vercel_filter),
}


def classify_cost_tier(output_cost_per_m):
    """Classify cost tier based on output cost per 1M tokens (USD).

    <= $1  → cheap
    <= $10 → moderate
    >  $10 → expensive
    """
    if output_cost_per_m is None or output_cost_per_m == 0:
        return "cheap"  # free or unknown → default to cheap
    if output_cost_per_m <= 1:
        return "cheap"
    if output_cost_per_m <= 10:
        return "moderate"
    return "expensive"


def extract_model(m):
    """Extract ClawBench KnownModel fields from a models.dev model entry."""
    limits = m.get("limit", {})
    cost = m.get("cost", {})

    context_length = limits.get("context", 0)
    max_output_tokens = limits.get("output", 0)
    output_cost = cost.get("output", 0)

    return {
        "id": m["id"],
        "name": m.get("name", m["id"]),
        "context_length": context_length,
        "max_output_tokens": max_output_tokens,
        "supports_thinking": bool(m.get("reasoning", False)),
        "cost_tier": classify_cost_tier(output_cost),
    }


def fetch_api():
    """Fetch models.dev API data."""
    print(f"Fetching {API_URL} ...")
    req = urllib.request.Request(API_URL, headers={"User-Agent": "clawbench-generate-models/1.0"})
    with urllib.request.urlopen(req, timeout=30) as resp:
        return json.loads(resp.read().decode("utf-8"))


def generate(data):
    """Generate the provider_models structure from API data."""
    providers = {}

    for clawbench_id, (modelsdev_id, extra_filter) in sorted(PROVIDER_MAPPING.items()):
        provider_data = data.get(modelsdev_id)
        if not provider_data:
            print(f"  WARNING: models.dev provider '{modelsdev_id}' not found (for ClawBench '{clawbench_id}')")
            continue

        models_raw = provider_data.get("models", {})
        models = []

        for model_id, model_data in sorted(models_raw.items()):
            # Only include tool_call-capable models
            if not model_data.get("tool_call"):
                continue

            # Apply extra provider-specific filter
            if extra_filter and not extra_filter(model_data):
                continue

            models.append(extract_model(model_data))

        if models:
            providers[clawbench_id] = {"models": models}
            print(f"  {clawbench_id}: {len(models)} models (from {modelsdev_id})")
        else:
            print(f"  {clawbench_id}: 0 models (from {modelsdev_id}) — SKIPPED")

    return {
        "_generated_at": datetime.now(timezone.utc).isoformat(),
        "_source": API_URL,
        "providers": providers,
    }


def main():
    output_path = DEFAULT_OUTPUT
    if len(sys.argv) >= 3 and sys.argv[1] == "--output":
        output_path = sys.argv[2]

    # Resolve path relative to project root (where this script's parent lives)
    script_dir = Path(__file__).resolve().parent
    project_root = script_dir.parent
    output_file = project_root / output_path

    try:
        data = fetch_api()
    except Exception as e:
        print(f"ERROR: Failed to fetch models.dev API: {e}", file=sys.stderr)
        print("Using existing provider_models.json if available.", file=sys.stderr)
        sys.exit(1)

    result = generate(data)

    # Write output
    output_file.parent.mkdir(parents=True, exist_ok=True)
    with open(output_file, "w", encoding="utf-8") as f:
        json.dump(result, f, indent=2, ensure_ascii=False)
        f.write("\n")

    total_models = sum(len(p["models"]) for p in result["providers"].values())
    total_providers = len(result["providers"])
    print(f"\nWrote {total_models} models across {total_providers} providers to {output_file}")


if __name__ == "__main__":
    main()
