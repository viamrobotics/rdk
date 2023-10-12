// Package main provides a server offering gRPC/REST/GUI APIs to control and monitor
// a robot.
package main

import (
	"go.viam.com/utils"

	// registers all components.
	_ "go.viam.com/rdk/components/register"
	"go.viam.com/rdk/logging"

	// registers all services.
	_ "go.viam.com/rdk/services/register"
	"go.viam.com/rdk/web/server"
)

var logger = logging.NewDebugLogger("robot_server")

func main() {
	utils.ContextualMain(server.RunServer, logger.AsZap())
}
