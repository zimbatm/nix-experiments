# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Nix experiment exploring Kubernetes as a build system for Nix. The project creates a "trampoline" system where you can run Nix builds in a Kubernetes cluster, potentially on different architectures than your local machine.

## Common Commands

```bash
# Development environment
nix develop                      # Enter dev shell with minikube, kubectl, k9s

# Full test suite
./test.sh                        # Run complete end-to-end test

# Trampoline operations
./nix-trampoline.sh setup        # Create and start nix pod
./nix-trampoline.sh push-tarball # Push git archive to pod
./nix-trampoline.sh push-flake   # Push flake archive to pod
./nix-trampoline.sh build        # Run nix build in pod
./nix-trampoline.sh build -f hello.nix  # Build specific derivation
./nix-trampoline.sh shell        # Interactive shell in pod
./nix-trampoline.sh ssh          # Get SSH connection info
./nix-trampoline.sh copy-closure <path>  # Copy closure via SSH
./nix-trampoline.sh maintenance  # Run GC and cleanup
./nix-trampoline.sh clean        # Delete pod and resources

# Minikube management
minikube start --memory=16384 --cpus=8 --driver=docker
minikube status
kubectl config use-context minikube
```

## Architecture

**Pod Structure:**
- **PVC**: 10Gi persistent volume for shared `/nix` store
- **nix-daemon container**: Privileged (nixos/nix:latest), copies base store and bind-mounts
- **user container**: Unprivileged (UID 1000), runs builds and provides SSH access
- **Synchronization**: User container waits for daemon initialization

**Key Features:**
- SSH server with pre-configured public key access
- Auto-shutdown monitoring with activity detection
- Resource limits (16GB memory, 8 CPUs for daemon; 8GB memory, 4 CPUs for user)
- Shared `/nix` store between containers via persistent volume

## Development Environment

This project uses Nix flakes with the Blueprint framework:

- `flake.nix`: Uses Blueprint framework for outputs
- `devshell.nix`: Defines development environment with minikube, kubectl, k9s
- Dependencies managed through Nix flake inputs (nixpkgs, blueprint)

## Container Configuration

Located in `container/` directory:
- `k8s-pod.yaml`: Main pod definition with PVC
- `kustomization.yaml`: Kustomize configuration
- `nix-daemon-startup.sh`: Daemon initialization script
- `user-startup.sh`: User container startup with SSH server
- `shutdown-monitor.sh`: Auto-shutdown based on activity
- `maintenance.sh`: Periodic GC and optimization
- `nix.conf`: Nix configuration shared between containers

## Testing

The `test.sh` script provides a complete end-to-end test:
1. Starts minikube if needed
2. Sets up the pod
3. Pushes repository archive
4. Builds test derivation (hello.nix)
5. Verifies shell access and program execution

## Data Transfer Methods

- **Git archive**: `push-tarball` exports and transfers git archive
- **Flake archive**: `push-flake` uses `nix flake archive` for dependencies
- **SSH closure**: `copy-closure` uses nix-copy-closure over SSH

## Agent Instructions

- Run `git add` on the files you create

## Memories

- Stop saying that I'm right, it's annoying. Push back, or just do it.
- never `cd` into a directory
- in bash scripts, prefer [[ ]] notation over [ ]
- in bash scripts, use `local` and underscore variables unless they are
    environment variables.
