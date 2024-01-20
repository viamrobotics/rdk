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
utils.ContextualMain(server.RunServer, logger)
//THIS WILL NOT PASS THE LINTER            fadfasdfasdf;sdakfjsadkflasjddafk;lsdjfsadkl;fjsdaklfjsadkaflsajdfklasdjfasdklfjsadflkasdjfl;kasdfjasdkl;f;jasdfas;kldfjasdlk;fjasdkflasjdafadslfjas
}
