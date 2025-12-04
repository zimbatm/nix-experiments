{
  dropModules ? [ ],
  extraModules ? [ ],
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

  moduleKey =
    module:
    let
      raw =
        if builtins.isPath module then
          toString module
        else if builtins.isAttrs module && module ? _file then
          module._file
        else if builtins.typeOf module == "string" then
          module
        else
          null;
    in
    if raw == null then null else canonicalise raw;

  dropSet = builtins.listToAttrs (
    map (path: {
      name = canonicalise path;
      value = true;
    }) dropModules
  );

  moduleList = import (nixpkgsPath + "/nixos/modules/module-list.nix");

  prunedModules = builtins.filter (
    module:
    let
      key = moduleKey module;
    in
    key != null && !(builtins.hasAttr key dropSet)
  ) moduleList;

  profileModules = {
    "k8s-master" = ./k8s-master-vm/configuration.nix;
  };

  userModule = lib.attrByPath [ profile ] (throw "Unsupported profile ${profile}") profileModules;

  evalConfig = import (nixpkgsPath + "/nixos/lib/eval-config.nix");

  evaluation = evalConfig {
    inherit system;
    baseModules = prunedModules;
    modules = extraModules ++ [
      (nixpkgsPath + "/nixos/modules/virtualisation/qemu-vm.nix")
      userModule
    ];
  };

  prunedModuleKeys = builtins.filter (key: key != null) (lib.unique (map moduleKey prunedModules));
in
{
  moduleCount = builtins.length moduleList;
  prunedModuleCount = builtins.length prunedModules;
  keptModules = prunedModuleKeys;
  config = evaluation.config;
  options = evaluation.options;
  vmDrv = evaluation.config.system.build.vm;
  toplevelDrv = evaluation.config.system.build.toplevel;
}
