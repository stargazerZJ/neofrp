#!/bin/bash

# Helper script to run frps with auto-reload capability
# This script runs frps in a loop and can be triggered to reload via the /shutdown endpoint

FRPS_BIN="${FRPS_BIN:-./bin/frps}"
FRPS_CONFIG="${FRPS_CONFIG:-./frps.toml}"
FRPS_PORT="${FRPS_PORT:-7000}"

echo "Starting frps with auto-reload capability..."
echo "Binary: $FRPS_BIN"
echo "Config: $FRPS_CONFIG"
echo "Port: $FRPS_PORT"
echo ""
echo "To reload config/binary, run: curl http://localhost:$FRPS_PORT/shutdown"
echo "Press Ctrl+C to stop completely"
echo ""

while true; do
    echo "[$(date)] Starting frps..."
    $FRPS_BIN -c $FRPS_CONFIG
    EXIT_CODE=$?
    
    echo "[$(date)] frps exited with code $EXIT_CODE"
    
    # Small delay before restart to avoid rapid restart loops
    sleep 1
done