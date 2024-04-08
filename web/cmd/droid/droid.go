// Package main provides a server offering gRPC/REST/GUI APIs to control and monitor
// a robot.
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

var logger = logging.NewDebugLogger("robot_server")

// android harness uses this to stop the thread
func DroidStopHook() {
	server.ForceRestart = true
}

func MainEntry(configPath, writeablePath, osEnv string) {
	os.Args = append(os.Args, "-config", configPath)
	for _, envEntry := range strings.Split(osEnv, "\n") {
		entryParts := strings.SplitN(envEntry, "=", 2)
		os.Setenv(entryParts[0], entryParts[1])
	}
	utils.ContextualMain(server.RunServer, logger)
}
