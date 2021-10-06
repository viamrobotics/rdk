package utils

import (
	"context"
	"runtime"
	"sync"

	goutils "go.viam.com/utils"
)

var parallelFactor = runtime.GOMAXPROCS(0)

func init() {
	if parallelFactor <= 0 {
		parallelFactor = 1
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
	if totalSize > parallelFactor {
		extra = totalSize % parallelFactor
	}
	groupSize := totalSize / parallelFactor

	numGroups := parallelFactor
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
	// 37784*2000
	// 2000*
	wait.Wait()
	return nil
}
