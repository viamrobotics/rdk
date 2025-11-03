package utils

import (
	"context"

	utils "go.viam.com/utils"
)

// ContextualMain calls a main entry point function with a cancellable
// context via SIGTERM and SIGPIPE. This should be called once per process so as
// to not clobber the signals from Notify.
//
// Deprecated: For module usage, use module.ModularMain instead.
func ContextualMain[L utils.ILogger](main func(ctx context.Context, args []string, logger L) error, logger L) {
	// Since the intended use is with modules, we should not ignore SIGPIPEs.
	utils.ContextualMainWithSIGPIPE(main, logger)
}
