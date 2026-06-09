#!/usr/bin/env python3
"""
Parse coverage.out and print a per-file coverage table styled after
Vitest's text reporter:

  File                               | % Stmts | % Funcs | % Lines | Uncovered Line #s
  -----------------------------------|---------|---------|---------|------------------
  internal/application/service.go    |   95.24 |  100.00 |   95.24 | 88,102
  internal/domain/user.go            |  100.00 |  100.00 |  100.00 |
  ...
  All files                          |   96.60 |   97.22 |   96.60 |

Usage:
    python3 scripts/coverage-report.py coverage.out [func-coverage.txt]

  Generate func-coverage.txt with:
    go tool cover -func=coverage.out > func-coverage.txt
"""

import sys
import re
from collections import defaultdict

# ── Parse coverage.out ──────────────────────────────────────────────────────

def parse_coverage(path):
    """
    Returns dict[rel_path] = {total, covered, uncovered_lines}.
    coverage.out format:
      github.com/user/repo/internal/foo/bar.go:12.30,15.2 3 1
                                                            ^stmts ^count
    """
    data = defaultdict(lambda: {"total": 0, "covered": 0, "uncovered": []})
    pat  = re.compile(r"^(.+):(\d+)\.\d+,(\d+)\.\d+\s+(\d+)\s+(\d+)$")

    with open(path) as f:
        for line in f:
            line = line.strip()
            if line.startswith("mode:") or not line:
                continue
            m = pat.match(line)
            if not m:
                continue
            full, start, _end, stmts, count = m.groups()
            stmts, count = int(stmts), int(count)
            rel = _rel(full)
            data[rel]["total"]   += stmts
            data[rel]["covered"] += stmts if count > 0 else 0
            if count == 0 and stmts > 0:
                data[rel]["uncovered"].append(int(start))

    return data

# ── Parse go tool cover -func output ────────────────────────────────────────

def parse_func(path):
    """
    Returns dict[rel_path] = {total_funcs, covered_funcs}.
    go tool cover -func format:
      github.com/.../service.go:42:   CreateUser   100.0%
    """
    data = defaultdict(lambda: {"total": 0, "covered": 0})
    pat  = re.compile(r"^(.+):\d+:\s+\S+\s+([\d.]+)%$")

    try:
        with open(path) as f:
            for line in f:
                line = line.strip()
                if "total:" in line or not line:
                    continue
                m = pat.match(line)
                if not m:
                    continue
                full, pct = m.groups()
                rel = _rel(full)
                data[rel]["total"]   += 1
                data[rel]["covered"] += 1 if float(pct) > 0 else 0
    except FileNotFoundError:
        pass  # func file is optional

    return data

# ── Helpers ──────────────────────────────────────────────────────────────────

def _rel(full_path):
    """Strip module prefix, keep from first known top-level dir."""
    parts = full_path.replace("\\", "/").split("/")
    anchors = {"internal", "pkg", "cmd", "test", "api", "proto"}
    for i, p in enumerate(parts):
        if p in anchors:
            return "/".join(parts[i:])
    return parts[-1]  # fallback: just filename

def _pct(covered, total):
    if total == 0:
        return 100.0
    return covered / total * 100

def _uncov_str(lines, max_len=35):
    if not lines:
        return ""
    lines = sorted(set(lines))
    groups, s, e = [], lines[0], lines[0]
    for l in lines[1:]:
        if l <= e + 2:
            e = l
        else:
            groups.append(str(s) if s == e else f"{s}-{e}")
            s = e = l
    groups.append(str(s) if s == e else f"{s}-{e}")
    out = ",".join(groups)
    return out[:max_len - 3] + "..." if len(out) > max_len else out

# ── Main ─────────────────────────────────────────────────────────────────────

def main():
    cov_file  = sys.argv[1] if len(sys.argv) > 1 else "coverage.out"
    func_file = sys.argv[2] if len(sys.argv) > 2 else "func-coverage.txt"

    cov  = parse_coverage(cov_file)
    func = parse_func(func_file)

    if not cov:
        print("No coverage data found in", cov_file)
        sys.exit(1)

    # Column widths
    W = max(max(len(f) for f in cov), len("File"), 30)
    W = min(W, 50)

    HDR = f"{'File':<{W}} | % Stmts | % Funcs | % Lines | Uncovered Line #s"
    SEP = f"{'-'*W}-+---------+---------+---------+------------------"

    print()
    print(HDR)
    print(SEP)

    tot_stmts = tot_cov = tot_funcs = tot_funcs_cov = 0

    for path in sorted(cov):
        d = cov[path]
        f = func.get(path, {"total": 0, "covered": 0})

        s_pct  = _pct(d["covered"],   d["total"])
        f_pct  = _pct(f["covered"],   f["total"]) if f["total"] else s_pct
        uncov  = _uncov_str(d["uncovered"])

        tot_stmts     += d["total"]
        tot_cov       += d["covered"]
        tot_funcs     += f["total"]
        tot_funcs_cov += f["covered"]

        name = path if len(path) <= W else "..." + path[-(W - 3):]
        print(f"{name:<{W}} | {s_pct:>7.2f} | {f_pct:>7.2f} | {s_pct:>7.2f} | {uncov}")

    print(SEP)
    all_s = _pct(tot_cov, tot_stmts)
    all_f = _pct(tot_funcs_cov, tot_funcs) if tot_funcs else all_s
    print(f"{'All files':<{W}} | {all_s:>7.2f} | {all_f:>7.2f} | {all_s:>7.2f} |")
    print()

if __name__ == "__main__":
    main()
