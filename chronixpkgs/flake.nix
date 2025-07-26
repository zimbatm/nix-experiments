{
  description = "Chronixpkgs - Nixpkgs Event Chronicle";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    gorefresh.url = "github:draganm/gorefresh?ref=tags/v0.0.4";
    gorefresh.inputs.nixpkgs.follows = "nixpkgs";
  };

  outputs = { self, nixpkgs, flake-utils, gorefresh }:
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
            vendorHash = "sha256-2u+soOQvX0Q+EX66kN0g3S1dSr0SgsnEks0HVSuCqU0=";
            
            # Include templates and static files in the build
            postInstall = ''
              mkdir -p $out/share/chronixpkgs
              cp -r ${self}/templates $out/share/chronixpkgs/
              cp -r ${self}/static $out/share/chronixpkgs/
            '';
            
            meta = with pkgs.lib; {
              description = "Chronixpkgs - Public event chronicle for nixpkgs";
              homepage = "https://github.com/zimbatm/nix-experiments/chronixpkgs";
              license = licenses.mit;
            };
          };
          
          jaeger = pkgs.buildGoModule rec {
            pname = "jaeger";
            version = "1.52.0";
            
            src = pkgs.fetchFromGitHub {
              owner = "jaegertracing";
              repo = "jaeger";
              rev = "v${version}";
              sha256 = "sha256-jeI+Iw1vTbD0NhtmmcT7RzWKnfFvXX0O8iRvABRRmbA=";
            };
            
            vendorHash = "sha256-j0J1hCkRYXJLawmf9Yb0xPf1DCjlslj3dBPBIltnBP0=";
            
            subPackages = [ "cmd/all-in-one" ];
            
            postInstall = ''
              mv $out/bin/all-in-one $out/bin/jaeger-all-in-one
            '';
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
            hivemind
            watch
            jq
            curl
            gorefresh.packages.${system}.default
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
