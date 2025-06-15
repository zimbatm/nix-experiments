## Simplified PRD: Nix Developer Sandbox

### 1. Overview

Developers and LLM agents often run untrusted code with broad system access. This tool provides a **secure**, **reproducible**, and **fast** development environment per project using **Nix**.

**Core Promise:** Safe-by-default environments that are portable and instant to enter.

### 2. Goals

1. **Security:** Restrict code to isolated project scopes.
2. **Reproducibility:** Use Nix to ensure consistent environments.
3. **Velocity:** Fast onboarding and instant re-entry.

### 3. User Stories

| Role          | Need                                                                 |
| ------------- | -------------------------------------------------------------------- |
| Developer     | Start coding immediately from a fresh clone, with no setup friction. |
| Consultant    | Fully isolated client environments.                                  |
| LLM Agent     | Secure sandbox to read/write/run project code safely.                |
| Returning Dev | Rejoin a project instantly.                                          |

### 4. Core Requirements

#### 4.1. Environment Definition

- Must support both **`flake.nix`** and **`devenv.nix`** as entry points.
- Use lockfiles (`flake.lock`, `devenv.lock`) to ensure reproducibility.
- Provide a `bash` shell (only `bash` in v1).

#### 4.2. Filesystem & Daemon Sandboxing

- Deny all host access by default.
- Allow full access to the project root.
- Allow **read-only** access to `/nix/store`.
- Allow **read-write** access to the Nix daemon socket (`/nix/var/nix/daemon-socket/socket`), required for builds inside the sandbox.
- Support per-project overrides (e.g., unshadow `~/.gitconfig`).
- Use native OS sandboxing:
  - macOS: `sandbox-exec`
  - Linux: user namespaces

#### 4.3. Git-Aware Session Management

- By default, the sandbox runs **in-place** in the current Git working directory.
- If a `<branch>` or session name is provided via `nix-sandbox enter --session <branch>`, it:
  - Creates a **separate Git workspace** linked to that branch (if not existing),
  - Checks out or creates the branch from `HEAD`,
  - Shares state if multiple sessions use the same branch.

#### 4.4. Performance

- Watch for changes to `flake.nix` / `devenv.nix` and prompt to reload.
- Cache environments for instant re-entry when unchanged.

### 5. Excluded in v1

- Secrets management
- IDE integration
- Global tooling overlays
- Shells other than `bash`
- Official Windows support

### 6. Assumptions

- Host has Nix (with flakes enabled) and Git.
- Projects are Git-managed.
