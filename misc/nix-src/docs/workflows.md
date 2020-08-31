

## Attributes

* load .src.json?
* write .nix?




```
nix-src pin . channel:nixos-18.03
# resolves to a github fetcher
# finds the last revision
# fetches the source and gets the sha256
# writes the default.src.json
# writes a standalone default.nix (abort with warning if it exists)
```


Update is like pin but also reads the .src.json to seed the arguments and doesn't write the default.nix
```
nix-src up path/to/file
# loads the .src.json
# finds that it's a github source
# update to the last revision (abort if it's the same)
# fetches the source and gets the sha256
# writes the updated default.src.json
```

```
nix-src up path/to/file --branch nixpkgs-unstable
# loads the default.src.json
# finds that it's a github source
# switch the branch to the passed argument
# uploae to the last revision
# fetches the source and gets the sha256
# writes the updates default.src.json
```

```
nix-src path/to/file

nix-src --file nixpkgs.src.json --force --branch=branch owner/repo@branch
```
