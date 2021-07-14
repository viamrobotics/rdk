// Package main provides a server offering gRPC/REST/GUI APIs to control and monitor
// a robot.
package main

import (
	"go.viam.com/utils"

	"go.viam.com/core/web/server"
	"github.com/edaniels/golog"
)

var logger = golog.NewDevelopmentLogger("robot_server")

func main() {
	utils.ContextualMain(server.RunServer, logger)
}
