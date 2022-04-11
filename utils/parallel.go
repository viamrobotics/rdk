package utils

import (
	"context"
	"image"
	"math"
	"runtime"
	"sync"

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
	MemberWorkFunc func(memberNum int, workNum int)
	// GroupWorkDoneFunc runs when a single group's work is done; helpful for merge stages.
	GroupWorkDoneFunc func()
	// GroupWorkFunc runs to determine what work members should do, if any.
	GroupWorkFunc func(groupNum, groupSize, from, to int) (MemberWorkFunc, GroupWorkDoneFunc)
)

// GroupWorkParallel parallelizes the given size of work over multiple workers.
func GroupWorkParallel(ctx context.Context, totalSize int, before BeforeParallelGroupWorkFunc, groupWork GroupWorkFunc) error {
	extra := 0
	if totalSize > ParallelFactor {
		extra = totalSize % ParallelFactor
	}
	groupSize := totalSize / ParallelFactor

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
func ParallelForEachPixel(size image.Point, f func(x int, y int)) {
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
