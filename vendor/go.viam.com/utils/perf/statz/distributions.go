package statz

import (
	"context"

	"github.com/edaniels/golog"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

// Distribution contains hisogram buckets for metric of distribution type.
type Distribution struct {
	buckets []float64 // Buckets are the bucket endpoints
}

// LatencyDistribution is a basic latency distribution.
var LatencyDistribution = DistributionFromBounds(0, 5, 25, 50, 75, 100, 200, 400, 600, 800, 1000, 2000, 4000, 6000)

// DistributionFromBounds create distribution from a list of bounds. Must be incrementing and non-overlapping.
func DistributionFromBounds(bounds ...float64) Distribution {
	return Distribution{
		buckets: bounds,
	}
}

// Distribution0 is a float64 histogram metic. Good for latencies.
type Distribution0 struct {
	wrapper *ocDistributionWrapper
}

// Observe records an observation of the metric.
func (c *Distribution0) Observe(v float64) {
	c.wrapper.observe(context.Background(), labelsToStringSlice(), v)
}

// Distribution1 is a float64 histogram metic. Good for latencies.
type Distribution1[T1 labelContraint] struct {
	wrapper *ocDistributionWrapper
}

// Observe records an observation of the metric.
func (c *Distribution1[T1]) Observe(v float64, l1 T1) {
	c.wrapper.observe(context.Background(), labelsToStringSlice(l1), v)
}

// Distribution2 is a float64 histogram metic. Good for latencies.
type Distribution2[T1 labelContraint, T2 labelContraint] struct {
	wrapper *ocDistributionWrapper
}

// Observe records an observation of the metric.
func (c *Distribution2[T1, T2]) Observe(v float64, l1 T1, l2 T2) {
	c.wrapper.observe(context.Background(), labelsToStringSlice(l1, l2), v)
}

// Distribution3 is a float64 histogram metic. Good for latencies.
type Distribution3[T1 labelContraint, T2 labelContraint, T3 labelContraint] struct {
	wrapper *ocDistributionWrapper
}

// Observe records an observation of the metric.
func (c *Distribution3[T1, T2, T3]) Observe(v float64, l1 T1, l2 T2, l3 T3) {
	c.wrapper.observe(context.Background(), labelsToStringSlice(l1, l2, l3), v)
}

// Distribution4 is a float64 histogram metic. Good for latencies.
type Distribution4[T1 labelContraint, T2 labelContraint, T3 labelContraint, T4 labelContraint] struct {
	wrapper *ocDistributionWrapper
}

// Observe records an observation of the metric.
func (c *Distribution4[T1, T2, T3, T4]) Observe(v float64, l1 T1, l2 T2, l3 T3, l4 T4) {
	c.wrapper.observe(context.Background(), labelsToStringSlice(l1, l2, l3, l4), v)
}

///// internal

type ocDistributionWrapper struct {
	data    *opencensusStatsData
	measure *stats.Float64Measure
}

func (w *ocDistributionWrapper) observe(ctx context.Context, labels []string, value float64) {
	mutations := w.data.labelsToMutations(labels)
	if err := stats.RecordWithTags(ctx, mutations, w.measure.M(value)); err != nil {
		golog.Global().Errorf("faild to write metric %s", err)
	}
}

func createocDistributionWrapper(name string, distributions Distribution, cfg MetricConfig) *ocDistributionWrapper {
	measure := stats.Float64(name, cfg.Description, string(cfg.Unit))
	ocData := createAndRegisterOpenCensusMetric(name, measure, view.Distribution(distributions.buckets...), cfg)

	return &ocDistributionWrapper{
		data:    ocData,
		measure: measure,
	}
}
