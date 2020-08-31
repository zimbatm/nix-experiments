{ stdenv
, lib
}:
stdenv.mkDerivation {
  name = "devenv";

  src = ./devenv;

  doBuild = false;

  installPhase = ''
    mkdir $out
    cp -r ./bin $out/bin
    cp -r ./libexec $out/libexec
    cp -r ./share $out/share
  '';
}
