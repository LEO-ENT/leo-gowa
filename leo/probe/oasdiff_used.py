"""Filter `oasdiff breaking --format json` on the endpoints orch uses (GOWA PLAN F6-T2, B12).

Input: the oasdiff JSON (a list of changes with `path`, `operation`, `level`, `id`, `text`)
and `leo/orch-endpoints.txt` (`METHOD /path` per line, generated in leo-orchestrator by
`scripts/gowa_endpoints.py`, path params as `{x}`). A change hits orch when its path,
with every `{param}` normalised to `{x}`, is in that list (the method is not compared:
a breaking change on any operation of a used path is reported, conservatively).

Exit 1 when a change of level ERR (3) hits a used path; WARN (2) changes on used paths are
listed but do not fail. The report (Markdown) goes to stdout.

Usage: python3 oasdiff_used.py BREAKING_JSON ORCH_ENDPOINTS_TXT
"""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path

ERR = 3
WARN = 2
_PARAM = re.compile(r"\{[^{}]*\}")


def norm(path: str) -> str:
    path = _PARAM.sub("{x}", path.split("?", 1)[0])
    return path.rstrip("/") if len(path) > 1 else path


def used_paths(text: str) -> set[str]:
    paths: set[str] = set()
    for line in text.splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        paths.add(norm(line.split()[-1]))
    return paths


def main(argv: list[str]) -> int:
    if len(argv) != 3:
        print(__doc__, file=sys.stderr)
        return 2
    changes = json.loads(Path(argv[1]).read_text(encoding="utf-8") or "[]") or []
    used = used_paths(Path(argv[2]).read_text(encoding="utf-8"))
    if not used:
        print("orch-endpoints list is empty: refusing to pass an unfiltered verdict", file=sys.stderr)
        return 2
    hits = [c for c in changes if norm(str(c.get("path", ""))) in used]
    errors = [c for c in hits if int(c.get("level", 0)) >= ERR]
    warns = [c for c in hits if int(c.get("level", 0)) == WARN]

    print(f"oasdiff: {len(changes)} breaking change(s) overall, {len(hits)} on the "
          f"{len(used)} paths orch uses ({len(errors)} ERR, {len(warns)} WARN).")
    for label, group in (("ERR", errors), ("WARN", warns)):
        if group:
            print(f"\n### {label} on endpoints used by orch\n")
            for c in sorted(group, key=lambda c: (c.get("path", ""), c.get("operation", ""))):
                print(f"- `{c.get('operation', '?')} {c.get('path', '?')}` "
                      f"[{c.get('id', '?')}]: {c.get('text', '')}")
    return 1 if errors else 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
