package main

import (
	"context"
	"flag"

	"go.viam.com/utils"

	"github.com/edaniels/golog"
)

var logger = golog.NewDevelopmentLogger("dishcare")

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) (err error) {
	flag.Parse()
	return nil
}
