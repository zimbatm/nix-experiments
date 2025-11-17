# Testing if the claim in 1000 instances of nixpkgs is true

Which is: if A and B follow the same instance of nixpkgs, are we avoiding
evaluating nixpkgs twice?

If it's true, `nix build root-flake` should only show "evaluation of nixpkgs" once.

## Result

```
$ nix build ./root-flake
trace: evaluation of nixpkgs
```
