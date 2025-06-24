package utils

import (
	"context"
	"sync"
	"time"
)

// StoppableWorkers is a collection of goroutines that can be stopped at a
// later time.
type StoppableWorkers struct {
	// Use a `sync.RWMutex` instead of a `sync.Mutex` so that additions of new
	// workers do not lock with each other in any way. We want
	// as-fast-as-possible worker addition.
	mu         sync.RWMutex
	ctx        context.Context
	cancelFunc func()

	workers sync.WaitGroup
}

// NewStoppableWorkers creates a new StoppableWorkers instance. The instance's
// context will be derived from passed in context.
func NewStoppableWorkers(ctx context.Context) *StoppableWorkers {
	ctx, cancelFunc := context.WithCancel(ctx)
	return &StoppableWorkers{ctx: ctx, cancelFunc: cancelFunc}
}

// NewBackgroundStoppableWorkers creates a new StoppableWorkers instance. The
// instance's context will be derived from `context.Background()`. The passed
// in workers will be `Add`ed. Workers:
//
//   - MUST respond appropriately to errors on the context parameter.
//
// Any `panic`s from workers will be `recover`ed and logged.
func NewBackgroundStoppableWorkers(workers ...func(context.Context)) *StoppableWorkers {
	ctx, cancelFunc := context.WithCancel(context.Background())
	sw := &StoppableWorkers{ctx: ctx, cancelFunc: cancelFunc}
	for _, worker := range workers {
		sw.Add(worker)
	}
	return sw
}

// NewStoppableWorkerWithTicker creates a `StoppableWorkers` object with a single worker that gets
// called every `tickRate`. Calls to the input `worker` function are serialized. I.e: a slow "work"
// iteration will just slow down when the next one is called.
func NewStoppableWorkerWithTicker(tickRate time.Duration, workFn func(context.Context)) *StoppableWorkers {
	ctx, cancelFunc := context.WithCancel(context.Background())
	sw := &StoppableWorkers{ctx: ctx, cancelFunc: cancelFunc}
	sw.workers.Add(1)
	PanicCapturingGo(func() {
		defer sw.workers.Done()

		timer := time.NewTicker(tickRate)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			select {
			case <-timer.C:
				workFn(ctx)
			case <-ctx.Done():
				return
			}
		}
	})

	return sw
}

// Add starts up a goroutine for the passed-in function. Workers must respond appropriately to
// errors on the context parameter.
//
// The worker will not be added if the StoppableWorkers instance has already
// been stopped. Any `panic`s from workers will be `recover`ed and logged.
func (sw *StoppableWorkers) Add(worker func(context.Context)) {
	// Acquire the read lock to allow concurrent worker addition. The Stop method will
	// write-lock. `Add` is guaranteed to either:
	// - Observe the context is canceled -- the worker will not be run, nor will the `workers`
	//   WaitGroup be incremented
	// - Observe the context is not canceled atomically with incrementing the `workers`
	//   WaitGroup. `Stop` is guaranteed to wait for this new worker to complete before returning.
	sw.mu.RLock()
	if sw.ctx.Err() != nil {
		sw.mu.RUnlock()
		return
	}
	sw.workers.Add(1)
	sw.mu.RUnlock()

	PanicCapturingGo(func() {
		defer sw.workers.Done()
		worker(sw.ctx)
	})
}

// Stop idempotently shuts down all the goroutines we started up.
func (sw *StoppableWorkers) Stop() {
	// Call `cancelFunc` with the write lock that competes with "readers" that can add workers. This
	// guarantees `Add` worker calls that start a goroutine have incremented the `workers` WaitGroup
	// prior to `Stop` calling `Wait`.
	sw.mu.Lock()
	if sw.ctx.Err() != nil {
		sw.mu.Unlock()
		return
	}
	sw.cancelFunc()
	// Make sure to unlock the mutex before waiting for background goroutines to shut down! That
	// way, any goroutine that was waiting on this lock (e.g., it was trying to spawn another
	// background worker) won't deadlock, and we'll shut down properly.
	sw.mu.Unlock()

	sw.workers.Wait()
}

// Context gets the context of the StoppableWorkers instance. Using this
// function is expected to be rare: usually you shouldn't need to interact with
// the context directly.
func (sw *StoppableWorkers) Context() context.Context {
	return sw.ctx
}
