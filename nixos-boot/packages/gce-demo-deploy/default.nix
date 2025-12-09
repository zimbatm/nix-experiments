# GCE demo deployment script
#
# This script:
# 1. Uploads the installer image to GCE
# 2. Creates a VM with user-data to automatically install the demo system
# 3. Deletes old VMs with the same base name in the background
# 4. Shows how to get logs and connect
#
{
  pkgs,
  perSystem,
  lib ? pkgs.lib,
  writeShellApplication ? pkgs.writeShellApplication,
  ...
}:
let
  gceImage = perSystem.self.gce-installer-image;
  demoSystem = perSystem.self.demo-system;
  # Get the closure path from the passthru attribute (avoids IFD)
  demoClosurePath = "${demoSystem.passthru.toplevel}";

  # Extract the hash from the gce-installer-image store path for versioning
  # e.g., /nix/store/abc123...-gce-installer-image -> abc123...
  imageHash = builtins.substring 11 32 (baseNameOf gceImage.outPath);

  # User-data is generated at runtime to allow BINARY_CACHE override

in
writeShellApplication {
  name = "gce-demo-deploy";
  runtimeInputs = [
    pkgs.google-cloud-sdk
    pkgs.jq
    pkgs.cachix
    pkgs.curl
    pkgs.xxd
  ];
  text = ''
    set -euo pipefail

    # Configuration - override with environment variables
    PROJECT="''${GCP_PROJECT:-}"
    BUCKET="''${GCS_BUCKET:-nixos-boot-images}"
    ZONE="''${GCE_ZONE:-us-central1-a}"
    IMAGE_NAME="''${GCE_IMAGE_NAME:-nixos-boot-installer-${imageHash}}"
    VM_BASE_NAME="''${GCE_VM_NAME:-nixos-demo}"
    MACHINE_TYPE="''${GCE_MACHINE_TYPE:-e2-medium}"
    BINARY_CACHE="''${BINARY_CACHE:-https://numtide.cachix.org}"
    CACHIX_CACHE="''${CACHIX_CACHE:-numtide}"

    # Generate unique VM name with timestamp
    timestamp=$(date +%Y%m%d-%H%M%S)
    VM_NAME="''${VM_BASE_NAME}-''${timestamp}"

    # Demo system closure path (built into the script)
    DEMO_CLOSURE="${demoClosurePath}"

    if [ -z "$PROJECT" ]; then
      PROJECT=$(gcloud config get-value project 2>/dev/null || true)
      if [ -z "$PROJECT" ]; then
        echo "Error: No GCP project set. Set GCP_PROJECT or run 'gcloud config set project <project>'"
        exit 1
      fi
    fi

    echo "============================================"
    echo "NixOS Boot - GCE Demo Deployment"
    echo "============================================"
    echo ""
    echo "Configuration:"
    echo "  Project:      $PROJECT"
    echo "  Bucket:       $BUCKET"
    echo "  Zone:         $ZONE"
    echo "  Image:        $IMAGE_NAME"
    echo "  VM Base Name: $VM_BASE_NAME"
    echo "  VM Name:      $VM_NAME"
    echo "  Machine Type: $MACHINE_TYPE"
    echo ""
    echo "Demo System Closure:"
    echo "  $DEMO_CLOSURE"
    echo "  Binary Cache: $BINARY_CACHE"
    echo "  Cachix Cache: $CACHIX_CACHE"
    echo ""

    # Prepare user-data file (needed for comparison and VM creation)
    user_data_file=$(mktemp)
    trap 'rm -f "$user_data_file"' EXIT

    # Build binary_caches array - always include cache.nixos.org, prepend custom cache if set
    if [ -n "$BINARY_CACHE" ]; then
      binary_caches=$(jq -n --arg cache "$BINARY_CACHE" '[$cache, "https://cache.nixos.org"]')
    else
      binary_caches='["https://cache.nixos.org"]'
    fi

    # Generate user-data JSON
    jq -n \
      --arg closure "$DEMO_CLOSURE" \
      --argjson caches "$binary_caches" \
      '{
        system_closure: $closure,
        binary_caches: $caches,
        skip_network: false,
        partition_only: false
      }' > "$user_data_file"

    # Step 1: Push demo closure to cachix (skip if already in cache)
    echo "[1/6] Checking cachix for demo system closure..."
    # Try to fetch narinfo to check if it's already cached
    closure_hash=$(basename "$DEMO_CLOSURE" | cut -d- -f1)
    if curl -sf "https://$CACHIX_CACHE.cachix.org/$closure_hash.narinfo" >/dev/null 2>&1; then
      echo "  Closure already in $CACHIX_CACHE cache, skipping push"
    else
      echo "  Pushing $DEMO_CLOSURE to $CACHIX_CACHE..."
      cachix push "$CACHIX_CACHE" "$DEMO_CLOSURE"
    fi

    # Step 2: Create bucket if needed
    echo "[2/6] Checking GCS bucket..."
    if gsutil ls "gs://$BUCKET" &>/dev/null; then
      echo "  Bucket gs://$BUCKET exists"
    else
      echo "  Creating bucket gs://$BUCKET..."
      gsutil mb -p "$PROJECT" "gs://$BUCKET"
    fi

    # Step 3: Upload image (skip if already uploaded with same hash)
    echo "[3/6] Checking installer image in GCS..."
    local_hash=$(sha256sum "${gceImage}/gce-installer.tar.gz" | cut -d' ' -f1)
    remote_hash=$(gsutil hash -h "gs://$BUCKET/gce-installer.tar.gz" 2>/dev/null | grep "Hash (sha256)" | awk '{print $3}' | xxd -r -p | base64 -d 2>/dev/null | xxd -p || echo "")
    if [ "$local_hash" = "$remote_hash" ] 2>/dev/null; then
      echo "  Installer image already uploaded (hash matches), skipping"
    else
      echo "  Uploading installer image..."
      gsutil cp "${gceImage}/gce-installer.tar.gz" "gs://$BUCKET/gce-installer.tar.gz"
    fi

    # Step 4: Create GCE image (skip if exists)
    echo "[4/6] Checking GCE image..."
    if gcloud compute images describe "$IMAGE_NAME" --project="$PROJECT" &>/dev/null; then
      echo "  Image $IMAGE_NAME already exists, skipping creation"
    else
      echo "  Creating GCE image $IMAGE_NAME..."
      gcloud compute images create "$IMAGE_NAME" \
        --project="$PROJECT" \
        --source-uri="gs://$BUCKET/gce-installer.tar.gz" \
        --guest-os-features=UEFI_COMPATIBLE
    fi

    # Step 5: Delete old VMs in background
    echo "[5/6] Cleaning up old VMs..."
    echo "  Looking for VMs matching: $VM_BASE_NAME-*"

    # Find all VMs with the base name pattern and delete them in the background
    old_vms=$(gcloud compute instances list \
      --project="$PROJECT" \
      --filter="name~'^''${VM_BASE_NAME}-[0-9]{8}-[0-9]{6}$' AND zone:$ZONE" \
      --format='value(name)' 2>/dev/null || true)

    if [ -n "$old_vms" ]; then
      echo "  Found old VMs to delete:"
      echo "$old_vms" | while read -r vm; do
        echo "    - $vm"
      done
      echo "  Deleting old VMs in background..."
      # Delete all old VMs in the background (shell backgrounding, not gcloud --async)
      echo "$old_vms" | while read -r vm; do
        gcloud compute instances delete "$vm" \
          --zone="$ZONE" \
          --project="$PROJECT" \
          --quiet &
      done
      echo "  Old VM deletion started (running in background)"
    else
      echo "  No old VMs found"
    fi

    # Step 6: Create new VM immediately (don't wait for old ones to be deleted)
    echo "[6/6] Creating new VM..."
    echo "  User-data: $(cat "$user_data_file")"
    echo "  Creating VM $VM_NAME..."
    gcloud compute instances create "$VM_NAME" \
      --project="$PROJECT" \
      --zone="$ZONE" \
      --machine-type="$MACHINE_TYPE" \
      --image="$IMAGE_NAME" \
      --boot-disk-size=20GB \
      --boot-disk-type=pd-ssd \
      --metadata-from-file="user-data=$user_data_file" \
      --tags=nixos-demo

    echo ""
    echo "============================================"
    echo "Deployment Complete!"
    echo "============================================"
    echo ""
    echo "To view serial console output (installation progress):"
    echo "  gcloud compute instances get-serial-port-output $VM_NAME --zone=$ZONE --project=$PROJECT"
    echo ""
    echo "To tail logs continuously:"
    echo "  gcloud compute instances tail-serial-port-output $VM_NAME --zone=$ZONE --project=$PROJECT"
    echo ""
    echo "To connect via serial console:"
    echo "  gcloud compute connect-to-serial-port $VM_NAME --zone=$ZONE --project=$PROJECT"
    echo ""
    echo "Note: The installer will:"
    echo "  1. Boot from the installer UKI"
    echo "  2. Read user-data from GCE metadata"
    echo "  3. Partition the disk"
    echo "  4. Fetch the NixOS system from the binary cache"
    echo "  5. Install and reboot into NixOS"
    echo ""
    echo "After installation completes, SSH in (if you add SSH keys):"
    echo "  gcloud compute ssh $VM_NAME --zone=$ZONE --project=$PROJECT"
  '';
}
