package utils

import (
	"context"

	utils "go.viam.com/utils"
)

func ContextualMain[L utils.ILogger](main func(ctx context.Context, args []string, logger L) error, logger L) {
	utils.ContextualMain(main, logger)
}
