# nix-store-edit

Another heretic tool.

Are you trying to edit a file on a production server? Skip changing the Nix
code, eval and instead, edit the file directly into the store.

This tool is majic :sparkles:

## Installation

```bash
go install github.com/zimbatm/nix-experiments/nix-store-edit@latest
```

Or build from source:

```bash
make build
```

## Usage

### Edit a file in the Nix store

```bash
nix-store-edit /nix/store/...-package/bin/program
```

This will:
1. Create a temporary mutable copy of the store path
2. Open it in your `$EDITOR` (defaults to vim)
3. After editing, create a new store path with your changes
4. Rewrite the system closure to use the new path (WIP)

### Current Limitations

- Uses `cp` for extraction instead of pure NAR operations (go-nix limitation)
- The `nixos-rebuild test` integration is untested on real NixOS systems
- No comprehensive integration tests with actual Nix store paths yet

## Credits

This is a Go port of the original shell scripts by edef and zimbatm.
