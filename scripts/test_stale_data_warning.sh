#!/usr/bin/env bash
# test_stale_data_warning.sh — Manual test script for stale data warning feature.
#
# Validates that diskSummaryTracker logs:
#   - WARN when shouldSync()=true (sync enabled, no selective sync sensor)
#   - DEBUG when shouldSync()=false (selective sync sensor returns false)
#
# Usage: ./scripts/test_stale_data_warning.sh
# Runtime: ~12 minutes (build + two 5-minute tests)

set -euo pipefail

RDK_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
WORK_DIR="$(mktemp -d /tmp/stale-data-test-XXXXXX)"
SERVER_BIN="${RDK_ROOT}/bin/$(uname -s)-$(uname -m)/viam-server"
NOSYNC_MODULE_DIR="${WORK_DIR}/nosync-module"
NOSYNC_BIN="${WORK_DIR}/nosync-module/nosync-module"
CONFIG_WARN="${WORK_DIR}/config-warn.json"
CONFIG_DEBUG="${WORK_DIR}/config-debug.json"
LOG_WARN="${WORK_DIR}/test1-warn.log"
LOG_DEBUG="${WORK_DIR}/test2-debug.log"
REPORT="${WORK_DIR}/report.md"
CAPTURE_DIR_1="${WORK_DIR}/capture-test1"
CAPTURE_DIR_2="${WORK_DIR}/capture-test2"
SERVER_PID=""
WAIT_SECS=300  # 5 minutes

# Network ports for the two tests (avoid conflicts with running viam-server).
PORT_TEST1=18080
PORT_TEST2=18081

cleanup() {
    if [[ -n "${SERVER_PID}" ]] && kill -0 "${SERVER_PID}" 2>/dev/null; then
        echo ">>> Stopping viam-server (PID ${SERVER_PID})..."
        kill "${SERVER_PID}" 2>/dev/null || true
        wait "${SERVER_PID}" 2>/dev/null || true
    fi
    echo ">>> Work directory preserved at: ${WORK_DIR}"
}
trap cleanup EXIT

# --------------------------------------------------------------------------- #
# Build viam-server
# --------------------------------------------------------------------------- #
build_server() {
    echo ">>> Building viam-server..."
    cd "${RDK_ROOT}"
    make server
    if [[ ! -x "${SERVER_BIN}" ]]; then
        echo "ERROR: viam-server not found at ${SERVER_BIN}" >&2
        exit 1
    fi
    echo ">>> viam-server built: ${SERVER_BIN}"
}

# --------------------------------------------------------------------------- #
# Build nosync-sensor module
# --------------------------------------------------------------------------- #
build_nosync_module() {
    echo ">>> Building nosync-sensor module..."
    mkdir -p "${NOSYNC_MODULE_DIR}"

    cat > "${NOSYNC_MODULE_DIR}/main.go" << 'GOEOF'
package main

import (
	"context"

	"go.viam.com/utils"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
)

var model = resource.NewModel("test", "test", "nosync")

func main() {
	utils.ContextualMainWithSIGPIPE(mainWithArgs, module.NewLoggerFromArgs("nosync-module"))
}

func mainWithArgs(ctx context.Context, args []string, logger logging.Logger) error {
	myMod, err := module.NewModuleFromArgs(ctx)
	if err != nil {
		return err
	}

	resource.RegisterComponent(
		sensor.API,
		model,
		resource.Registration[resource.Resource, resource.NoNativeConfig]{
			Constructor: func(
				ctx context.Context,
				deps resource.Dependencies,
				conf resource.Config,
				logger logging.Logger,
			) (resource.Resource, error) {
				return &nosyncSensor{Named: conf.ResourceName().AsNamed()}, nil
			},
		},
	)

	if err := myMod.AddModelFromRegistry(ctx, sensor.API, model); err != nil {
		return err
	}

	if err := myMod.Start(ctx); err != nil {
		return err
	}
	defer myMod.Close(ctx)
	<-ctx.Done()
	return nil
}

type nosyncSensor struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
}

func (s *nosyncSensor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{"should_sync": false}, nil
}
GOEOF

    cat > "${NOSYNC_MODULE_DIR}/go.mod" << MODEOF
module nosync-module

go 1.22

require (
	go.viam.com/rdk v0.0.0
	go.viam.com/utils v0.1.120
)

