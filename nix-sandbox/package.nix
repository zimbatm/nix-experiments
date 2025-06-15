{ pkgs }:

let
  inherit (pkgs) lib;
in

pkgs.rustPlatform.buildRustPackage {
  pname = "nix-sandbox";
  version = "0.1.0";

  src = ./.;

  cargoLock = {
    lockFile = ./Cargo.lock;
  };

  nativeBuildInputs = [
    pkgs.pkg-config
    pkgs.nix
    pkgs.git
    pkgs.makeWrapper
  ];

  # bubblewrap is required for Linux sandboxing
  buildInputs = lib.optionals pkgs.stdenv.isLinux [
    pkgs.bubblewrap
  ];

  # Required for tests
  checkInputs =
    [
      pkgs.git
    ]
    ++ lib.optionals pkgs.stdenv.isLinux [
      pkgs.bubblewrap
    ];

  # Set up environment for tests
  preCheck = ''
    export HOME=$(mktemp -d)
    export PATH=${pkgs.git}/bin:$PATH
  '';

  # Skip integration tests in Nix build as they require the binary to be in specific locations
  # Run unit tests only
  checkPhase = ''
    runHook preCheck
    cargo test --lib
    runHook postCheck
  '';

  # Ensure bubblewrap is available at runtime on Linux
  postInstall = lib.optionalString pkgs.stdenv.isLinux ''
    wrapProgram $out/bin/nix-sandbox \
      --prefix PATH : ${lib.makeBinPath [ pkgs.bubblewrap ]}
  '';

  meta = with lib; {
    description = "Secure, reproducible development environments using Nix";
    homepage = "https://github.com/zimbatm/nix-experiments";
    license = licenses.mit;
    maintainers = [ ];
    platforms = platforms.unix;
    mainProgram = "nix-sandbox";
  };
}
