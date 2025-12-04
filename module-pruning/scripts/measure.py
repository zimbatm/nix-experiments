#!/usr/bin/env python3
"""Compare evaluation performance before and after pruning."""

import argparse
import json
import os
import subprocess
import sys
import tempfile
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Optional

ROOT = Path(__file__).resolve().parent.parent
EVAL_NIX = ROOT / "eval.nix"
STORE_DIR = ROOT / "tmp-store" / "store"
STORE_URI = f"local?root={STORE_DIR}"


def state_file_for(mode: str) -> Path:
    return ROOT / f"pruning-state-{mode}.json"


def ensure_store() -> None:
    STORE_DIR.parent.mkdir(parents=True, exist_ok=True)
    if not STORE_DIR.exists():
        STORE_DIR.mkdir(parents=True)
        subprocess.run(
            ["nix-store", "--store", STORE_URI, "--init"],
            check=True,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )


def nix_instantiate(profile: str, drop_expr: str, attribute: str, *, json_mode: bool = True) -> str:
    cmd = [
        "nix-instantiate",
        str(EVAL_NIX),
        "--arg",
        "dropModules",
        drop_expr,
        "--argstr",
        "profile",
        profile,
        "--store",
        STORE_URI,
        "-A",
        attribute,
        "--eval",
        "--strict",
    ]
    if json_mode:
        cmd.append("--json")
    completed = subprocess.run(
        cmd,
        check=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    return completed.stdout.strip()


@dataclass
class EvaluationResult:
    label: str
    duration: float
    module_count: int
    kept_count: int
    gc_summary: Optional[str]


def parse_gc_summary(stats_path: Path) -> Optional[str]:
    if not stats_path.exists() or stats_path.stat().st_size == 0:
        return None
    try:
        data = json.loads(stats_path.read_text())
    except Exception:
        return None
    gc = data.get("gc")
    if not isinstance(gc, dict):
        return None

    def fmt_bytes(value: int) -> str:
        return f"{value / (1024 * 1024):.1f} MiB"

    parts = []
    heap_size = gc.get("heapSize")
    if isinstance(heap_size, int):
        parts.append(f"heap {fmt_bytes(heap_size)}")
    total_bytes = gc.get("totalBytes")
    if isinstance(total_bytes, int):
        parts.append(f"total {fmt_bytes(total_bytes)}")
    cycles = gc.get("cycles")
    if isinstance(cycles, int):
        parts.append(f"cycles {cycles}")

    return "gc stats: " + ", ".join(parts) if parts else None


def measure(profile: str, drop_expr: str, label: str) -> EvaluationResult:
    cmd = [
        "nix-instantiate",
        str(EVAL_NIX),
        "--arg",
        "dropModules",
        drop_expr,
        "--argstr",
        "profile",
        profile,
        "--store",
        STORE_URI,
        "-A",
        "config.system.build.toplevel.drvPath",
        "--eval",
        "--strict",
    ]

    # Warm-up run
    subprocess.run(cmd, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, check=True)

    with tempfile.TemporaryDirectory() as tmp:
        stats_file = Path(tmp) / "stats.json"
        env = os.environ.copy()
        env["NIX_SHOW_STATS"] = "1"
        env["NIX_SHOW_STATS_PATH"] = str(stats_file)

        start = time.perf_counter()
        subprocess.run(cmd, stdout=subprocess.DEVNULL, stderr=subprocess.PIPE, check=True, text=True, env=env)
        duration = time.perf_counter() - start

        gc_summary = parse_gc_summary(stats_file)

    module_count = int(json.loads(nix_instantiate(profile, drop_expr, "moduleCount")))
    kept_count = int(json.loads(nix_instantiate(profile, drop_expr, "prunedModuleCount")))

    return EvaluationResult(
        label=label,
        duration=duration,
        module_count=module_count,
        kept_count=kept_count,
        gc_summary=gc_summary,
    )


def load_state(mode: str) -> tuple[list[str], str]:
    state_path = state_file_for(mode)
    if not state_path.exists():
        print(f"No state file found for mode '{mode}': {state_path}", file=sys.stderr)
        sys.exit(1)
    try:
        data = json.loads(state_path.read_text())
    except Exception as exc:
        print(f"Failed to parse state file {state_path}: {exc}", file=sys.stderr)
        sys.exit(1)
    dropped = data.get("dropped")
    if not isinstance(dropped, list) or not all(isinstance(item, str) for item in dropped):
        print(f"State file {state_path} does not contain a valid 'dropped' list", file=sys.stderr)
        sys.exit(1)
    path_expr = json.dumps(str(state_path))
    drop_expr = f"(let state = builtins.fromJSON (builtins.readFile {path_expr}); in state.dropped)"
    return dropped, drop_expr


def format_duration(seconds: float) -> str:
    if seconds >= 60:
        mins = int(seconds // 60)
        secs = seconds % 60
        return f"{mins}m{secs:04.1f}s"
    return f"{seconds:.2f}s"


def main() -> None:
    parser = argparse.ArgumentParser(
        description="Compare evaluation timings before and after applying pruned modules",
    )
    parser.add_argument("--profile", default="k8s-master", help="Profile name passed to eval.nix")
    parser.add_argument(
        "--mode",
        choices=["eval", "vm"],
        default="eval",
        help="Which pruning mode to read drops from (default: eval)",
    )
    args = parser.parse_args()

    if args.mode == "vm":
        print("warning: VM mode state may be incomplete without full test confirmations", file=sys.stderr)

    ensure_store()

    dropped_modules, drop_expr = load_state(args.mode)
    drop_count = len(dropped_modules)
    if drop_count == 0:
        print("State file contains no dropped modules; run the pruner first.", file=sys.stderr)
        sys.exit(1)

    print(f"Loaded {drop_count} dropped modules from pruning-state-{args.mode}.json")

    baseline = measure(args.profile, "[]", "Baseline (no drops)")
    pruned = measure(args.profile, drop_expr, "Pruned configuration")

    for result in (baseline, pruned):
        print(f"\n{result.label}:")
        print(f"  duration: {format_duration(result.duration)}")
        print(f"  modules kept: {result.kept_count}/{result.module_count}")
        if result.gc_summary:
            print(f"  {result.gc_summary}")

    delta = baseline.duration - pruned.duration
    speedup = baseline.duration / pruned.duration if pruned.duration else None

    print("\nComparison:")
    print(f"  time saved: {format_duration(delta)}")
    if speedup:
        print(f"  speed-up: {speedup:.2f}x")
    print(f"  modules dropped: {drop_count}")


if __name__ == "__main__":
    try:
        main()
    except subprocess.CalledProcessError as exc:
        print(exc.stderr or "evaluation command failed", file=sys.stderr)
        sys.exit(exc.returncode)
