package utils

type RollingAverage struct {
	data []int
	pos  int
}

func NewRollingAverage(numSamples int) *RollingAverage {
	return &RollingAverage{data: make([]int, numSamples), pos: 0}
}

func (ra *RollingAverage) NumSamples() int {
	return len(ra.data)
}

func (ra *RollingAverage) Add(x int) {
	ra.data[ra.pos] = x
	ra.pos++
	if ra.pos >= len(ra.data) {
		ra.pos = 0
	}
}

func (ra *RollingAverage) Average() int {
	sum := 0

	for _, d := range ra.data {
		sum += d
	}

	return sum / len(ra.data)
}
