#!/usr/bin/env bash
set -euo pipefail

# Check if nix-daemon is ready, exit if not (pod will restart)
while [ ! -d /nix/store ] || [ ! -S /nix/var/nix/daemon-socket/socket ]; do
  echo "Nix store or daemon socket not ready yet, exiting to restart..."
  sleep 1
done
echo "Nix store and daemon ready!"

# Setup user environment  
mkdir -p "$HOME"

# Setup SSH server
mkdir -p /workspace/sshd

# Generate host keys if they don't exist
if [ ! -f /workspace/ssh_host_ed25519_key ]; then
  ssh-keygen -t ed25519 -f /workspace/ssh_host_ed25519_key -N ""
fi

sshd_flags=(
  -D
  -f /dev/null
  -o AuthorizedKeysFile=/etc/ssh/authorized_keys
  -o ChallengeResponseAuthentication=no
  -o HostKey=/workspace/ssh_host_ed25519_key
  -o LogLevel=VERBOSE
  -o PasswordAuthentication=no
  -o PermitRootLogin=no
  -o PidFile=/workspace/sshd/sshd.pid
  -o Port=2222
  -o SetEnv="PATH=/workspace/home/.nix-profile/bin:/nix/var/nix/profiles/default/bin"
  -o StrictModes=no
  -o UseDNS=no
  -o UsePAM=no
)

# Start SSH daemon so we can use the ssh-ng:// store protocol.
exec /nix/var/nix/profiles/default/bin/sshd "${sshd_flags[@]}"
