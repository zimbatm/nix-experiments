with import <nixpkgs> { };
mkShell {
  buildInputs = [
    go
  ];

  shellHook = ''
    unset GOROOT GOPATH
    export GO111MODULE=on
  '';
}
