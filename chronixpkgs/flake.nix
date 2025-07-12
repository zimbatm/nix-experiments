{
  description = "Chronixpkgs - Nixpkgs Event Chronicle";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        packages = {
          default = pkgs.buildGoModule {
            pname = "chronixpkgs";
            version = "0.1.0";
            src = self;
            vendorHash = "sha256-k3KE6cvRpXQ0bLEDj08+pg1Zecx1P+VPYpdktjrUqWY=";
            
            meta = with pkgs.lib; {
              description = "Chronixpkgs - Public event chronicle for nixpkgs";
              homepage = "https://github.com/zimbatm/nix-experiments/chronixpkgs";
              license = licenses.mit;
            };
          };
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gopls
            golangci-lint
            gotools
            go-tools
            sqlite
          ];
        };

        # Systemd service module
        nixosModules.default = { config, lib, pkgs, ... }:
          with lib;
          let
            cfg = config.services.chronixpkgs;
          in
          {
            options.services.chronixpkgs = {
              enable = mkEnableOption "Chronixpkgs event chronicle service";

              package = mkOption {
                type = types.package;
                default = self.packages.${pkgs.system}.default;
                description = "Chronixpkgs package to use";
              };

              githubToken = mkOption {
                type = types.str;
                description = "GitHub API token";
              };

              webhookSecret = mkOption {
                type = types.str;
                description = "GitHub webhook secret";
              };

              repo = mkOption {
                type = types.str;
                default = "NixOS/nixpkgs";
                description = "GitHub repository to monitor (owner/repo)";
              };

              listen = mkOption {
                type = types.str;
                default = ":8080";
                description = "HTTP server listen address";
              };

              dataDir = mkOption {
                type = types.path;
                default = "/var/lib/chronixpkgs";
                description = "Data directory for storage";
              };

              retentionDays = mkOption {
                type = types.int;
                default = 90;
                description = "Days to retain events";
              };

              pollInterval = mkOption {
                type = types.str;
                default = "5m";
                description = "Polling interval for GitHub events";
              };

              enableCORS = mkOption {
                type = types.bool;
                default = false;
                description = "Enable CORS";
              };

              corsOrigins = mkOption {
                type = types.listOf types.str;
                default = [ "*" ];
                description = "Allowed CORS origins";
              };

              enableRateLimit = mkOption {
                type = types.bool;
                default = false;
                description = "Enable rate limiting";
              };

              rateLimit = mkOption {
                type = types.int;
                default = 60;
                description = "Requests per minute per IP";
              };
            };

            config = mkIf cfg.enable {
              systemd.services.chronixpkgs = {
                description = "Chronixpkgs - Nixpkgs Event Chronicle";
                after = [ "network.target" ];
                wantedBy = [ "multi-user.target" ];

                serviceConfig = {
                  Type = "simple";
                  ExecStart = "${cfg.package}/bin/chronixpkgs " +
                    "--repo ${cfg.repo} " +
                    "--listen ${cfg.listen} " +
                    "--data-dir ${cfg.dataDir} " +
                    "--retention-days ${toString cfg.retentionDays} " +
                    "--poll-interval ${cfg.pollInterval} " +
                    optionalString cfg.enableCORS "--enable-cors --cors-origins ${concatStringsSep "," cfg.corsOrigins} " +
                    optionalString cfg.enableRateLimit "--enable-rate-limit --rate-limit ${toString cfg.rateLimit}";
                  Restart = "always";
                  RestartSec = "10s";
                  
                  # Pass secrets via environment
                  Environment = [
                    "GITHUB_TOKEN=${cfg.githubToken}"
                    "WEBHOOK_SECRET=${cfg.webhookSecret}"
                  ];
                  
                  # Security hardening
                  DynamicUser = true;
                  StateDirectory = "chronixpkgs";
                  ProtectSystem = "strict";
                  ProtectHome = true;
                  NoNewPrivileges = true;
                  PrivateTmp = true;
                };
              };
            };
          };
      });
}
