# k8s Nix trampoline

The goal of this project is to explore using Kubernetes as a build system for
Nix. Where you have a local flake, and then push it to Kubernetes (that
potentially runs on a different architecture). And then run commands in the
same environment (like pushing docker images to the cluster-specific registry).

The way to do this, is to spin up a "nix" pod. The pod `/nix` folder is mounted
in a volume. The pod has two containers: one that runs the nix-daemon, in
privileged mode. And one for the user where they can get the source, and talk
to the daemon.

Locally, we use Minikube to stand up a cluster. We limit the cluster memory to
16GB and 8 CPUs to keep things tight.

In the flake itself, we add some scripts to package the flake and all its
dependencies, and then ship that to the Kubernetes cluster.

## Quick Start

```bash
# Run the full test suite
./test.sh

# Or run individual commands:
nix develop                      # Enter dev environment  
./nix-trampoline.sh setup        # Create pod
./nix-trampoline.sh push         # Push git archive
./nix-trampoline.sh build        # Build current flake
./nix-trampoline.sh shell        # Interactive shell
./nix-trampoline.sh clean        # Clean up

# Build a specific derivation
./nix-trampoline.sh build -f hello.nix
```

## Architecture

- **PVC**: Persistent volume for shared `/nix` store
- **nix-daemon**: Privileged container that copies base store and bind-mounts
- **user**: Unprivileged container (UID 1000) for builds
- **Sync**: User container waits for daemon initialization


