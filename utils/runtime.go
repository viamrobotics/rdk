package utils

import (
	"context"

	utils "go.viam.com/utils"
)

// ContextualMain calls a main entry point function with a cancellable
// context via SIGTERM. This should be called once per process so as
// to not clobber the signals from Notify.
func ContextualMain[L utils.ILogger](main func(ctx context.Context, args []string, logger L) error, logger L) {
	utils.ContextualMain(main, logger)
}
