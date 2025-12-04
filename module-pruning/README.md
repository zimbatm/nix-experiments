# Module pruning harness

These helpers exercise the `module-list.nix` import path while evaluating the
Kubernetes master VM configuration. They make it easy to drop modules, compare
evaluation timing, and inspect the resulting option set.

## Baseline evaluation

```
cd module-pruning
./scripts/measure.sh
```

The script performs a warm-up `nix-instantiate -A vmDrv` and then runs a timed
instantiation. Passing `--drop config/appstream.nix` (repeatable flag) removes
modules by their relative path inside `nixos/modules` and re-runs the same
measurement. The final `modules kept:` line reports the new count.

## Inspecting options after pruning

Grab the option tree or individual values with `nix-instantiate --eval`:

```
nix-instantiate eval.nix -A options.services.kubernetes.roles.type.description \
  --arg dropModules '["config/appstream.nix"]' --argstr profile k8s-master
```

The `keptModules` attribute exposes the canonicalised module paths that remain
active, which is handy when diffing against the baseline list.

## Suggested workflow

1. Capture baseline timing and module count.
2. Use `./scripts/prune-modules.py` to propose a minimal module list that still
   evaluates, or trial a manual batch of removals via `--drop` flags.
3. Re-run the VM build (`nix-build ./k8s-master-vm/vm.nix -A config.system.build.vm`).
4. Boot the VM and run Kubernetes smoke tests (`kubectl get componentstatuses`).
5. Iterate while keeping an eye on the reported module count and the option
   attributes you expect to stay present.

## Automated pruning

```
cd module-pruning
./scripts/prune-modules.py
```

The script uses recursive bisection to drop the largest possible module batches
while keeping `eval.nix` evaluable. It prints the remaining required modules on
stdout and reports how many evaluations were needed. Pass extra arguments after
`--` to forward them to `nix-instantiate` (for example, `--argstr profile foo`).
