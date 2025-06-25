{ pkgs }:

pkgs.buildGoModule rec {
  pname = "nix-patch";
  version = "0.1.0";

  src = ./.;

  vendorHash = "sha256-rWtBgKc94woa4941wuovyuk+kQTPfXEYwRaTyZhsJ2k=";

  ldflags = [ "-s" "-w" ];
  
  nativeCheckInputs = [ pkgs.nix ];
  
  # Skip integration tests for now
  checkPhase = ''
    runHook preCheck
    
    # Run tests for each package individually, skipping integration tests
    go test ./cmd
    go test ./internal/archive
    go test ./internal/config
    go test ./internal/errors
    go test ./internal/patch
    go test ./internal/rewrite
    go test ./internal/store
    go test ./internal/system
    
    runHook postCheck
  '';

  meta = with pkgs.lib; {
    description = "Tool to patch Nix store paths";
    homepage = "https://github.com/zimbatm/nix-experiments/tree/main/nix-patch";
    license = licenses.mit;
    maintainers = with maintainers; [ zimbatm ];
    mainProgram = "nix-store-edit";
  };
}
