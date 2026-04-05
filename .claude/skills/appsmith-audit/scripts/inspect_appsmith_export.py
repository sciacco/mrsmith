#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
if str(ROOT) not in sys.path:
    sys.path.insert(0, str(ROOT))

from appsmith_audit import analyze_source, render_markdown, write_markdown_artifacts


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Inspect an Appsmith export zip or extracted repository.")
    parser.add_argument("source", help="Path to an Appsmith export zip or extracted repository")
    parser.add_argument("--output-json", help="Write normalized inventory JSON to this file")
    parser.add_argument("--output-md", help="Write the application inventory Markdown to this file")
    parser.add_argument("--artifacts-dir", help="Write structured audit markdown artifacts to this directory")
    parser.add_argument("--pretty", action="store_true", help="Pretty-print JSON to stdout")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    source = Path(args.source).expanduser().resolve()
    try:
        inventory = analyze_source(source)
    except Exception as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        return 1

    payload = inventory.to_dict()
    if args.output_json:
        Path(args.output_json).write_text(json.dumps(payload, indent=2, ensure_ascii=False), encoding="utf-8")
    if args.output_md:
        Path(args.output_md).write_text(render_markdown(inventory), encoding="utf-8")
    if args.artifacts_dir:
        write_markdown_artifacts(inventory, Path(args.artifacts_dir))
    if not args.output_json and not args.output_md and not args.artifacts_dir:
        print(json.dumps(payload, indent=2 if args.pretty else None, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
