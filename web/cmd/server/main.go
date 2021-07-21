// Package main provides a server offering gRPC/REST/GUI APIs to control and monitor
// a robot.
package main

import (
	"go.viam.com/utils"

	"github.com/edaniels/golog"

	"go.viam.com/core/web/server"
)

var logger = golog.NewDevelopmentLogger("robot_server")

func main() {
	utils.ContextualMain(server.RunServer, logger)
}
