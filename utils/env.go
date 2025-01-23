package utils

import (
	"os"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"

	"go.viam.com/rdk/logging"
)

const (
	// DefaultResourceConfigurationTimeout is the default resource configuration
	// timeout.
	DefaultResourceConfigurationTimeout = time.Minute

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

	// MachineIDEnvVar is the environment variable that contains the machine ID of the machine.
	MachineIDEnvVar = "VIAM_MACHINE_ID"

	// MachinePartIDEnvVar is the environment variable that contains the machine part ID of the machine.
	MachinePartIDEnvVar = "VIAM_MACHINE_PART_ID"

	// LocationIDEnvVar is the environment variable that contains the location ID of the machine.
	LocationIDEnvVar = "VIAM_LOCATION_ID"

	// PrimaryOrgIDEnvVar is the environment variable that contains the primary org ID of the machine.
	PrimaryOrgIDEnvVar = "VIAM_PRIMARY_ORG_ID"
)

// EnvTrueValues contains strings that we interpret as boolean true in env vars.
var EnvTrueValues = []string{"true", "yes", "1", "TRUE", "YES"}

// TCPRegex tests whether a module address is TCP (vs unix sockets). See also ViamTCPSockets().
var TCPRegex = regexp.MustCompile(`:\d+$`)

// GetResourceConfigurationTimeout calculates the resource configuration
// timeout (env variable value if set, DefaultResourceConfigurationTimeout
// otherwise).
func GetResourceConfigurationTimeout(logger logging.Logger) time.Duration {
	return timeoutHelper(DefaultResourceConfigurationTimeout, ResourceConfigurationTimeoutEnvVar, logger)
}

// GetModuleStartupTimeout calculates the module startup timeout
// (env variable value if set, DefaultModuleStartupTimeout otherwise).
func GetModuleStartupTimeout(logger logging.Logger) time.Duration {
	return timeoutHelper(DefaultModuleStartupTimeout, ModuleStartupTimeoutEnvVar, logger)
}

func timeoutHelper(defaultTimeout time.Duration, timeoutEnvVar string, logger logging.Logger) time.Duration {
	if timeoutVal := os.Getenv(timeoutEnvVar); timeoutVal != "" {
		timeout, err := time.ParseDuration(timeoutVal)
		if err != nil {
			logger.Warn("Failed to parse %s env var, falling back to default %v timeout",
				timeoutEnvVar, defaultTimeout)
			return defaultTimeout
		}
		return timeout
	}
	return defaultTimeout
}

// PlatformHomeDir wraps Getenv("HOME"), except on android, where it returns the app cache directory.
func PlatformHomeDir() string {
	if runtime.GOOS == "android" {
		return AndroidFilesDir
	}
	if runtime.GOOS == "windows" {
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

// ViamTCPSockets returns true if an env is set or if the platform requires it.
func ViamTCPSockets() bool {
	// note: unix sockets have been supported on windows for a while, but go-grpc does not support them.
	// 2017 support announcement: https://devblogs.microsoft.com/commandline/af_unix-comes-to-windows/
	// go grpc client bug on win: https://github.com/dotnet/aspnetcore/issues/47043
	return runtime.GOOS == "windows" ||
		slices.Contains(EnvTrueValues, os.Getenv("VIAM_TCP_SOCKETS"))
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
