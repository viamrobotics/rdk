#!/bin/bash
# Reproduces data loss during collector reinitialization.
# Starts a viam-server with a counter sensor module capturing at high
# frequency, triggers a reconfigure by editing the config file, then
# analyzes capture files for gaps in the counter sequence.
#
# The counter sensor returns {"count": N} where N increments by 1 on
# each Readings call. If the captured sequence skips a value (e.g.
# 1, 2, 3, 5) then reading 4 was polled but never written to disk.
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

# Build counter sensor module
echo "Building counter sensor module..."
MODULE_BIN="$(mktemp -d)/countersensor"
(cd "$REPO_ROOT" && go build -o "$MODULE_BIN" ./services/datamanager/builtin/cmd/repro_capture_drop/countersensor)
echo "Built module: $MODULE_BIN"

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
    "modules": [
        {
            "name": "counter_module",
            "executable_path": "$MODULE_BIN"
        }
    ],
    "components": [
        {
            "name": "counter1",
            "type": "sensor",
            "model": "repro:test:countersensor",
            "depends_on": [],
            "service_configs": [
                {
                    "type": "data_manager",
                    "attributes": {
                        "capture_methods": [
                            {
                                "method": "Readings",
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
go run "$SCRIPT_DIR/analyze.go" "$CAPTURE_DIR"
echo ""

# Show reconfigure log lines
echo "=== Reconfigure Log ==="
grep -E "closing collector|collector initialized|Reconfigure|config.*file changed" "$LOG_FILE" || true
