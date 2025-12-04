#!/usr/bin/env python3
import argparse
import json
import re
import subprocess
import sys
from pathlib import Path
from typing import Iterable, List, Sequence, Set, Tuple, Optional

ROOT = Path(__file__).resolve().parent.parent
EVAL_NIX = ROOT / "eval.nix"
TEST_NIX = ROOT / "test-vm.nix"
STORE_DIR = ROOT / "tmp-store" / "store"
STORE_URI = f"local?root={STORE_DIR}"
NIXPKGS_PATH = (ROOT.parent.parent.parent / "NixOS" / "nixpkgs").resolve()
MODULE_PREFIX = str(NIXPKGS_PATH / "nixos" / "modules") + "/"


def state_file_for(mode: str) -> Path:
    return ROOT / f"pruning-state-{mode}.json"


def ensure_store() -> None:
    """Ensure the local store path exists and is initialised."""
    STORE_DIR.parent.mkdir(parents=True, exist_ok=True)
    if not STORE_DIR.exists():
        STORE_DIR.mkdir(parents=True)
        subprocess.run(
            [
                "nix-store",
                "--store",
                STORE_URI,
                "--init",
            ],
            check=True,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )


def load_state(state_file: Path) -> dict:
    """Load pruning state from disk if available."""
    if state_file.exists():
        try:
            with open(state_file, "r") as f:
                return json.load(f)
        except Exception as e:  # pragma: no cover - diagnostic only
            print(f"Warning: Could not load state file: {e}", file=sys.stderr)
    return {"dropped": [], "required": [], "untested": [], "dependencies": []}


def save_state(state_file: Path, state: dict) -> None:
    """Persist pruning state to disk."""
    try:
        state = {
            "dropped": state.get("dropped", []),
            "required": state.get("required", []),
            "untested": state.get("untested", []),
            "dependencies": state.get("dependencies", []),
        }
        with open(state_file, "w") as f:
            json.dump(state, f, indent=2, sort_keys=True)
        print(f"State saved to {state_file}")
    except Exception as e:  # pragma: no cover - diagnostic only
        print(f"Warning: Could not save state file: {e}", file=sys.stderr)


def nix_cmd(common_args: Sequence[str]) -> List[str]:
    return [
        "nix-instantiate",
        str(EVAL_NIX),
        "--store",
        STORE_URI,
        *common_args,
    ]


