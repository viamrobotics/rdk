#!/bin/bash
# Reproduces data loss during collector reinitialization.
# Starts a viam-server with a fake arm capturing at high frequency,
# triggers a reconfigure by editing the config file, then analyzes
# capture files for gaps.
#
# Usage:
#   ./run.sh [path/to/viam-server]
#
# If no binary is provided, it will be built from the repo root.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../../../.." && pwd)"

CAPTURE_DIR=$(mktemp -d)/capture
CONFIG_FILE=$(mktemp).json
CAPTURE_HZ=100
CAPTURE_DURATION=5
POST_RECONFIG_DURATION=3

# Build or locate viam-server
if [ -n "${1:-}" ]; then
    VIAM_SERVER="$1"
else
    echo "Building viam-server..."
    VIAM_SERVER="$(mktemp -d)/viam-server"
    (cd "$REPO_ROOT" && go build -o "$VIAM_SERVER" ./web/cmd/server)
    echo "Built: $VIAM_SERVER"
fi

# Write config
cat > "$CONFIG_FILE" <<EOF
{
    "network": {
        "bind_address": "localhost:18080"
    },
    "components": [
        {
            "name": "arm1",
            "type": "arm",
            "model": "fake",
            "service_configs": [
                {
                    "type": "data_manager",
                    "attributes": {
                        "capture_methods": [
                            {
                                "method": "EndPosition",
                                "capture_frequency_hz": $CAPTURE_HZ
                            }
                        ]
                    }
                }
            ]
        }
    ],
    "services": [
        {
            "name": "dm",
            "type": "data_manager",
            "model": "builtin",
            "attributes": {
                "sync_disabled": true,
                "capture_dir": "$CAPTURE_DIR",
                "capture_disabled": false
            }
        }
    ]
}
EOF

echo "Config:      $CONFIG_FILE"
echo "Capture dir: $CAPTURE_DIR"
echo "Frequency:   ${CAPTURE_HZ}Hz"
echo ""

# Start server
LOG_FILE=$(mktemp)
echo "Starting viam-server (log: $LOG_FILE)..."
"$VIAM_SERVER" -config "$CONFIG_FILE" > "$LOG_FILE" 2>&1 &
SERVER_PID=$!
trap 'kill $SERVER_PID 2>/dev/null; wait $SERVER_PID 2>/dev/null' EXIT

# Capture data
echo "Capturing for ${CAPTURE_DURATION}s..."
sleep "$CAPTURE_DURATION"

# Trigger reconfigure
NEW_HZ=$((CAPTURE_HZ - 1))
echo "Triggering reconfigure (${CAPTURE_HZ}Hz -> ${NEW_HZ}Hz)..."
if [[ "$(uname)" == "Darwin" ]]; then
    sed -i '' "s/\"capture_frequency_hz\": $CAPTURE_HZ/\"capture_frequency_hz\": $NEW_HZ/" "$CONFIG_FILE"
else
    sed -i "s/\"capture_frequency_hz\": $CAPTURE_HZ/\"capture_frequency_hz\": $NEW_HZ/" "$CONFIG_FILE"
fi

echo "Capturing for ${POST_RECONFIG_DURATION}s after reconfigure..."
sleep "$POST_RECONFIG_DURATION"

# Stop server
echo "Stopping server..."
kill "$SERVER_PID" 2>/dev/null
wait "$SERVER_PID" 2>/dev/null || true
trap - EXIT

echo ""
echo "=== Capture Files ==="
find "$CAPTURE_DIR" -type f
echo ""

# Analyze
echo "=== Analysis ==="
go run "$SCRIPT_DIR/analyze.go" "$CAPTURE_DIR" "$CAPTURE_HZ"
echo ""

# Show reconfigure log lines
echo "=== Reconfigure Log ==="
grep -E "closing collector|collector initialized|Reconfigure|config.*file changed" "$LOG_FILE" || true
