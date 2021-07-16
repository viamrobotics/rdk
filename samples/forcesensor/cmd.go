package main

import (
	"context"
	"fmt"

	"github.com/edaniels/golog"
	"go.viam.com/core/board"
	"go.viam.com/utils"
)

var logger = golog.NewDevelopmentLogger("force-sensor")

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) (err error) {
	cfg := board.Config{
		Model: "pi",
	}

	for i := 0; i < 5; i++ {
		cfg.Analogs = append(cfg.Analogs, board.AnalogConfig{
			Name: fmt.Sprintf("a%d", i),
			Pin:  fmt.Sprintf("%d", i),
		})
	}

	b, err := board.NewBoard(ctx, cfg, logger)
	fmt.Println(b, err)
	if err != nil {
		return err
	}
	defer b.Close()

	fmt.Println(cfg)

	return nil
}
