# Create a derivation with dependencies for testing rewrite performance
let
  dep1 = derivation {
    name = "dep1";
    system = builtins.currentSystem;
    builder = "/bin/sh";
    args = [ "-c" "echo 'dep1' > $out" ];
  };
  
  dep2 = derivation {
    name = "dep2";
    system = builtins.currentSystem;
    builder = "/bin/sh";
    args = [ "-c" "echo 'dep2' > $out; echo ${dep1} >> $out" ];
  };
  
  dep3 = derivation {
    name = "dep3";
    system = builtins.currentSystem;
    builder = "/bin/sh";
    args = [ "-c" "echo 'dep3' > $out; echo ${dep2} >> $out" ];
  };
in
derivation {
  name = "main-package";
  system = builtins.currentSystem;
  builder = "/bin/sh";
  args = [ "-c" "echo 'main content' > $out; echo ${dep1} ${dep2} ${dep3} >> $out" ];
}