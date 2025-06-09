#!/bin/sh
set -eu

# Start nix-daemon
echo "Starting nix-daemon..."
exec /nix/var/nix/profiles/default/bin/nix-daemon --daemon -v