replace go.viam.com/rdk => ${RDK_ROOT}
MODEOF

    cd "${NOSYNC_MODULE_DIR}"
    go mod tidy
    go build -o "${NOSYNC_BIN}" .
    if [[ ! -x "${NOSYNC_BIN}" ]]; then
        echo "ERROR: nosync-module not found at ${NOSYNC_BIN}" >&2
        exit 1
    fi
    echo ">>> nosync-module built: ${NOSYNC_BIN}"
}

# --------------------------------------------------------------------------- #
# Write config files
# --------------------------------------------------------------------------- #
write_configs() {
    echo ">>> Writing config files..."

    # Test 1 config: capture + sync enabled, no cloud, no selective syncer.
    # shouldSync() = SchedulerEnabled() [true, since sync not disabled] && ReadyToSyncDirectories() [true, no selective sensor]
    # => shouldSync() = true => expect WARN
    cat > "${CONFIG_WARN}" << JSONEOF
{
    "network": {
        "fqdn": "stale-data-test-1",
        "bind_address": "localhost:${PORT_TEST1}"
    },
    "components": [
        {
            "name": "fake-arm",
            "type": "arm",
            "model": "fake",
            "service_configs": [
                {
                    "type": "data_manager",
                    "attributes": {
                        "capture_methods": [
                            {
                                "method": "EndPosition",
                                "capture_frequency_hz": 10
                            }
                        ]
                    }
                }
            ]
        }
    ],
    "services": [
        {
            "name": "data_manager",
            "type": "data_manager",
            "model": "builtin",
            "attributes": {
                "capture_disabled": false,
                "sync_disabled": false,
                "sync_interval_mins": 0.1,
                "capture_dir": "${CAPTURE_DIR_1}"
            }
        }
    ]
}
JSONEOF

    # Test 2 config: same + nosync module + selective syncer returning false.
    # shouldSync() = SchedulerEnabled() [true] && ReadyToSyncDirectories() [false, sensor returns should_sync=false]
    # => shouldSync() = false => expect DEBUG
    cat > "${CONFIG_DEBUG}" << JSONEOF
{
    "network": {
        "fqdn": "stale-data-test-2",
        "bind_address": "localhost:${PORT_TEST2}"
    },
    "modules": [
        {
            "name": "nosync-module",
            "executable_path": "${NOSYNC_BIN}",
            "type": "local"
        }
    ],
    "components": [
        {
            "name": "fake-arm",
            "type": "arm",
            "model": "fake",
            "service_configs": [
                {
                    "type": "data_manager",
                    "attributes": {
                        "capture_methods": [
                            {
                                "method": "EndPosition",
                                "capture_frequency_hz": 10
                            }
                        ]
                    }
                }
            ]
        },
        {
            "name": "nosync-sensor",
            "type": "sensor",
            "model": "test:test:nosync",
            "depends_on": []
        }
    ],
    "services": [
        {
            "name": "data_manager",
            "type": "data_manager",
            "model": "builtin",
            "attributes": {
                "capture_disabled": false,
                "sync_disabled": false,
                "sync_interval_mins": 0.1,
                "selective_syncer_name": "nosync-sensor",
                "capture_dir": "${CAPTURE_DIR_2}"
            }
        }
    ]
}
JSONEOF

    echo ">>> Configs written: ${CONFIG_WARN} and ${CONFIG_DEBUG}"
}

# --------------------------------------------------------------------------- #
# Run a single test
# --------------------------------------------------------------------------- #
run_test() {
    local test_name="$1"
    local config_file="$2"
    local log_file="$3"

    echo ""
    echo "============================================================"
    echo ">>> Starting ${test_name}"
    echo ">>> Config: ${config_file}"
    echo ">>> Log:    ${log_file}"
    echo ">>> Waiting ${WAIT_SECS}s for stale data warning..."
    echo "============================================================"

    "${SERVER_BIN}" -config "${config_file}" -debug > "${log_file}" 2>&1 &
    SERVER_PID=$!
    sleep 3
    if ! kill -0 "${SERVER_PID}" 2>/dev/null; then
        echo "ERROR: viam-server failed to start. Log tail:" >&2
        tail -20 "${log_file}" >&2
        SERVER_PID=""
        return 1
    fi
    echo ">>> viam-server started (PID ${SERVER_PID})"

    # Wait with progress output every 30 seconds.
    local elapsed=0
    local interval=30
    while [[ ${elapsed} -lt ${WAIT_SECS} ]]; do
        sleep ${interval}
        elapsed=$((elapsed + interval))
        echo "    ... ${elapsed}/${WAIT_SECS}s elapsed"
    done

    echo ">>> Stopping viam-server (PID ${SERVER_PID})..."
    kill "${SERVER_PID}" 2>/dev/null || true
    wait "${SERVER_PID}" 2>/dev/null || true
    SERVER_PID=""

    echo ">>> ${test_name} complete. Log: ${log_file}"
}

