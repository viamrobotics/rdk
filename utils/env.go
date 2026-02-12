package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"go.viam.com/rdk/logging"
)

const (
	// DefaultResourceConfigurationTimeout is the default resource configuration
	// timeout.
	DefaultResourceConfigurationTimeout = 2 * time.Minute

	// ResourceConfigurationTimeoutEnvVar is the environment variable that can
	// be set to override DefaultResourceConfigurationTimeout as the duration
	// that resources are allowed to (re)configure.
	ResourceConfigurationTimeoutEnvVar = "VIAM_RESOURCE_CONFIGURATION_TIMEOUT"

	// DefaultModuleStartupTimeout is the default module startup timeout.
	DefaultModuleStartupTimeout = 5 * time.Minute

	// ModuleStartupTimeoutEnvVar is the environment variable that can
	// be set to override DefaultModuleStartupTimeout as the duration
	// that modules are allowed to startup.
	ModuleStartupTimeoutEnvVar = "VIAM_MODULE_STARTUP_TIMEOUT"

	// DefaultConfigReadTimeout is the default config read timeout. If there
	// is a cached config on the machine, a shorter default timeout will be used.
	DefaultConfigReadTimeout = 15 * time.Second

	// ConfigReadTimeoutEnvVar is the environment variable that can
	// be set to override DefaultConfigReadTimeout as the duration
	// for config read.
	ConfigReadTimeoutEnvVar = "VIAM_CONFIG_READ_TIMEOUT"

	// AndroidFilesDir is hardcoded because golang inits before our android code can override HOME var.
	AndroidFilesDir = "/data/user/0/com.viam.rdk.fgservice/cache"

	// ViamEnvVarPrefix is the prefix for all Viam-related environment variables.
	ViamEnvVarPrefix = "VIAM_"

	// APIKeyEnvVar is the environment variable which contains an API key that can be used for
	// communications to app.viam.com.
	//nolint:gosec
	APIKeyEnvVar = "VIAM_API_KEY"

	// APIKeyIDEnvVar is the environment variable which contains an API key ID that can be used for
	// communications to app.viam.com.
	//nolint:gosec
	APIKeyIDEnvVar = "VIAM_API_KEY_ID"

	// MachineFQDNEnvVar is the environment variable that contains the fqdn of the machine.
	MachineFQDNEnvVar = "VIAM_MACHINE_FQDN"

	// MachineIDEnvVar is the environment variable that contains the machine ID of the machine.
	MachineIDEnvVar = "VIAM_MACHINE_ID"

	// MachinePartIDEnvVar is the environment variable that contains the machine part ID of the machine.
	MachinePartIDEnvVar = "VIAM_MACHINE_PART_ID"

	// LocationIDEnvVar is the environment variable that contains the location ID of the machine.
	LocationIDEnvVar = "VIAM_LOCATION_ID"

	// PrimaryOrgIDEnvVar is the environment variable that contains the primary org ID of the machine.
	PrimaryOrgIDEnvVar = "VIAM_PRIMARY_ORG_ID"

	// HomeEnvVar is the environment variable that contains the VIAM_HOME directory of the machine.
	HomeEnvVar = "VIAM_HOME"

	// ViamResourceRequestsLimitEnvVar is the environment that controls the
	// per-resource gRPC request limit. If it is unset or invalid the limit
	// defaults to 100.
	ViamResourceRequestsLimitEnvVar = "VIAM_RESOURCE_REQUESTS_LIMIT"

	// ViamModuleTracingEnvVar is the environment variable that configures
	// modules to record trace spans and send them to their parent viam-server
	// process. Any non-empty string other than "0" or "false" enables module
	// tracing.
	ViamModuleTracingEnvVar = "VIAM_MODULE_TRACING"

	// ViamAgentHandlesNeedsRestartChecking is the environment variable that viam-agent will
	// set before starting viam-server to indicate that agent is a new enough version to
	// have its own background loop that runs NeedsRestart against app.viam.com to determine
	// if the system needs a restart. MUST be kept in line with the equivalent value in the
	// agent repo.
	//
	// TODO(RSDK-12057): Remove sensitivity to this environment variable once we fully
	// remove all NeedsRestart checking logic from viam-server.
	ViamAgentHandlesNeedsRestartChecking = "VIAM_AGENT_HANDLES_NEEDS_RESTART_CHECKING"

	// ViamTCPSocketsEnvVar if set to a true-like value, indicates that TCP sockets should be used
	// in lieu of Unix sockets.
	ViamTCPSocketsEnvVar = "VIAM_TCP_SOCKETS"
)

// EnvTrueValues contains strings that we interpret as boolean true in env vars.
var EnvTrueValues = []string{"true", "yes", "1", "TRUE", "YES"}

// TCPRegex tests whether a module address is TCP (vs unix sockets). See also OnlyUseViamTCPSockets().
var TCPRegex = regexp.MustCompile(`:\d+$`)

