package utils

import (
	"context"
	"errors"
	"fmt"
	"image"
	"math"
	"runtime"
	"sync"
	"time"

	"go.uber.org/multierr"
	"go.viam.com/utils"
)

// ParallelFactor controls the max level of parallelization. This might be useful
// to set in tests where too much parallelism actually slows tests down in
// aggregate.
var ParallelFactor = runtime.GOMAXPROCS(0)

func init() {
	if ParallelFactor <= 0 {
		ParallelFactor = 1
	}
	quarterProcs := float64(ParallelFactor) * .25
	if quarterProcs > 8 {
		ParallelFactor = int(quarterProcs)
	}
}

type (
	// BeforeParallelGroupWorkFunc executes before any work starts with the calculated group size.
	BeforeParallelGroupWorkFunc func(groupSize int)
	// MemberWorkFunc runs for each work item (member) of a group.
	MemberWorkFunc func(memberNum, workNum int)
	// GroupWorkDoneFunc runs when a single group's work is done; helpful for merge stages.
	GroupWorkDoneFunc func()
	// GroupWorkFunc runs to determine what work members should do, if any.
	GroupWorkFunc func(groupNum, groupSize, from, to int) (MemberWorkFunc, GroupWorkDoneFunc)
)

// GroupWorkParallel parallelizes the given size of work over multiple workers.
func GroupWorkParallel(ctx context.Context, totalSize int, before BeforeParallelGroupWorkFunc, groupWork GroupWorkFunc) error {
	extra := 0
	if totalSize > ParallelFactor {
		extra = (totalSize % ParallelFactor)
	}
	groupSize := int(math.Floor(float64(totalSize) / float64(ParallelFactor)))

	numGroups := ParallelFactor
	before(numGroups)

	var wait sync.WaitGroup
	wait.Add(numGroups)
	for groupNum := 0; groupNum < numGroups; groupNum++ {
		groupNumCopy := groupNum
		utils.PanicCapturingGo(func() {
			defer wait.Done()
			groupNum := groupNumCopy

			thisGroupSize := groupSize
			thisExtra := 0
			if groupNum == (numGroups - 1) {
				thisExtra = extra
				thisGroupSize += thisExtra
			}
			from := groupSize * groupNum
			to := (groupSize * (groupNum + 1)) + thisExtra
			memberWork, groupWorkDone := groupWork(groupNum, thisGroupSize, from, to)
			if memberWork != nil {
				memberNum := 0
				for workNum := from; workNum < to; workNum++ {
					memberWork(memberNum, workNum)
					memberNum++
				}
			}
			if groupWorkDone != nil {
				groupWorkDone()
			}
		})
	}
	wait.Wait()
	return nil
}

// ParallelForEachPixel loops through the image and calls f functions for each [x, y] position.
// The image is divided into N * N blocks, where N is the number of available processor threads. For each block a
// parallel Goroutine is started.
func ParallelForEachPixel(size image.Point, f func(x, y int)) {
	procs := runtime.GOMAXPROCS(0)
	var waitGroup sync.WaitGroup
	waitGroup.Add(procs * procs)
	for i := 0; i < procs; i++ {
		startX := i * int(math.Floor(float64(size.X)/float64(procs)))
		var endX int
		if i < procs-1 {
			endX = (i + 1) * int(math.Floor(float64(size.X)/float64(procs)))
		} else {
			endX = size.X
		}
		for j := 0; j < procs; j++ {
			startY := j * int(math.Floor(float64(size.Y)/float64(procs)))
			var endY int
			if j < procs-1 {
				endY = (j + 1) * int(math.Floor(float64(size.Y)/float64(procs)))
			} else {
				endY = size.Y
			}
			sX, eX, sY, eY := startX, endX, startY, endY
			utils.PanicCapturingGo(func() {
				defer waitGroup.Done()
				for x := sX; x < eX; x++ {
					for y := sY; y < eY; y++ {
						f(x, y)
					}
				}
			})
		}
	}
	waitGroup.Wait()
}

// SimpleFunc is for RunInParallel.
type SimpleFunc func(ctx context.Context) error

// RunInParallel runs all functions in parallel, return is elapsed time and an error.
func RunInParallel(ctx context.Context, fs []SimpleFunc) (time.Duration, error) {
	start := time.Now()
	ctx, cancel := context.WithCancel(ctx)

	var wg sync.WaitGroup

	var bigError error
	var bigErrorMutex sync.Mutex
	storeError := func(err error) {
		bigErrorMutex.Lock()
		defer bigErrorMutex.Unlock()
		if bigError == nil || !errors.Is(err, context.Canceled) {
			bigError = multierr.Combine(bigError, err)
		}
	}

	helper := func(f SimpleFunc) {
		defer func() {
			if thePanic := recover(); thePanic != nil {
				storeError(fmt.Errorf("got panic running something in parallel: %v", thePanic))
				cancel()
			}
			wg.Done()
		}()
		err := f(ctx)
		if err != nil {
			storeError(err)
			cancel()
		}
	}

	for _, f := range fs {
		wg.Add(1)
		go helper(f)
	}

	wg.Wait()
	return time.Since(start), bigError
}

// FloatFunc is for GetInParallel.
type FloatFunc func(ctx context.Context) (float64, error)

// GetInParallel runs all functions in parallel, return is elapsed time, a list of floats, and an error.
func GetInParallel(ctx context.Context, fs []FloatFunc) (time.Duration, []float64, error) {
	start := time.Now()
	ctx, cancel := context.WithCancel(ctx)

	var wg sync.WaitGroup

	var bigError error
	var bigErrorMutex sync.Mutex
	storeError := func(err error) {
		bigErrorMutex.Lock()
		defer bigErrorMutex.Unlock()
		if bigError == nil || !errors.Is(err, context.Canceled) {
			bigError = multierr.Combine(bigError, err)
		}
	}

	results := make([]float64, len(fs))

	helper := func(f FloatFunc, i int) {
		defer func() {
			if thePanic := recover(); thePanic != nil {
				storeError(fmt.Errorf("got panic getting something in parallel: %v", thePanic))
				cancel()
			}
			wg.Done()
		}()
		value, err := f(ctx)
		if err != nil {
			storeError(err)
			cancel()
		}
		results[i] = value
	}

	for i, f := range fs {
		wg.Add(1)
		go helper(f, i)
	}

	wg.Wait()
	return time.Since(start), results, bigError
}
