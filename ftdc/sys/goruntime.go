package sys

import (
	"runtime/metrics"

	"go.viam.com/rdk/ftdc"
)

const (
	goCPUGC       = "/cpu/classes/gc/total:cpu-seconds"
	goCPUScavenge = "/cpu/classes/scavenge/total:cpu-seconds"
	goCPUUser     = "/cpu/classes/user:cpu-seconds"
	goCPUIdle     = "/cpu/classes/idle:cpu-seconds"
	goCPUTotal    = "/cpu/classes/total:cpu-seconds"
	goGCCycles    = "/gc/cycles/total:gc-cycles"
	goHeapAllocs  = "/gc/heap/allocs:bytes"
	goGoroutines  = "/sched/goroutines:goroutines"
	goMaxProcs    = "/sched/gomaxprocs:threads"
)

var goRuntimeMetricNames = []string{
	goCPUGC, goCPUScavenge, goCPUUser, goCPUIdle, goCPUTotal,
	goGCCycles, goHeapAllocs, goGoroutines, goMaxProcs,
}

// goCPUSecs are cumulative estimates that the runtime updates once per GC cycle. Compare them only
// with each other: they are not comparable with the kernel's CPU accounting.
type goCPUSecs struct {
	GC       float64
	Scavenge float64
	User     float64
	Idle     float64
	Total    float64
}

type goRuntimeStats struct {
	CPUSecs         goCPUSecs
	GCCycles        float64
	TotalAllocBytes float64
	Goroutines      float64
	GOMAXPROCS      float64
}

type goRuntimeStatser struct{}

// NewGoRuntimeStatser returns an ftdc statser for the Go runtime of the current process.
func NewGoRuntimeStatser() ftdc.Statser {
	return goRuntimeStatser{}
}

func (goRuntimeStatser) Stats() any {
	samples := make([]metrics.Sample, len(goRuntimeMetricNames))
	values := make(map[string]float64, len(samples))
	for i, name := range goRuntimeMetricNames {
		samples[i].Name = name
	}
	metrics.Read(samples)
	for _, sample := range samples {
		//nolint:exhaustive
		switch sample.Value.Kind() {
		case metrics.KindUint64:
			values[sample.Name] = float64(sample.Value.Uint64())
		case metrics.KindFloat64:
			values[sample.Name] = sample.Value.Float64()
		}
	}

	return goRuntimeStats{
		CPUSecs: goCPUSecs{
			GC:       values[goCPUGC],
			Scavenge: values[goCPUScavenge],
			User:     values[goCPUUser],
			Idle:     values[goCPUIdle],
			Total:    values[goCPUTotal],
		},
		GCCycles:        values[goGCCycles],
		TotalAllocBytes: values[goHeapAllocs],
		Goroutines:      values[goGoroutines],
		GOMAXPROCS:      values[goMaxProcs],
	}
}
