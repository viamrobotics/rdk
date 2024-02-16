package utils

import (
	"context"
	"sync"

	goutils "go.viam.com/utils"
)

// TODO: When this struct is widely used and feature complete, move this to goutils instead of
// here. Until then, we cannot use this in any package imported by utils (e.g., the logging
// package) without introducing a circular import dependency.

// StoppableWorkersImpl is a collection of goroutines that can be stopped at a later time.
type StoppableWorkersImpl struct {
	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

// We want to be able to return a StoppableWorkers when we create it, but we can't make a copy of
// any struct that contains a sync.WaitGroup. So, make the one people will use be a pointer.
type StoppableWorkers *StoppableWorkersImpl

// NewStoppableWorkers runs the functions in separate goroutines. They can be stopped later.
func NewStoppableWorkers(funcs ...func(context.Context)) StoppableWorkers {
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	workers := &StoppableWorkersImpl{cancelCtx: cancelCtx, cancelFunc: cancelFunc}
	workers.activeBackgroundWorkers.Add(len(funcs))
	for _, f := range funcs {
		// In Go 1.21 and earlier, variables created in a loop were reused from one iteration to
		// the next. Make a "fresh" copy of it here so that, if we're on to the next iteration of
		// the loop before the goroutine starts up, it starts this function instead of the next
		// one. For details, see https://go.dev/blog/loopvar-preview
		f := f
		goutils.PanicCapturingGo(func() {
			defer workers.activeBackgroundWorkers.Done()
			f(cancelCtx)
		})
	}
	return workers
}

// Stop shuts down all the goroutines we started up.
func (sw *StoppableWorkersImpl) Stop() {
	sw.cancelFunc()
	sw.activeBackgroundWorkers.Wait()
}

// Context gets the context the workers are checking on. Using this function is expected to be
// rare: usually you shouldn't need to interact with the context directly.
func (sw *StoppableWorkersImpl) Context() context.Context {
	return sw.cancelCtx
}
