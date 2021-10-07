package utils

import (
	"context"
	"runtime"
	"sync"

	goutils "go.viam.com/utils"
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
		goutils.PanicCapturingGo(func() {
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
