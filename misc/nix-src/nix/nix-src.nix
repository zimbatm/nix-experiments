{ buildPythonApplication, lib, docopt, srcPath ? ../. }:
let
  pname = "nix-src";
  version = lib.fileContents ./version.txt;

  # remove the /nix folder
  nixFilter = name: type:
    name == srcPath + "/nix" || srcPath + "/default.nix";

  src = lib.cleanSourceWith
    {
      filter = nixFilter;
      src = lib.cleanSource srcPath;
    }
    in
    buildPythonApplication {
    inherit pname version src;

  propagatedBuildInputs = [ docopt ];

  meta = with lib; {
    description = "nix source fetcher and pinning tool";
    homepage = "https://github.com/nix-community/nix-src";
    license = licenses.isc;
    maintainers = with maintainers; [ zimbatm ];
  };
  }
