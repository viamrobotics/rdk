package utils

import (
	"context"
	"sync"

	goutils "go.viam.com/utils"
)

// TODO: Move this to goutils instead of here.

// TODO: Come up with a better name.
// StoppableWorkers is a collection of goroutines that can be stopped at a later time.
type StoppableWorkers struct {
	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

// NewStoppableWorkers runs the functions in separate goroutines. They can be stopped later.
func NewStoppableWorkers(funcs... func(context.Context)) StoppableWorkers {
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	workers := StoppableWorkers{cancelCtx: cancelCtx, cancelFunc: cancelFunc}
	workers.activeBackgroundWorkers.Add(len(funcs))
	for _, f := range funcs {
		goutils.PanicCapturingGo(func() {
			defer workers.activeBackgroundWorkers.Done()
			f(cancelCtx)
		})
	}
	return workers
}

// Stop shuts down all the goroutines we started up.
func (sw *StoppableWorkers) Stop() {
	sw.cancelFunc()
	sw.activeBackgroundWorkers.Wait()
}

// Stop with trigger stops the goroutines by also calling the trigger function. It's for unusual
// situations where you need to do something extra for the goroutine to wake up, like
// https://github.com/viamrobotics/rdk/pull/3577/commits/6506580b4661fa5a016698270d4e0359f947c22d
func (sw *StoppableWorkers) StopWithTrigger(trigger func()) {
	sw.cancelFunc()
	trigger()
	sw.activeBackgroundWorkers.Wait()
}


