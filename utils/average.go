package utils

import "sync/atomic"

// RollingAverage computes an average in a moving window
// of a certain size. It is goroutine-safe but should be
// used for statistical purposes only due to the use of
// atomics and not mutexes.
type RollingAverage struct {
	pos  int64
	data []int64
}

// NewRollingAverage returns a rolling average computed on the given
// window size.
func NewRollingAverage(windowSize int) *RollingAverage {
	return &RollingAverage{data: make([]int64, windowSize), pos: 0}
}

// NumSamples returns the number of samples currently collected.
func (ra *RollingAverage) NumSamples() int {
	return len(ra.data)
}

// Add adds the given value to the samples.
func (ra *RollingAverage) Add(x int) {
	atomic.StoreInt64(&ra.data[ra.pos], int64(x))
	atomic.AddInt64(&ra.pos, 1)
	atomic.CompareAndSwapInt64(&ra.pos, int64(len(ra.data)), 0)
}

// Average recomputes and returns the current rolling average.
func (ra *RollingAverage) Average() int {
	var sum int64
	for i := range ra.data {
		sum += atomic.LoadInt64(&ra.data[i])
	}
	return int(sum / int64(len(ra.data)))
}
