# Minimal test for handling complex dependency graphs
let
  # Two configs that reference each other's paths
  configA = derivation {
    name = "config-a";
    system = builtins.currentSystem;
    builder = "/bin/sh";
    # We'll fill in configB path later
    args = [ "-c" "echo 'Config A' > $out" ];
  };
  
  configB = derivation {
    name = "config-b";
    system = builtins.currentSystem;
    builder = "/bin/sh";
    args = [ "-c" ''
      echo "Config B" > $out
      echo "References config A at: ${configA}" >> $out
    '' ];
  };
  
  # Main bundle that references both
  bundle = derivation {
    name = "circular-test";
    system = builtins.currentSystem;
    builder = "/bin/sh";
    args = [ "-c" ''
      echo "Config A location: ${configA}" > $out
      echo "Config B location: ${configB}" >> $out
      echo "" >> $out
      echo "This creates a closure where both configs are referenced." >> $out
    '' ];
  };
in bundle