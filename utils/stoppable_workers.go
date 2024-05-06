package utils

import (
	"context"
	"sync"

	goutils "go.viam.com/utils"
)

// TODO: When this struct is widely used and feature complete, move this to goutils instead of
// here. Until then, we cannot use this in any package imported by utils (e.g., the logging
// package) without introducing a circular import dependency.

// StoppableWorkers is a collection of goroutines that can be stopped at a later time.
type StoppableWorkers interface {
	AddWorkers(...func(context.Context))
	Stop()
	Context() context.Context
}

// stoppableWorkersImpl is the implementation of StoppableWorkers. The linter will complain if you
// try to make a copy of something that contains a sync.WaitGroup (and returning a value at the end
// of NewStoppableWorkers() would make a copy of it), so we do everything through the
// StoppableWorkers interface to avoid making copies (since interfaces do everything by pointer).
type stoppableWorkersImpl struct {
	mu                      sync.Mutex
	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

// NewStoppableWorkers runs the functions in separate goroutines. They can be stopped later.
func NewStoppableWorkers(funcs ...func(context.Context)) StoppableWorkers {
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	workers := &stoppableWorkersImpl{cancelCtx: cancelCtx, cancelFunc: cancelFunc}
	workers.AddWorkers(funcs...)
	return workers
}

// AddWorkers starts up additional goroutines for each function passed in. If you call this after
// calling Stop(), it will return immediately without starting any new goroutines.
func (sw *stoppableWorkersImpl) AddWorkers(funcs ...func(context.Context)) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if sw.cancelCtx.Err() != nil { // We've already stopped everything.
		return
	}

	sw.activeBackgroundWorkers.Add(len(funcs))
	for _, f := range funcs {
		// In Go 1.21 and earlier, variables created in a loop were reused from one iteration to
		// the next. Make a "fresh" copy of it here so that, if we're on to the next iteration of
		// the loop before the goroutine starts up, it starts this function instead of the next
		// one. For details, see https://go.dev/blog/loopvar-preview
		f := f
		goutils.PanicCapturingGo(func() {
			defer sw.activeBackgroundWorkers.Done()
			f(sw.cancelCtx)
		})
	}
}

// Stop shuts down all the goroutines we started up.
func (sw *stoppableWorkersImpl) Stop() {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	sw.cancelFunc()
	sw.activeBackgroundWorkers.Wait()
}

// Context gets the context the workers are checking on. Using this function is expected to be
// rare: usually you shouldn't need to interact with the context directly.
func (sw *stoppableWorkersImpl) Context() context.Context {
	return sw.cancelCtx
}
