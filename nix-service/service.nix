# Playing with how a service definition could look like.
{ pkgs, config, lib, }:
let
  cfg = config.svc;
in
{
  # FIXME: this "svc" name should be a stable prefix. Is that really the best
  # name?
  options.svc = {
    package = lib.mkPackageOption pkgs "nginx" { };

    # TODO: make this the default?
    port = lib.mkOption {
      type = lib.types.int;
      default = 80;
    };

    settings = lib.mkSettingsOption {
      format = "json";
      default = { };
    };
  };

  config = {
    # The output is a set of containers.
    services.default = {
      command = [(lib.getExe cfg.package)];

      ports = [
        { tcp = 80; }
      ];

      socketActivation = true;
      stateDirectory = true;

    };
  };
}