# --------------------------------------------------------------------------- #
# Analyze logs and write report
# --------------------------------------------------------------------------- #
write_report() {
    local stale_msg="Capture data may not be syncing"
    local test1_pass="FAIL"
    local test2_pass="FAIL"
    local test1_lines=""
    local test2_lines=""

    echo ""
    echo ">>> Analyzing logs..."

    # Test 1: expect WARN level with the stale message
    test1_lines=$(grep "${stale_msg}" "${LOG_WARN}" 2>/dev/null || true)
    if echo "${test1_lines}" | grep -q "WARN"; then
        test1_pass="PASS"
    fi

    # Test 2: expect DEBUG level with the stale message
    test2_lines=$(grep "${stale_msg}" "${LOG_DEBUG}" 2>/dev/null || true)
    if echo "${test2_lines}" | grep -q "DEBUG"; then
        test2_pass="PASS"
    fi

    cat > "${REPORT}" << REPORTEOF
# Stale Data Warning Test Report

**Date:** $(date -u '+%Y-%m-%d %H:%M:%S UTC')
**Branch:** $(cd "${RDK_ROOT}" && git rev-parse --abbrev-ref HEAD)
**Commit:** $(cd "${RDK_ROOT}" && git rev-parse --short HEAD)
**Work Dir:** ${WORK_DIR}

## Results

| Test | Expected Level | Result |
|------|---------------|--------|
| Test 1: shouldSync()=true (no selective syncer) | WARN | **${test1_pass}** |
| Test 2: shouldSync()=false (nosync sensor) | DEBUG | **${test2_pass}** |

## Test 1: WARN (shouldSync=true)

**Config:** Capture + sync enabled, no cloud connection, no selective sync sensor.
**Expected:** \`shouldSync()=true\` → WARN-level stale data message.

### Matching log lines:
\`\`\`
${test1_lines:-<no matching lines found>}
\`\`\`

## Test 2: DEBUG (shouldSync=false)

**Config:** Capture + sync enabled, selective sync sensor returning \`should_sync: false\`.
**Expected:** \`shouldSync()=false\` → DEBUG-level stale data message.

### Matching log lines:
\`\`\`
${test2_lines:-<no matching lines found>}
\`\`\`

## Files

- Test 1 config: \`${CONFIG_WARN}\`
- Test 1 log: \`${LOG_WARN}\`
- Test 2 config: \`${CONFIG_DEBUG}\`
- Test 2 log: \`${LOG_DEBUG}\`
REPORTEOF

    echo ""
    echo "============================================================"
    echo ">>> REPORT: ${REPORT}"
    echo ">>> Test 1 (WARN):  ${test1_pass}"
    echo ">>> Test 2 (DEBUG): ${test2_pass}"
    echo "============================================================"

    if [[ "${test1_pass}" == "PASS" && "${test2_pass}" == "PASS" ]]; then
        echo ">>> ALL TESTS PASSED"
    else
        echo ">>> SOME TESTS FAILED — see report for details"
    fi
}

# --------------------------------------------------------------------------- #
# Main
# --------------------------------------------------------------------------- #
echo ">>> Stale Data Warning Test"
echo ">>> RDK root: ${RDK_ROOT}"
echo ">>> Work dir: ${WORK_DIR}"
echo ""

build_server
build_nosync_module
write_configs
run_test "Test 1: WARN (shouldSync=true)" "${CONFIG_WARN}" "${LOG_WARN}"
run_test "Test 2: DEBUG (shouldSync=false)" "${CONFIG_DEBUG}" "${LOG_DEBUG}"
write_report

echo ""
echo ">>> Done. Report at: ${REPORT}"
