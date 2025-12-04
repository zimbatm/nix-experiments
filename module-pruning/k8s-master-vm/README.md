# Kubernetes master QEMU VM (POC)

This directory contains a small NixOS configuration that enables the Kubernetes master role using the locally checked out nixpkgs at commit `cd33cfc252e785f0004590a2618f3644209eac07`.

## Build and run the VM

```
cd module-pruning/k8s-master-vm
nix-build vm.nix -A config.system.build.vm
./result/bin/run-k8s-master-vm
```

The run script forwards the Kubernetes API server (port 6443) from the guest to the host, and `kubectl` plus `helm` are part of the VM environment for quick smoke tests.
