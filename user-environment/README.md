# User environment.nix

Re-create nix-env profiles that are generated when using `nix-env -i`, but
purely with nix.

## Usage

[$ my-env.nix](my-env.nix)
```
let
  pkgs = import <nixpkgs> {};
  mkUserEnvironment = pkgs.callPackage ./. {};
in
mkUserEnvironment {
  derivations = [
    # Put the packages that you want in your user environment here
    pkgs.git
    pkgs.groff
    pkgs.hello
    pkgs.vim
  ];
}
```

This produces a user environment compatible with `nix-env`.

`$ nix-build ./my-env.nix`
```
/nix/store/gixpqzn5l510wachqy9slzq5rc8i600m-user-environment
```


Now the user can query the profile:
`$ nix-env --profile ./result -q`
```
git-minimal-2.24.0
groff-1.22.4
hello-2.10
vim-8.1.2237
```

## Uses cases

Provision user environments declaratively, but still allow later modifications
with `nix-env`.

## TODO

Test that the profile can be mutated later.

Test on my user profile for real.

Work on the user workflow.

Intercept `nix-env -i` to open my_env.nix into the EDITOR? What if the nix
file could be patched?

Import existing manifests?
