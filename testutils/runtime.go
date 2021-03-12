package testutils

import (
	"context"
	"os"
	"syscall"

	"go.viam.com/robotcore/utils"
)

// ContextualMainExecution reflects the execution of a main function
// that can have its lifecycel partially controlled.
type ContextualMainExecution struct {
	Ready      <-chan struct{}
	Done       <-chan error
	Stop       func()
	QuitSignal func() // reflects syscall.SIGQUIT
}

// ContextualMain calls a main entry point function with a cancellable
// context via the returned execution struct. The main function is run
// in a separate goroutine.
func ContextualMain(main func(ctx context.Context, args []string) error, args []string) ContextualMainExecution {
	return contextualMain(main, args)
}

func contextualMain(main func(ctx context.Context, args []string) error, args []string) ContextualMainExecution {
	ctx, stop := context.WithCancel(context.Background())
	quitC := make(chan os.Signal, 1)
	ctx = utils.ContextWithQuitSignal(ctx, quitC)
	readyC := make(chan struct{}, 1)
	ctx = utils.ContextWithReadyFunc(ctx, readyC)
	readyF := utils.ContextMainReadyFunc(ctx)
	doneC := make(chan error, 1)

	go func() {
		// if main is not a daemon like function or does not error out, just be "ready"
		// after execution is complete.
		defer readyF()
		doneC <- main(ctx, append([]string{"main"}, args...))
	}()
	return ContextualMainExecution{
		Ready: readyC,
		Done:  doneC,
		Stop:  stop,
		QuitSignal: func() {
			quitC <- syscall.SIGQUIT
		},
	}
}
