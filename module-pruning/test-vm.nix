{
  dropModules ? [ ],
  profile ? "k8s-master",
}:
let
  system = "x86_64-linux";
  nixpkgsPath = builtins.toPath ../../../NixOS/nixpkgs;
  pkgs = import nixpkgsPath { inherit system; };
  lib = pkgs.lib;

  canonicalise =
    str:
    let
      absolutePrefix = (toString nixpkgsPath) + "/nixos/modules/";
      trimmed = if lib.hasPrefix absolutePrefix str then lib.removePrefix absolutePrefix str else str;
      stripped = if lib.hasPrefix "./" trimmed then lib.removePrefix "./" trimmed else trimmed;
    in
    stripped;

  normaliseModule =
    module:
    if builtins.isPath module then
      canonicalise (toString module)
    else if builtins.isAttrs module && module ? _file then
      canonicalise module._file
    else if builtins.typeOf module == "string" then
      canonicalise module
    else
      throw "Unsupported module value: ${builtins.toString module}";

  disabled = lib.unique (map normaliseModule dropModules);

  profileModules = {
    "k8s-master" = ./k8s-master-vm/configuration.nix;
  };

  userModule = lib.attrByPath [ profile ] (throw "Unsupported profile ${profile}") profileModules;

  testingLib = import (nixpkgsPath + "/nixos/lib/testing-python.nix") {
    inherit system pkgs;
  };

  test = testingLib.runTest {
    name = "k8s-master-pruned";

    defaults = {
      disabledModules = disabled;
    };

    nodes.machine =
      {
        config,
        pkgs,
        lib,
        ...
      }:
      {
        imports = [
          userModule
        ];

        nixpkgs.hostPlatform = system;
        virtualisation.memorySize = lib.mkDefault 2048;
        virtualisation.diskSize = lib.mkDefault 4096;
      };

    testScript = ''
      machine.start()
      machine.wait_for_unit("multi-user.target")

      # Check if Kubernetes API server is running
      machine.wait_for_unit("kube-apiserver.service")
      machine.wait_for_open_port(6443)

      kubeconfig = "/var/lib/kubernetes/secrets/admin.conf"
      machine.wait_until_succeeds(f"test -f {kubeconfig}")

      # Give the API server time to accept connections
      machine.wait_until_succeeds(f"KUBECONFIG={kubeconfig} kubectl version --short --request-timeout=10s")

      # Basic health probes
      machine.wait_until_succeeds(f"KUBECONFIG={kubeconfig} kubectl get --raw=/readyz?verbose")
      machine.succeed(f"KUBECONFIG={kubeconfig} kubectl cluster-info")

      # Check if the master node reports ready nodes
      machine.wait_until_succeeds(f"KUBECONFIG={kubeconfig} kubectl get nodes")

      # Basic API health check
      machine.succeed(f"KUBECONFIG={kubeconfig} kubectl get pods -A")
    '';
  };
in
test
