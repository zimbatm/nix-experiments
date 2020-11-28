# A pure but simplified implementation of pkgs.writeText
#
# This script assumes that /bin/sh is part of the sandbox.
{ name
, text
, system ? builtins.currentSystem
}:
derivation {
  inherit name text system;

  # Pure sh implementation of cat
  cat = ''
    while IFS= read -r; do
      printf "%s\n" "$REPLY"
    done
  '';

  passAsFile = [ "text" "cat" ];

  builder = "/bin/sh";
  args = [ "-c" "/bin/sh $catPath < $textPath > $out" ];

  # Pointless to do this on a remote machine.
  preferLocalBuild = true;
  allowSubstitutes = false;
}
