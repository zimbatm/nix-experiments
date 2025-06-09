#!/usr/bin/env bash
set -euo pipefail

echo "Initializing /nix store volume and shared resources..."

# Set proper permissions for /tmp (1777 = sticky bit + rwxrwxrwx)
chmod 1777 /tmp
echo "Set /tmp permissions to 1777"

# Check if the shared volume already has /nix contents
if [ ! -e /nix-shared/.done ]; then
    # Remove any potential leftovers
    rm -rf /nix-shared/*
    
    echo "Copying base /nix contents to shared volume..."
    echo "This may take a minute on first run..."
    
    # Copy the entire /nix directory structure to the shared volume
    cp -a /nix/* /nix-shared/

    # Mark the end of the copy atomically
    touch /nix-shared/.done
    
    echo "Base /nix store copied successfully"
else
    echo "Shared volume already initialized"
fi

echo "Shared volume contains $(du -sh /nix-shared | cut -f1) of data"

echo "Init container completed successfully"
