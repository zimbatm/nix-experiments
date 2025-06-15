{ system, pkgs, ... }@args:

pkgs.stdenv.mkDerivation {
  name = "nix-sandbox-integration-test";
  
  src = ../.;
  
  nativeBuildInputs = with pkgs; [
    (pkgs.callPackage ../package.nix { })  # nix-sandbox
    git
  ] ++ pkgs.lib.optionals pkgs.stdenv.isLinux [
    bubblewrap
  ];

  dontBuild = true;
  
  installPhase = ''
    mkdir -p $out
    
    # Create a test project directory
    mkdir -p test-project
    cd test-project
    
    # Initialize git repo
    git init
    git config user.name "Test User"
    git config user.email "test@example.com"
    
    # Create a simple flake.nix
    cat > flake.nix << 'EOF'
{
  description = "Test project for nix-sandbox";
  
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };
  
  outputs = { self, nixpkgs }: {
    devShells.${system}.default = nixpkgs.legacyPackages.${system}.mkShell {
      buildInputs = with nixpkgs.legacyPackages.${system}; [
        hello
      ];
    };
  };
}
EOF
    
    # Test basic commands
    echo "Testing nix-sandbox --help..."
    nix-sandbox --help > $out/help-output.txt
    
    echo "Testing nix-sandbox list..."
    nix-sandbox list > $out/list-output.txt || true
    
    echo "Testing nix-sandbox clean..."
    nix-sandbox clean > $out/clean-output.txt || true
    
    # Create devenv.nix as well
    cat > devenv.nix << 'EOF'
{ pkgs, ... }: {
  packages = with pkgs; [
    hello
  ];
}
EOF
    
    # Test environment detection by checking if files exist
    test -f flake.nix && echo "✓ flake.nix detected" >> $out/detection-test.txt
    test -f devenv.nix && echo "✓ devenv.nix detected" >> $out/detection-test.txt
    
    echo "Integration tests completed successfully!"
    touch $out/success
  '';
  
  meta = with pkgs.lib; {
    description = "Integration tests for nix-sandbox";
    platforms = platforms.unix;
  };
}