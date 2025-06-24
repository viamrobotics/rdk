package statz

import (
	"context"

	"github.com/edaniels/golog"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

// Gauge0 is an int64 gauge type with no metric labels.
type Gauge0 struct {
	wrapper *ocGaugeWrapper
}

// Set sets gauge to X.
func (c *Gauge0) Set(to int64) {
	c.wrapper.set(context.Background(), labelsToStringSlice(), to)
}

// Gauge1 is an int64 gauge type with 1 metric label.
type Gauge1[T1 labelContraint] struct {
	wrapper *ocGaugeWrapper
}

// Set sets gauge to X.
func (c *Gauge1[T1]) Set(v1 T1, to int64) {
	c.wrapper.set(context.Background(), labelsToStringSlice(v1), to)
}

// Gauge2 is an int64 gauge type with 2 metric label.
type Gauge2[T1 labelContraint, T2 labelContraint] struct {
	wrapper *ocGaugeWrapper
}

// Set sets gauge to X.
func (c *Gauge2[T1, T2]) Set(v1 T1, v2 T2, to int64) {
	c.wrapper.set(context.Background(), labelsToStringSlice(v1, v2), to)
}

// Gauge3 is an int64 gauge type with 3 metric label.
type Gauge3[T1 labelContraint, T2 labelContraint, T3 labelContraint] struct {
	wrapper *ocGaugeWrapper
}

// Set sets gauge to X.
func (c *Gauge3[T1, T2, T3]) Set(v1 T1, v2 T2, v3 T3, to int64) {
	c.wrapper.set(context.Background(), labelsToStringSlice(v1, v2, v3), to)
}

// Gauge4 is an int64 gauge type with 4 metric label.
type Gauge4[T1 labelContraint, T2 labelContraint, T3 labelContraint, T4 labelContraint] struct {
	wrapper *ocGaugeWrapper
}

// Set sets gauge to X.
func (c *Gauge4[T1, T2, T3, T4]) Set(v1 T1, v2 T2, v3 T3, v4 T4, to int64) {
	c.wrapper.set(context.Background(), labelsToStringSlice(v1, v2, v3, v4), to)
}

///// internal

func createGaugeWrapper(name string, cfg MetricConfig) *ocGaugeWrapper {
	measure := stats.Int64(name, cfg.Description, string(cfg.Unit))
	ocData := createAndRegisterOpenCensusMetric(name, measure, view.LastValue(), cfg)

	return &ocGaugeWrapper{
		data:    ocData,
		measure: measure,
	}
}

type ocGaugeWrapper struct {
	data    *opencensusStatsData
	measure *stats.Int64Measure
}

func (w *ocGaugeWrapper) set(ctx context.Context, labels []string, to int64) {
	mutations := w.data.labelsToMutations(labels)
	if err := stats.RecordWithTags(ctx, mutations, w.measure.M(to)); err != nil {
		golog.Global().Errorf("faild to write metric %s", err)
	}
}
