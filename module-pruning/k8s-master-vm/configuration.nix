{
  config,
  pkgs,
  lib,
  ...
}:
let
  kubeAPIPort = 6443;
in
{
  nixpkgs.hostPlatform = "x86_64-linux";

  networking.hostName = "k8s-master";
  networking.useDHCP = lib.mkDefault true;

  services.kubernetes = {
    roles = [ "master" ];
    masterAddress = "k8s-master";
    apiserver = {
      advertiseAddress = "127.0.0.1";
      securePort = kubeAPIPort;
    };
    controllerManager.extraOpts = "--v=2";
  };

  networking.firewall.allowedTCPPorts = [ kubeAPIPort ];

  virtualisation = {
    memorySize = 4096;
    cores = 2;
    graphics = false;
    forwardPorts = [
      {
        from = "host";
        host.port = kubeAPIPort;
        guest.port = kubeAPIPort;
      }
    ];
  };

  environment.systemPackages = with pkgs; [
    kubectl
    kubernetes-helm
  ];

  system.stateVersion = "23.11";
}
