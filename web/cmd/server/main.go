// Package main provides a server offering gRPC/REST/GUI APIs to control and monitor
// a robot.
package main

import (
	"go.viam.com/utils"

	// registers all components.
	_ "go.viam.com/rdk/components/arm/wrapper" // this is special
	_ "go.viam.com/rdk/components/register"
	"go.viam.com/rdk/logging"
	// registers all services.
	_ "go.viam.com/rdk/services/register"
	"go.viam.com/rdk/web/server"
)

var logger = logging.NewDebugLogger("entrypoint")

func main() {
	// Set up camera observer for hot-plug support (darwin only, no-op on other platforms).
	// See observer_darwin.go for details on why this must be called from main().
	cleanup := setupCameraObserver(logger)
	defer cleanup()

	utils.ContextualMain(server.RunServer, logger)
}
