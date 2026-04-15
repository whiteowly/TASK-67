#!/usr/bin/env python3
"""
check_api_coverage_drift.py — verifies that three sources agree on the
backend API endpoint set:

  1. internal/router/router.go            (canonical)
  2. docs/api-endpoints-inventory.md
  3. docs/api-coverage-after.md

Exits 0 if all three describe the same set of (METHOD, PATH) pairs.
Exits 1 if any source diverges (drift).
Exits 2 on usage / IO error.

Invoked by scripts/check_api_coverage_drift.sh.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path
from typing import Set, Tuple

Endpoint = Tuple[str, str]  # (METHOD, PATH)
METHODS = ("GET", "POST", "PATCH", "PUT", "DELETE")


# ---------------------------------------------------------------------------
# Source 1: router.go
# ---------------------------------------------------------------------------

# Matches:  varname := groupExpr.Group("/path", ...)
# Captures the variable name and the literal path.
_GROUP_RE = re.compile(
    r'(?P<var>[A-Za-z_][A-Za-z_0-9]*)\s*:=\s*'
    r'(?P<parent>[A-Za-z_][A-Za-z_0-9]*)\.Group\("(?P<path>[^"]*)"'
)

# Matches:  receiver.METHOD("path", ...)
_ROUTE_RE = re.compile(
    r'(?P<recv>[A-Za-z_][A-Za-z_0-9]*)\.'
    r'(?P<method>GET|POST|PATCH|PUT|DELETE)\("(?P<path>[^"]*)"'
)


def _parse_router(path: Path) -> Set[Endpoint]:
    """Statically extract (METHOD, PATH) pairs from router.go."""
    src = path.read_text()

    # Group prefix lookup: var name -> absolute path prefix.
    # Seed with "r" -> "" so r.GET("/health") yields "/health".
    prefixes: dict[str, str] = {"r": ""}

    # Walk groups in declaration order so children inherit parents.
    for m in _GROUP_RE.finditer(src):
        parent = m.group("parent")
        var = m.group("var")
        sub = m.group("path")
        parent_prefix = prefixes.get(parent, "")
        prefixes[var] = parent_prefix + sub

    endpoints: Set[Endpoint] = set()
    for m in _ROUTE_RE.finditer(src):
        recv = m.group("recv")
        method = m.group("method")
        sub = m.group("path")
        # Skip false positives where the receiver isn't a known group.
        if recv not in prefixes:
            # Not a router group call — could be e.g. response.OK on an
            # unrelated http verb constant; ignore safely.
            continue
        full = prefixes[recv] + sub
        # Scope: this inventory tracks the JSON API surface only.
        # Web (Templ) page routes are out of scope and are validated by
        # the Playwright suite, not by the API coverage docs.
        if full == "/health" or full.startswith("/api/v1/"):
            endpoints.add((method, full))

    return endpoints


# ---------------------------------------------------------------------------
# Source 2: docs/api-endpoints-inventory.md
# ---------------------------------------------------------------------------

# Matches markdown backticks containing METHOD PATH, e.g.: `GET /api/v1/posts`
_INV_RE = re.compile(r"`(GET|POST|PATCH|PUT|DELETE) (/[^`]*)`")


def _parse_inventory(path: Path) -> Set[Endpoint]:
    text = path.read_text()
    return {(m.group(1), m.group(2)) for m in _INV_RE.finditer(text)}


# ---------------------------------------------------------------------------
# Source 3: docs/api-coverage-after.md
# ---------------------------------------------------------------------------

# Matches table rows like:
#   | 1 | GET | /health | `EXT::TestExternal_Health`; `MX` |
_AFTER_RE = re.compile(
    r"^\|\s*\d+\s*\|\s*(GET|POST|PATCH|PUT|DELETE)\s*\|\s*(\S+)\s*\|"
)


def _parse_after(path: Path) -> Set[Endpoint]:
    endpoints: Set[Endpoint] = set()
    for line in path.read_text().splitlines():
        m = _AFTER_RE.match(line)
        if m:
            endpoints.add((m.group(1), m.group(2)))
    return endpoints


# ---------------------------------------------------------------------------
# Comparison
# ---------------------------------------------------------------------------

def _fmt(eps: Set[Endpoint]) -> str:
    return "\n".join(f"    - {m} {p}" for m, p in sorted(eps))


def _compare(a_label: str, a: Set[Endpoint], b_label: str, b: Set[Endpoint]) -> bool:
    """Return True if drift detected, False if matched."""
    missing_in_b = a - b
    extra_in_b = b - a
    if not missing_in_b and not extra_in_b:
        print(f"OK: {a_label} == {b_label}")
        return False

    print(f"DRIFT: {a_label} vs {b_label}")
    if missing_in_b:
        print(f"  endpoints in {a_label} but MISSING from {b_label}:")
        print(_fmt(missing_in_b))
    if extra_in_b:
        print(f"  endpoints in {b_label} but NOT in {a_label} (stale entries):")
        print(_fmt(extra_in_b))
    print()
    return True


def main(argv: list[str]) -> int:
    if len(argv) != 2:
        print("usage: check_api_coverage_drift.py <repo-root>", file=sys.stderr)
        return 2

    root = Path(argv[1]).resolve()
    router_path = root / "internal" / "router" / "router.go"
    inv_path = root / "docs" / "api-endpoints-inventory.md"
    after_path = root / "docs" / "api-coverage-after.md"

    for p in (router_path, inv_path, after_path):
        if not p.is_file():
            print(f"ERROR: required file missing: {p}", file=sys.stderr)
            return 2

    router_eps = _parse_router(router_path)
    inv_eps = _parse_inventory(inv_path)
    after_eps = _parse_after(after_path)

    print("=== API Coverage Drift Check ===")
    print(f"  router.go              : {len(router_eps)} endpoints")
    print(f"  inventory doc          : {len(inv_eps)} endpoints")
    print(f"  after-coverage mapping : {len(after_eps)} endpoints")
    print()

    drift = False
    drift |= _compare("router.go", router_eps, "inventory doc", inv_eps)
    drift |= _compare("router.go", router_eps, "after-coverage mapping", after_eps)
    drift |= _compare("inventory", inv_eps, "after-coverage mapping", after_eps)

    print()
    if drift:
        print("===========================================")
        print("  COVERAGE DRIFT DETECTED — see above.")
        print("===========================================")
        return 1

    print("===========================================")
    print("  NO COVERAGE DRIFT — all sources agree.")
    print("===========================================")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
