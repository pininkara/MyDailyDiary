#!/usr/bin/env python3
"""
Check which calendar days are missing in a diary import JSON file.

The JSON format is expected to be:
{
  "entries": [
    { "Date": "YYYY-MM-DD", "Title": "...", "Content": "...", "UpdatedAt": "..." },
    ...
  ]
}

Options:
  --input PATH                Input JSON file (default: diary-import.json)
  --start YYYY-MM-DD         Start date for completeness check (inclusive)
  --end YYYY-MM-DD           End date for completeness check (inclusive)
  --json                     Output result as JSON
  --treat-empty-as-missing   Consider entries whose Content is empty/whitespace as "missing"

If --start/--end are not provided, they are inferred from the min/max Date in the file.
If there are no entries and no explicit range is provided, the script exits with a message.
"""
from __future__ import annotations
import argparse
import json
import sys
from datetime import datetime, timedelta
from pathlib import Path
from typing import Any, Dict, Iterable, List, Optional, Set, Tuple


def parse_args(argv: Optional[Iterable[str]] = None) -> argparse.Namespace:
    ap = argparse.ArgumentParser(description="Check missing diary days from JSON")
    ap.add_argument("--input", default="diary-import.json", help="Input JSON path (default: diary-import.json)")
    ap.add_argument("--start", help="Start date YYYY-MM-DD")
    ap.add_argument("--end", help="End date YYYY-MM-DD")
    ap.add_argument("--json", action="store_true", help="Output result as JSON")
    ap.add_argument("--treat-empty-as-missing", action="store_true", help="Treat empty Content as missing")
    return ap.parse_args(argv)


def norm_date_str(s: str) -> Optional[str]:
    try:
        dt = datetime.strptime(s, "%Y-%m-%d")
        return dt.strftime("%Y-%m-%d")
    except Exception:
        return None


def date_range(start: datetime, end: datetime) -> Iterable[datetime]:
    cur = start
    one = timedelta(days=1)
    while cur <= end:
        yield cur
        cur += one


def load_entries(path: Path) -> List[Dict[str, Any]]:
    data = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(data, dict):
        raise ValueError("Top-level JSON must be an object")
    entries = data.get("entries")
    if not isinstance(entries, list):
        raise ValueError("'entries' must be a list")
    return entries


def get_key_ci(d: Dict[str, Any], name: str) -> Any:
    """Case-insensitive, underscore-insensitive key lookup (e.g., Date/date, UpdatedAt/updated_at)."""
    needle = name.replace("_", "").lower()
    for k, v in d.items():
        if k.replace("_", "").lower() == needle:
            return v
    return None


def main(argv: Optional[Iterable[str]] = None) -> int:
    args = parse_args(argv)
    in_path = Path(args.input)
    if not in_path.exists():
        print(f"Input not found: {in_path}", file=sys.stderr)
        return 2

    try:
        entries = load_entries(in_path)
    except Exception as e:
        print(f"Failed to read entries: {e}", file=sys.stderr)
        return 2

    # Collect present dates and optionally filter out empty content
    present: Set[str] = set()
    min_dt: Optional[datetime] = None
    max_dt: Optional[datetime] = None

    for item in entries:
        date_val = get_key_ci(item, "Date")
        if not isinstance(date_val, str):
            continue
        d = norm_date_str(date_val.strip())
        if not d:
            continue

        content_val = get_key_ci(item, "Content")
        if args.treat_empty_as_missing:
            if not isinstance(content_val, str) or content_val.strip() == "":
                # skip counting this as present
                continue

        present.add(d)
        dt = datetime.strptime(d, "%Y-%m-%d")
        if min_dt is None or dt < min_dt:
            min_dt = dt
        if max_dt is None or dt > max_dt:
            max_dt = dt

    # Determine start/end
    start_str = args.start
    end_str = args.end

    if start_str:
        start_norm = norm_date_str(start_str)
        if not start_norm:
            print(f"Invalid --start: {start_str}", file=sys.stderr)
            return 2
        start_dt = datetime.strptime(start_norm, "%Y-%m-%d")
    else:
        if min_dt is None:
            print("No entries found and --start not specified. Provide --start/--end.", file=sys.stderr)
            return 2
        start_dt = min_dt

    if end_str:
        end_norm = norm_date_str(end_str)
        if not end_norm:
            print(f"Invalid --end: {end_str}", file=sys.stderr)
            return 2
        end_dt = datetime.strptime(end_norm, "%Y-%m-%d")
    else:
        if max_dt is None:
            print("No entries found and --end not specified. Provide --start/--end.", file=sys.stderr)
            return 2
        end_dt = max_dt

    if end_dt < start_dt:
        print("--end must be >= --start", file=sys.stderr)
        return 2

    expected = [d.strftime("%Y-%m-%d") for d in date_range(start_dt, end_dt)]
    missing = [d for d in expected if d not in present]

    if args.json:
        out = {
            "start": start_dt.strftime("%Y-%m-%d"),
            "end": end_dt.strftime("%Y-%m-%d"),
            "present": len(present),
            "expected": len(expected),
            "missing_count": len(missing),
            "missing": missing,
        }
        sys.stdout.write(json.dumps(out, ensure_ascii=False, indent=2) + "\n")
    else:
        print(f"Range: {start_dt.strftime('%Y-%m-%d')} .. {end_dt.strftime('%Y-%m-%d')}  (expected {len(expected)} days)")
        print(f"Present days: {len(present)}  Missing: {len(missing)}")
        if missing:
            print("Missing dates:")
            for d in missing:
                print("  ", d)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
