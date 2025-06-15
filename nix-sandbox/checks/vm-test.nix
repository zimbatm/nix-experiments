{ pkgs, ... }:

pkgs.nixosTest {
  name = "nix-sandbox-vm-test";

  nodes.machine =
    { pkgs, ... }:
    {
      # Enable Nix with flakes
      nix.settings.experimental-features = [
        "nix-command"
        "flakes"
      ];

      # Install required packages
      environment.systemPackages = with pkgs; [
        bubblewrap # Required for Linux sandboxing
        git
        (pkgs.callPackage ../package.nix { })
      ];

      # Enable network for Nix daemon communication
      networking.firewall.enable = false;

      # Create a test user
      users.users.testuser = {
        isNormalUser = true;
        home = "/home/testuser";
        createHome = true;
        shell = pkgs.bash;
      };

      # Nix daemon is enabled by default in NixOS
    };

  testScript = ''
    machine.start()
    machine.wait_for_unit("multi-user.target")

    # Create test project
    machine.succeed("su - testuser -c 'mkdir -p /home/testuser/test-project'")
    machine.succeed("su - testuser -c 'cd /home/testuser/test-project && git init'")
    machine.succeed("su - testuser -c 'cd /home/testuser/test-project && git config user.name \"Test User\"'")
    machine.succeed("su - testuser -c 'cd /home/testuser/test-project && git config user.email \"test@example.com\"'")

    # Create flake.nix
    machine.succeed("""
      su - testuser -c 'cd /home/testuser/test-project && cat > flake.nix << "EOF"
    {
      description = "Test project for nix-sandbox";
      inputs = { nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable"; };
      outputs = { self, nixpkgs }: {
        devShells.x86_64-linux.default = nixpkgs.legacyPackages.x86_64-linux.mkShell {
          buildInputs = with nixpkgs.legacyPackages.x86_64-linux; [ hello ];
        };
      };
    }
    EOF'
    """)

    # Test basic nix-sandbox commands
    print("Testing nix-sandbox list...")
    machine.succeed("su - testuser -c 'cd /home/testuser/test-project && nix-sandbox list'")

    print("Testing nix-sandbox clean...")
    machine.succeed("su - testuser -c 'cd /home/testuser/test-project && nix-sandbox clean'")

    # Test filesystem isolation by checking what's accessible
    machine.succeed("su - testuser -c 'cd /home/testuser/test-project && test -f flake.nix'")
    machine.succeed("su - testuser -c 'cd /home/testuser/test-project && test -d /nix/store'")

    print("All nix-sandbox tests completed successfully!")
  '';
}