def get_kept_modules(profile: str) -> List[str]:
    cmd = nix_cmd(
        [
            "--arg",
            "dropModules",
            "[]",
            "--argstr",
            "profile",
            profile,
            "-A",
            "keptModules",
            "--eval",
            "--strict",
            "--json",
        ]
    )
    completed = subprocess.run(
        cmd,
        check=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    return json.loads(completed.stdout)


def drop_expr(mods: Iterable[str]) -> str:
    parts = [f'"{m}"' for m in mods]
    if parts:
        return "[ " + " ".join(parts) + " ]"
    return "[]"


def canonicalise_module(path: str) -> str:
    path = path.strip()
    if MODULE_PREFIX in path:
        idx = path.find(MODULE_PREFIX)
        path = path[idx + len(MODULE_PREFIX) :]
    if path.startswith("./"):
        path = path[2:]
    return path


def parse_missing_option(stderr: str) -> Optional[Tuple[str, List[str]]]:
    option_match = re.search(r"The option `([^']+)' does not exist", stderr)
    if not option_match:
        return None
    option = option_match.group(1)
    dependents = []
    block_match = re.search(r"Definition values:(.*?)(?:\n\S|$)", stderr, re.S)
    if block_match:
        block = block_match.group(1)
        for path in re.findall(r"- In [`']([^`']+)[`']", block):
            dependents.append(canonicalise_module(path))
    return option, dependents


def option_declarations_expr(profile: str, option_path: str) -> str:
    eval_path = json.dumps(str(EVAL_NIX))
    profile_str = json.dumps(profile)
    segments = option_path.split(".")

    def attr_segment(seg: str) -> str:
        if re.match(r"^[A-Za-z_][A-Za-z0-9_']*$", seg):
            return f".{seg}"
        return f".{json.dumps(seg)}"

    attr_path = "".join(attr_segment(seg) for seg in segments if seg)
    return (
        f"(let eval = import {eval_path}; cfg = eval {{ dropModules = []; extraModules = []; profile = {profile_str}; }}; "
        f"(cfg.options{attr_path}.declarations or []))"
    )


class EvalOnlyEvaluator:
    def __init__(self, profile: str, extra_args: Sequence[str]) -> None:
        self.profile = profile
        self.extra_args = list(extra_args)
        self.cache: dict[Tuple[str, ...], bool] = {}
        self.eval_count = 0
        self.last_failure: Optional[str] = None

    def _command(self, drops: Sequence[str]) -> List[str]:
        expr = drop_expr(drops)
        return nix_cmd(
            [
                "--arg",
                "dropModules",
                expr,
                "--argstr",
                "profile",
                self.profile,
                "-A",
                "config.system.build.toplevel.drvPath",
                "--eval",
                "--strict",
            ]
            + list(self.extra_args)
        )

    def evaluate(self, current_drops: Set[str]) -> bool:
        key = tuple(sorted(current_drops))
        if key in self.cache:
            print(f"  Using cached result for {len(current_drops)} dropped modules")
            return self.cache[key]

        self.eval_count += 1
        print(f"  Eval #{self.eval_count}: Testing with {len(current_drops)} modules dropped...")

        cmd = self._command(sorted(current_drops))
        proc = subprocess.run(
            cmd,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.PIPE,
            text=True,
        )
        success = proc.returncode == 0
        self.cache[key] = success

        if success:
            self.last_failure = None
            print("    ✓ Evaluation passed")
        else:
            self.last_failure = proc.stderr
            print("    ✗ Evaluation failed")
            if proc.stderr:
                print("      stderr (last 20 lines):")
                lines = proc.stderr.strip().splitlines()
                for line in lines[-20:]:
                    print(f"        {line}")

        return success


class VMTestEvaluator:
    def __init__(self, profile: str, extra_args: Sequence[str]) -> None:
        self.profile = profile
        self.extra_args = list(extra_args)
        self.cache: dict[Tuple[str, ...], bool] = {}
        self.eval_count = 0
        self.last_failure: Optional[str] = None

    def _command(self, drops: Sequence[str]) -> List[str]:
        expr = drop_expr(drops)
        return [
            "nix-build",
            str(TEST_NIX),
            "--store",
            STORE_URI,
            "--arg",
            "dropModules",
            expr,
            "--argstr",
            "profile",
            self.profile,
            "--no-out-link",
        ] + list(self.extra_args)

    def evaluate(self, current_drops: Set[str]) -> bool:
        key = tuple(sorted(current_drops))
        if key in self.cache:
            print(f"  Using cached result for {len(current_drops)} dropped modules")
            return self.cache[key]

        self.eval_count += 1
        print(f"  VM test #{self.eval_count}: Testing with {len(current_drops)} modules dropped...")

        cmd = self._command(sorted(current_drops))
        proc = subprocess.run(
            cmd,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.PIPE,
            text=True,
        )
        success = proc.returncode == 0
        self.cache[key] = success

        if success:
            self.last_failure = None
            print("    ✓ VM test passed")
        else:
            self.last_failure = proc.stderr
            print("    ✗ VM test failed")
            if proc.stderr:
                print("      stderr (last 20 lines):")
                lines = proc.stderr.strip().splitlines()
                for line in lines[-20:]:
                    print(f"        {line}")

        return success


class PersistentPruner:
    def __init__(
        self,
        modules: Sequence[str],
        evaluator,
        state: dict,
        state_file: Path,
        profile: str,
    ) -> None:
        self.modules = list(modules)
        self.evaluator = evaluator
        self.state_file = state_file
        self.profile = profile
        self.extra_args = getattr(self.evaluator, "extra_args", [])

        self.drop_set: Set[str] = set(state.get("dropped", []))
        self.required: Set[str] = set(state.get("required", []))

        raw_untested = state.get("untested")
        if raw_untested:
            self.untested = set(raw_untested)
        elif not self.drop_set and not self.required:
            self.untested = set(modules)
        else:
            self.untested = set()

        self.dependencies: List[dict] = list(state.get("dependencies", []))
        self._dependency_index: Set[Tuple[str, str, Tuple[str, ...]]] = set()
        for entry in self.dependencies:
            option = entry.get("option")
            dependent = entry.get("dependent")
            providers = tuple(sorted(entry.get("providers", [])))
            if option and dependent and providers:
                self._dependency_index.add((option, dependent, providers))

        self._option_provider_cache: dict[str, List[str]] = {}

        self.untested -= self.drop_set
        self.untested -= self.required

        print(
            "Loaded state: "
            f"{len(self.drop_set)} dropped, "
            f"{len(self.required)} required, "
            f"{len(self.untested)} untested"
        )

    def save_state(self) -> None:
        state = {
            "dropped": sorted(self.drop_set),
            "required": sorted(self.required),
            "untested": sorted(self.untested),
            "dependencies": self.dependencies,
        }
        save_state(self.state_file, state)

    def prune_group(self, group: Sequence[str]) -> None:
        filtered = [m for m in group if m in self.untested]
        if not filtered:
            return

        allowed: List[str] = []
        candidate = set(filtered)
        required_updated = False
        for mod in filtered:
            blockers = self.dependency_blockers(mod, candidate)
            if blockers:
                self.required.add(mod)
                self.untested.discard(mod)
                details = ", ".join(f"{dep} (via {opt})" for dep, opt in blockers)
                print(f"⚠  Keeping {mod} due to dependencies: {details}")
                candidate.discard(mod)
                required_updated = True
            else:
                allowed.append(mod)

        if not allowed:
            if required_updated:
                self.save_state()
            return

        candidate = set(allowed)
        if required_updated:
            self.save_state()

        if self.evaluator.evaluate(self.drop_set | candidate):
            self.drop_set.update(candidate)
            self.untested -= candidate
            print(
                f"✓ Dropped {len(candidate)} modules, "
                f"total dropped: {len(self.drop_set)}"
            )
            self.save_state()
            return

        failure = getattr(self.evaluator, "last_failure", None)
        if failure:
            self.record_dependency(failure)

        if len(filtered) == 1:
            mod = filtered[0]
            self.required.add(mod)
            self.untested.discard(mod)
            print(f"✗ Required: {mod}")
            self.save_state()
            return

        mid = len(filtered) // 2
        self.prune_group(filtered[:mid])
        self.prune_group(filtered[mid:])

    def run(self) -> None:
        if not self.untested:
            print("No untested modules remaining!")
            return

        print(f"Starting pruning on {len(self.untested)} untested modules...")
        self.prune_group(sorted(self.untested))

    def lookup_option_providers(self, option: str) -> List[str]:
        if option in self._option_provider_cache:
            return self._option_provider_cache[option]

        expr = option_declarations_expr(self.profile, option)
        cmd = [
            "nix-instantiate",
            "--store",
            STORE_URI,
            "--eval",
            "--strict",
            "--json",
        ]
        cmd.extend(self.extra_args)
        cmd.extend(["--expr", expr])

        try:
            completed = subprocess.run(
                cmd,
                check=True,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                text=True,
            )
            data = json.loads(completed.stdout)
            providers = sorted(
                {canonicalise_module(path) for path in data if isinstance(path, str)}
            )
        except subprocess.CalledProcessError as exc:
            print(
                f"Warning: failed to resolve providers for option {option}: {exc.stderr.strip()}",
                file=sys.stderr,
            )
            providers = []

        self._option_provider_cache[option] = providers
        return providers

    def record_dependency(self, stderr: str) -> None:
        parsed = parse_missing_option(stderr)
        if not parsed:
            return
        option, dependents = parsed
        providers = self.lookup_option_providers(option)
        if not providers:
            return

        dependents = dependents or ["<unknown>"]
        key_providers = tuple(sorted(providers))

        for dependent in dependents:
            entry = {
                "option": option,
                "dependent": dependent,
                "providers": providers,
            }
            key = (option, dependent, key_providers)
            if key in self._dependency_index:
                continue
            self._dependency_index.add(key)
            self.dependencies.append(entry)
            print(
                "    Recorded dependency: "
                f"{dependent} needs option {option} from {', '.join(providers)}"
            )
            self.save_state()

    def dependency_blockers(
        self, module: str, candidate: Set[str]
    ) -> List[Tuple[str, str]]:
        blockers: List[Tuple[str, str]] = []
        for entry in self.dependencies:
            providers = entry.get("providers", [])
            if module not in providers:
                continue
            dependent = entry.get("dependent")
            option = entry.get("option")
            if not dependent or not option:
                continue
            if dependent in candidate:
                continue
            if dependent in self.drop_set:
                continue
            blockers.append((dependent, option))
        return blockers


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Automatically drop NixOS modules using persistent VM tests or eval-only checks",
    )
    parser.add_argument("--profile", default="k8s-master", help="Profile name passed to eval.nix")
    parser.add_argument(
        "--mode",
        choices=["vm", "eval"],
        default="vm",
        help="Evaluation strategy: full VM tests (default) or eval-only",
    )
    parser.add_argument("--reset", action="store_true", help="Reset state and start fresh")
    parser.add_argument(
        "--extra",
        nargs=argparse.REMAINDER,
        help="Extra arguments forwarded to nix-build/nix-instantiate after --",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    extra = args.extra or []
    if extra and extra[0] == "--":
        extra = extra[1:]

    state_file = state_file_for(args.mode)

    if args.reset and state_file.exists():
        state_file.unlink()
        print(f"Removed state file: {state_file}")

    ensure_store()

    modules = get_kept_modules(args.profile)
    print(f"Total modules in baseline: {len(modules)}")

    state = load_state(state_file)

    if args.mode == "vm":
        evaluator = VMTestEvaluator(args.profile, extra)
        baseline_msg = "\nTesting baseline configuration with no modules dropped..."
        baseline_error = "ERROR: Baseline VM test failed; aborting"
        summary_label = "VM tests run"
        verify_label = "VM tests"
        print("Mode: VM tests (nix-build on test-vm.nix)")
    else:
        evaluator = EvalOnlyEvaluator(args.profile, extra)
        baseline_msg = "\nEvaluating baseline configuration with no modules dropped..."
        baseline_error = "ERROR: Baseline evaluation failed; aborting"
        summary_label = "Evaluations run"
        verify_label = "evaluations"
        print("Mode: evaluation only (nix-instantiate -A config.system.build.toplevel.drvPath)")

    print(baseline_msg)
    if not evaluator.evaluate(set()):
        print(baseline_error, file=sys.stderr)
        return 1
    print("Baseline check passed!\n")

    if state.get("dropped"):
        print(
            f"Verifying {len(state['dropped'])} previously dropped modules still work via {verify_label}..."
        )
        if not evaluator.evaluate(set(state["dropped"])):
            print(
                "ERROR: Previously dropped modules no longer work! Resetting state.",
                file=sys.stderr,
            )
            state = {
                "dropped": [],
                "required": [],
                "untested": modules,
                "dependencies": [],
            }
            save_state(state_file, state)

    pruner = PersistentPruner(modules, evaluator, state, state_file, args.profile)

    try:
        pruner.run()
    except KeyboardInterrupt:
        print("\nInterrupted! Saving state...")
        pruner.save_state()
        raise

    kept = sorted(set(modules) - pruner.drop_set)

    print("\n" + "=" * 60)
    print("FINAL RESULTS")
    print("=" * 60)
    print(f"{summary_label}: {evaluator.eval_count}")
    print(f"Modules dropped: {len(pruner.drop_set)}")
    print(f"Modules required: {len(kept)}")
    print(f"Modules untested: {len(pruner.untested)}")

    if pruner.untested:
        print("\nNote: Not all modules were tested. Run again to continue.")
    else:
        print("\nAll modules tested!")

    print(f"\nState saved to: {state_file}")
    print("\nRequired modules list:")
    print("-" * 40)
    for mod in kept:
        print(mod)

    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except KeyboardInterrupt:
        print("\nInterrupted", file=sys.stderr)
        sys.exit(130)
