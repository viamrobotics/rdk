package statz

import (
	"context"

	"github.com/edaniels/golog"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

// Counter0 is a incremental int64 counter type with no metric labels.
type Counter0 struct {
	wrapper *ocCounterWrapper
}

// Inc increments counter by 1.
func (c *Counter0) Inc() {
	c.IncBy(1)
}

// IncBy increments counter by X.
func (c *Counter0) IncBy(by int64) {
	c.wrapper.incBy(context.Background(), labelsToStringSlice(), by)
}

// Counter1 is a incremental int64 counter type with 1 metric label.
type Counter1[T1 labelContraint] struct {
	wrapper *ocCounterWrapper
}

// Inc increments counter by 1.
func (c *Counter1[T1]) Inc(v1 T1) {
	c.IncBy(v1, 1)
}

// IncBy increments counter by X.
func (c *Counter1[T1]) IncBy(v1 T1, by int64) {
	c.wrapper.incBy(context.Background(), labelsToStringSlice(v1), by)
}

// Counter2 is a incremental int64 counter type with 2 metric label.
type Counter2[T1 labelContraint, T2 labelContraint] struct {
	wrapper *ocCounterWrapper
}

// Inc increments counter by 1.
func (c *Counter2[T1, T2]) Inc(v1 T1, v2 T2) {
	c.IncBy(v1, v2, 1)
}

// IncBy increments counter by X.
func (c *Counter2[T1, T2]) IncBy(v1 T1, v2 T2, by int64) {
	c.wrapper.incBy(context.Background(), labelsToStringSlice(v1, v2), by)
}

// Counter3 is a incremental int64 counter type with 3 metric label.
type Counter3[T1 labelContraint, T2 labelContraint, T3 labelContraint] struct {
	wrapper *ocCounterWrapper
}

// Inc increments counter by 1.
func (c *Counter3[T1, T2, T3]) Inc(v1 T1, v2 T2, v3 T3) {
	c.IncBy(v1, v2, v3, 1)
}

// IncBy increments counter by X.
func (c *Counter3[T1, T2, T3]) IncBy(v1 T1, v2 T2, v3 T3, by int64) {
	c.wrapper.incBy(context.Background(), labelsToStringSlice(v1, v2, v3), by)
}

// Counter4 is a incremental int64 counter type with 4 metric label.
type Counter4[T1 labelContraint, T2 labelContraint, T3 labelContraint, T4 labelContraint] struct {
	wrapper *ocCounterWrapper
}

// Inc increments counter by 1.
func (c *Counter4[T1, T2, T3, T4]) Inc(v1 T1, v2 T2, v3 T3, v4 T4) {
	c.IncBy(v1, v2, v3, v4, 1)
}

// IncBy increments counter by X.
func (c *Counter4[T1, T2, T3, T4]) IncBy(v1 T1, v2 T2, v3 T3, v4 T4, by int64) {
	c.wrapper.incBy(context.Background(), labelsToStringSlice(v1, v2, v3, v4), by)
}

// Counter5 is a incremental int64 counter type with 5 metric label.
type Counter5[T1 labelContraint, T2 labelContraint, T3 labelContraint, T4 labelContraint, T5 labelContraint] struct {
	wrapper *ocCounterWrapper
}

// Inc increments counter by 1.
func (c *Counter5[T1, T2, T3, T4, T5]) Inc(v1 T1, v2 T2, v3 T3, v4 T4, v5 T5) {
	c.IncBy(v1, v2, v3, v4, v5, 1)
}

// IncBy increments counter by X.
func (c *Counter5[T1, T2, T3, T4, T5]) IncBy(v1 T1, v2 T2, v3 T3, v4 T4, v5 T5, by int64) {
	c.wrapper.incBy(context.Background(), labelsToStringSlice(v1, v2, v3, v4, v5), by)
}

///// internal

type ocCounterWrapper struct {
	data    *opencensusStatsData
	measure *stats.Int64Measure
}

func (w *ocCounterWrapper) incBy(ctx context.Context, labels []string, incBy int64) {
	mutations := w.data.labelsToMutations(labels)
	for i := int64(0); i < incBy; i++ {
		if err := stats.RecordWithTags(ctx, mutations, w.measure.M(1)); err != nil {
			golog.Global().Errorf("faild to write metric %s", err)
		}
	}
}

func createCounterWrapper(name string, cfg MetricConfig) *ocCounterWrapper {
	measure := stats.Int64(name, cfg.Description, string(cfg.Unit))
	ocData := createAndRegisterOpenCensusMetric(name, measure, view.Count(), cfg)

	return &ocCounterWrapper{
		data:    ocData,
		measure: measure,
	}
}
