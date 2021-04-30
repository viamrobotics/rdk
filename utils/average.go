package utils

import "sync/atomic"

type RollingAverage struct {
	data []int64
	pos  int64
}

func NewRollingAverage(numSamples int) *RollingAverage {
	return &RollingAverage{data: make([]int64, numSamples), pos: 0}
}

func (ra *RollingAverage) NumSamples() int {
	return len(ra.data)
}

func (ra *RollingAverage) Add(x int) {
	atomic.StoreInt64(&ra.data[ra.pos], int64(x))
	atomic.AddInt64(&ra.pos, 1)
	atomic.CompareAndSwapInt64(&ra.pos, int64(len(ra.data)), 0)
}

func (ra *RollingAverage) Average() int {
	var sum int64 = 0

	for i := range ra.data {
		sum += atomic.LoadInt64(&ra.data[i])
	}

	return int(sum / int64(len(ra.data)))
}
