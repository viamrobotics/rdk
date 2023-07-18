package utils

import (
	"os"
	"time"

	"github.com/edaniels/golog"
)

const (
	defaultResourceConfigurationTimeout = time.Minute

	// ResourceConfigurationTimeoutEnvVar is the environment variable that can
	// be set to override defaultResourceConfigurationTimeout as the duration
	// that resources and modules are allowed to (re)configure and startup
	// respectively.
	ResourceConfigurationTimeoutEnvVar = "VIAM_RESOURCE_CONFIGURATION_TIMEOUT"
)

// GetResourceConfigurationTimeout calculates the resource configuration
// timeout (env variable value if set, defaultResourceConfigurationTimeout
// otherwise).
func GetResourceConfigurationTimeout(logger golog.Logger) time.Duration {
	if timeoutVal := os.Getenv(ResourceConfigurationTimeoutEnvVar); timeoutVal != "" {
		timeout, err := time.ParseDuration(timeoutVal)
		if err != nil {
			logger.Warn("Failed to parse %s env var, falling back to default 1 minute timeout",
				ResourceConfigurationTimeoutEnvVar)
			return defaultResourceConfigurationTimeout
		}
		return timeout
	}
	return defaultResourceConfigurationTimeout
}
