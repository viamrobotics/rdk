// Package main provides a server offering gRPC/REST/GUI APIs to control and monitor
// a robot.
package main

import (
	"github.com/edaniels/golog"
	"go.viam.com/utils"

	// registers all components.
	_ "go.viam.com/rdk/component/register"

	// registers all services.
	_ "go.viam.com/rdk/services/register"
	"go.viam.com/rdk/web/server"
)

var logger = golog.NewDevelopmentLogger("robot_server")

func main() {
	utils.ContextualMain(server.RunServer, logger)
}
