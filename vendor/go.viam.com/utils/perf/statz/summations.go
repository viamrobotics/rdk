package statz

import (
	"context"

	"github.com/edaniels/golog"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

// Summation0 is a incremental int64 summation type with no metric labels.
type Summation0 struct {
	wrapper *ocSummationWrapper
}

// Inc increments summation by 1.
func (c *Summation0) Inc() {
	c.IncBy(1)
}

// IncBy increments summation by X.
func (c *Summation0) IncBy(by int64) {
	c.wrapper.incBy(context.Background(), labelsToStringSlice(), by)
}

// Summation1 is a incremental int64 summation type with 1 metric label.
type Summation1[T1 labelContraint] struct {
	wrapper *ocSummationWrapper
}

// Inc increments summation by 1.
func (c *Summation1[T1]) Inc(v1 T1) {
	c.IncBy(v1, 1)
}

// IncBy increments summation by X.
func (c *Summation1[T1]) IncBy(v1 T1, by int64) {
	c.wrapper.incBy(context.Background(), labelsToStringSlice(v1), by)
}

// Summation2 is a incremental int64 summation type with 2 metric label.
type Summation2[T1 labelContraint, T2 labelContraint] struct {
	wrapper *ocSummationWrapper
}

// Inc increments summation by 1.
func (c *Summation2[T1, T2]) Inc(v1 T1, v2 T2) {
	c.IncBy(v1, v2, 1)
}

// IncBy increments summation by X.
func (c *Summation2[T1, T2]) IncBy(v1 T1, v2 T2, by int64) {
	c.wrapper.incBy(context.Background(), labelsToStringSlice(v1, v2), by)
}

// Summation3 is a incremental int64 summation type with 3 metric label.
type Summation3[T1 labelContraint, T2 labelContraint, T3 labelContraint] struct {
	wrapper *ocSummationWrapper
}

// Inc increments summation by 1.
func (c *Summation3[T1, T2, T3]) Inc(v1 T1, v2 T2, v3 T3) {
	c.IncBy(v1, v2, v3, 1)
}

// IncBy increments summation by X.
func (c *Summation3[T1, T2, T3]) IncBy(v1 T1, v2 T2, v3 T3, by int64) {
	c.wrapper.incBy(context.Background(), labelsToStringSlice(v1, v2, v3), by)
}

// Summation4 is a incremental int64 summation type with 4 metric label.
type Summation4[T1 labelContraint, T2 labelContraint, T3 labelContraint, T4 labelContraint] struct {
	wrapper *ocSummationWrapper
}

// Inc increments summation by 1.
func (c *Summation4[T1, T2, T3, T4]) Inc(v1 T1, v2 T2, v3 T3, v4 T4) {
	c.IncBy(v1, v2, v3, v4, 1)
}

// IncBy increments summation by X.
func (c *Summation4[T1, T2, T3, T4]) IncBy(v1 T1, v2 T2, v3 T3, v4 T4, by int64) {
	c.wrapper.incBy(context.Background(), labelsToStringSlice(v1, v2, v3, v4), by)
}

///// internal

type ocSummationWrapper struct {
	data    *opencensusStatsData
	measure *stats.Int64Measure
}

func (w *ocSummationWrapper) incBy(ctx context.Context, labels []string, incBy int64) {
	mutations := w.data.labelsToMutations(labels)
	if err := stats.RecordWithTags(ctx, mutations, w.measure.M(incBy)); err != nil {
		golog.Global().Errorf("faild to write metric %s", err)
	}
}

func createSummationWrapper(name string, cfg MetricConfig) *ocSummationWrapper {
	measure := stats.Int64(name, cfg.Description, string(cfg.Unit))
	ocData := createAndRegisterOpenCensusMetric(name, measure, view.Sum(), cfg)

	return &ocSummationWrapper{
		data:    ocData,
		measure: measure,
	}
}
