# nix-store-lan: a distributed nix store for your LAN

Have you ever built a derivation on one machine and then had to re-build it on
another? WTF?

Use nix-store-lan to automatically distribute your build results accross your
Local Area Network.

## Design

All machines that hold the shared signing key can contribute to the party.

Use mdns / other to discover new machines.

On build, broadcast the build result to all the known hosts.

On connect, fetch the build table from the other hosts.

Configure Nix to use the localhost HTTP server to fetch from the cache.

## Configuration

Open port UDP 5353

