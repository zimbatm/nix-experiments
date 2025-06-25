# Minimal derivation without nixpkgs dependency
derivation {
  name = "test-file";
  system = builtins.currentSystem;
  builder = "/bin/sh";
  args = [ "-c" "echo 'original content' > $out" ];
}