package main

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	_ "go.viam.com/rdk/examples/mycomponent/component"
	"go.viam.com/rdk/module"

	"go.viam.com/utils"
)


var logger = NewLogger()

func NewLogger() (*zap.SugaredLogger) {
	cfg := zap.NewDevelopmentConfig()
	cfg.OutputPaths = []string{"/tmp/mod.log"}
	l, err := cfg.Build()
	if err != nil {
		return nil
	}
	return l.Sugar()
}

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) (err error) {
	if len(args) < 2 {
		return errors.New("need socket path as command line argument")
	}
	socketPath := args[1]

	myMod := module.NewModule(socketPath, logger)
	err = myMod.Start(ctx)
	defer myMod.Close()
	if err != nil {
		return err
	}
	<-ctx.Done()
	return nil
}
