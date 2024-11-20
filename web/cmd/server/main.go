// Package main provides a server offering gRPC/REST/GUI APIs to control and monitor
// a robot.
package main

import (
	"log"
	"net/http"

	"go.viam.com/utils"

	// registers all components.
	_ "go.viam.com/rdk/components/register"
	"go.viam.com/rdk/logging"

	// registers all services.
	_ "net/http/pprof"

	_ "go.viam.com/rdk/services/register"
	"go.viam.com/rdk/web/server"
)

var logger = logging.NewDebugLogger("entrypoint")

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6061", nil))
	}()
	utils.ContextualMain(server.RunServer, logger)
}