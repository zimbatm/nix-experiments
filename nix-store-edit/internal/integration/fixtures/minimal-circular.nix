# Minimal test for handling complex interdependencies
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
      cat > $out << EOF
      Config B
      References config A at: ${configA}
      EOF
    '' ];
  };
  
  # Main bundle that includes both
  bundle = derivation {
    name = "circular-test";
    system = builtins.currentSystem;
    builder = "/bin/sh";
    args = [ "-c" ''
      mkdir -p $out
      ln -s ${configA} $out/config-a
      ln -s ${configB} $out/config-b
    '' ];
  };
in bundle