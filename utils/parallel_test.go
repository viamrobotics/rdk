package utils

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.viam.com/test"
	gutils "go.viam.com/utils"
)

// This test may be flaky due to timing-based tests.
func TestRunInParallel(t *testing.T) {
	wait200ms := func(ctx context.Context) error {
		gutils.SelectContextOrWait(ctx, 200*time.Millisecond)
		return ctx.Err()
	}

	elapsed, err := RunInParallel(context.Background(), []SimpleFunc{wait200ms, wait200ms})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, elapsed, test.ShouldBeLessThan, 300*time.Millisecond)
	test.That(t, elapsed, test.ShouldBeGreaterThan, 180*time.Millisecond)

	errFunc := func(ctx context.Context) error {
		return errors.New("bad")
	}

	elapsed, err = RunInParallel(context.Background(), []SimpleFunc{wait200ms, wait200ms, errFunc})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, elapsed, test.ShouldBeLessThan, 50*time.Millisecond)

	panicFunc := func(ctx context.Context) error {
		panic(1)
	}

	_, err = RunInParallel(context.Background(), []SimpleFunc{panicFunc})
	test.That(t, err, test.ShouldNotBeNil)
}

func TestGroupWorkParallel(t *testing.T) {
	// Add one slice of ones with another slice of twos and see if the sum of the entire result is correct.
	N := 5001
	sliceA := make([]int, N)
	sliceB := make([]int, N)
	for i := 0; i < N; i++ {
		sliceA[i] = 1
		sliceB[i] = 2
	}
	var results []int
	err := GroupWorkParallel(
		context.Background(),
		N,
		func(numGroups int) {
			results = make([]int, numGroups)
		},
		func(groupNum, groupSize, from, to int) (MemberWorkFunc, GroupWorkDoneFunc) {
			mySum := 0
			return func(memberNum, workNum int) {
					a := sliceA[workNum]
					b := sliceB[workNum]
					mySum += a + b
				}, func() {
					results[groupNum] = mySum
				}
		},
	)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(results), test.ShouldEqual, ParallelFactor)
	total := 0
	for _, ans := range results {
		total += ans
	}
	test.That(t, total, test.ShouldEqual, 3*N)
}