// ViamDotDir is the directory for Viam's cached files.
var ViamDotDir = filepath.Join(PlatformHomeDir(), ".viam")

var windowsPathRegex = regexp.MustCompile(`^(\w:)?(.+)$`)

// GetResourceConfigurationTimeout calculates the resource configuration
// timeout (env variable value if set, DefaultResourceConfigurationTimeout
// otherwise).
func GetResourceConfigurationTimeout(logger logging.Logger) time.Duration {
	timeout, _ := timeoutHelper(DefaultResourceConfigurationTimeout, ResourceConfigurationTimeoutEnvVar, logger)
	return timeout
}

// GetModuleStartupTimeout calculates the module startup timeout
// (env variable value if set, DefaultModuleStartupTimeout otherwise).
func GetModuleStartupTimeout(logger logging.Logger) time.Duration {
	timeout, _ := timeoutHelper(DefaultModuleStartupTimeout, ModuleStartupTimeoutEnvVar, logger)
	return timeout
}

// GetConfigReadTimeout returns the config read timeout set by the env variable value,
// DefaultConfigReadTimeout otherwise.
func GetConfigReadTimeout(logger logging.Logger) (time.Duration, bool) {
	return timeoutHelper(DefaultConfigReadTimeout, ConfigReadTimeoutEnvVar, logger)
}

func timeoutHelper(defaultTimeout time.Duration, timeoutEnvVar string, logger logging.Logger) (time.Duration, bool) {
	if timeoutVal := os.Getenv(timeoutEnvVar); timeoutVal != "" {
		timeout, err := time.ParseDuration(timeoutVal)
		if err != nil {
			logger.Warnf("Failed to parse %s env var, falling back to default %v timeout",
				timeoutEnvVar, defaultTimeout)
			return defaultTimeout, true
		}
		return timeout, false
	}
	return defaultTimeout, true
}

// PlatformHomeDir wraps Getenv("HOME"), except on android, where it returns the app cache directory.
func PlatformHomeDir() string {
	if runtime.GOOS == "android" {
		return AndroidFilesDir
	}
	if runtime.GOOS == "windows" { //nolint:goconst
		homedir, _ := os.UserHomeDir() //nolint:errcheck
		if homedir != "" {
			return homedir
		}
	}
	return os.Getenv("HOME")
}

// PlatformMkdirTemp wraps MkdirTemp. On android, when dir is empty, it uses a path
// that is writable + executable.
func PlatformMkdirTemp(dir, pattern string) (string, error) {
	if runtime.GOOS == "android" && dir == "" {
		dir = AndroidFilesDir
	}
	return os.MkdirTemp(dir, pattern)
}

// LogViamEnvVariables logs the list of viam environment variables in [os.Environ] along with the env passed in.
func LogViamEnvVariables(msg string, envVars map[string]string, logger logging.Logger) {
	var env []string
	for _, v := range os.Environ() {
		if !strings.HasPrefix(v, ViamEnvVarPrefix) {
			continue
		}
		env = append(env, v)
	}
	for key, val := range envVars {
		// mask the secret
		if key == APIKeyEnvVar {
			val = "XXXXXXXXXX"
		}
		env = append(env, key+"="+val)
	}
	if len(env) != 0 {
		logger.Infow(msg, "environment", env)
	}
}

// GetenvInt gets a variable from the environment, and returns as int, if can't, then uses default.
func GetenvInt(v string, def int) int {
	x := os.Getenv(v)
	if x == "" {
		return def
	}

	num, err := strconv.Atoi(x)
	if err != nil {
		return def
	}

	return num
}

// GetenvBool gets a variable from the environment, and returns as bool, if can't, then uses default.
func GetenvBool(v string, def bool) bool {
	x := os.Getenv(v)
	if x == "" {
		return def
	}

	return x[0] == 't' || x[0] == 'T' || x[0] == '1'
}

// CleanWindowsSocketPath mutates socket paths on windows only so they
// work well with the GRPC library.
// It converts e.g. C:\x\y.sock to /x/y.sock
// If you don't do this, it will confuse grpc-go's url.Parse call and surrounding logic.
// See https://github.com/grpc/grpc-go/blob/v1.71.0/clientconn.go#L1720-L1727
func CleanWindowsSocketPath(goos, orig string) (string, error) {
	if goos == "windows" {
		match := windowsPathRegex.FindStringSubmatch(orig)
		if match == nil {
			return "", fmt.Errorf("error cleaning socket path %s", orig)
		}
		if match[1] != "" && strings.ToLower(match[1]) != "c:" {
			return "", fmt.Errorf("we expect unix sockets on C: drive, not %s", match[1])
		}
		return strings.ReplaceAll(match[2], "\\", "/"), nil
	}
	return orig, nil
}
