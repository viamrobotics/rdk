// Package droid is the entrypoint for gomobile.
package droid

import (
	"os"
	"strings"

	"go.viam.com/utils"

	// registers all components.
	_ "go.viam.com/rdk/components/register"
	"go.viam.com/rdk/logging"
	// registers all services.
	_ "go.viam.com/rdk/services/register"
	"go.viam.com/rdk/web/server"
)

var logger = logging.NewDebugLogger("rdk")

// DroidStopHook used by android harness to stop the RDK.
func DroidStopHook() { //nolint:revive
	server.ForceRestart = true
}

// MainEntry is called by our android app to start the RDK.
func MainEntry(configPath, writeablePath, osEnv string) {
	os.Args = append(os.Args, "-config", configPath)
	for _, envEntry := range strings.Split(osEnv, "\n") {
		entryParts := strings.SplitN(envEntry, "=", 2)
		os.Setenv(entryParts[0], entryParts[1]) //nolint:errcheck,gosec
	}
	utils.ContextualMain(server.RunServer, logger)
}
