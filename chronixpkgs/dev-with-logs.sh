#!/usr/bin/env bash
set -euo pipefail

# Create logs directory if it doesn't exist
mkdir -p logs

# Ensure we're in the nix develop shell
if ! command -v hivemind &> /dev/null; then
    echo "Error: hivemind not found. Please run 'nix develop' first."
    exit 1
fi

# Log file with timestamp
LOG_FILE="logs/hivemind-$(date +%Y%m%d-%H%M%S).log"

echo "Starting hivemind with logging..."
echo "Logs will be written to: $LOG_FILE"
echo "Press Ctrl+C to stop"
echo ""

# Run hivemind and tee output to both console and log file
exec hivemind "$@" 2>&1 | tee "$LOG_FILE"