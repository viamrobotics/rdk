package utils

import (
	"os"
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
)

// GetResourceConfigurationTimeout calculates the resource configuration
// timeout (env variable value if set, DefaultResourceConfigurationTimeout
// otherwise).
func GetResourceConfigurationTimeout(logger logging.Logger) time.Duration {
	if timeoutVal := os.Getenv(ResourceConfigurationTimeoutEnvVar); timeoutVal != "" {
		timeout, err := time.ParseDuration(timeoutVal)
		if err != nil {
			logger.Warn("Failed to parse %s env var, falling back to default %v timeout",
				ResourceConfigurationTimeoutEnvVar, DefaultResourceConfigurationTimeout)
			return DefaultResourceConfigurationTimeout
		}
		return timeout
	}
	return DefaultResourceConfigurationTimeout
}

// GetModuleStartupTimeout calculates the module startup timeout
// (env variable value if set, DefaultModuleStartupTimeout otherwise).
func GetModuleStartupTimeout(logger logging.Logger) time.Duration {
	if timeoutVal := os.Getenv(ModuleStartupTimeoutEnvVar); timeoutVal != "" {
		timeout, err := time.ParseDuration(timeoutVal)
		if err != nil {
			logger.Warn("Failed to parse %s env var, falling back to default %v timeout",
				ModuleStartupTimeoutEnvVar, DefaultModuleStartupTimeout)
			return DefaultModuleStartupTimeout
		}
		return timeout
	}
	return DefaultModuleStartupTimeout
}
