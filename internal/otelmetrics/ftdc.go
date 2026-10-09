package otelmetrics

import (
	"context"
	"strings"
	"sync"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// FTDCPoint is one data point of a metric: Value for a sum or gauge, Count and Sum for a histogram.
type FTDCPoint struct {
	Attrs attribute.Set
	Value float64
	Count uint64
	Sum   float64

	histogram bool
}

// FTDCKeyFunc writes one data point to FTDC by calling add, which sums the values given for a key.
type FTDCKeyFunc func(p FTDCPoint, add func(key string, value float64))

type ftdcStatser struct {
	reader *sdkmetric.ManualReader

	mu   sync.Mutex
	keys map[string]FTDCKeyFunc
}

func newFTDCStatser() *ftdcStatser {
	return &ftdcStatser{reader: sdkmetric.NewManualReader(), keys: map[string]FTDCKeyFunc{}}
}

func (s *ftdcStatser) mapMetric(metricName string, fn FTDCKeyFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys[metricName] = fn
}

// Stats collects metrics into a map of fully qualified FTDC keys. An unmapped metric is keyed by its
// name and attribute values; a histogram writes only its count and sum. otelgrpc reports only
// client calls, which have no FTDC keys.
func (s *ftdcStatser) Stats() any {
	ret := map[string]float64{}
	var rm metricdata.ResourceMetrics
	if err := s.reader.Collect(context.Background(), &rm); err != nil {
		return ret
	}
	add := func(key string, value float64) { ret[key] += value }

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sm := range rm.ScopeMetrics {
		if sm.Scope.Name == otelgrpc.ScopeName {
			continue
		}
		for _, m := range sm.Metrics {
			fn := s.keys[m.Name]
			for _, p := range points(m.Data) {
				switch {
				case fn != nil:
					fn(p, add)
				case p.histogram:
					key := genericName(m.Name, p.Attrs)
					add(key+".count", float64(p.Count))
					add(key+".sum", p.Sum)
				default:
					add(genericName(m.Name, p.Attrs), p.Value)
				}
			}
		}
	}
	return ret
}

func points(data metricdata.Aggregation) []FTDCPoint {
	switch data := data.(type) {
	case metricdata.Gauge[int64]:
		return valuePoints(data.DataPoints)
	case metricdata.Gauge[float64]:
		return valuePoints(data.DataPoints)
	case metricdata.Sum[int64]:
		return valuePoints(data.DataPoints)
	case metricdata.Sum[float64]:
		return valuePoints(data.DataPoints)
	case metricdata.Histogram[int64]:
		return histogramPoints(data.DataPoints)
	case metricdata.Histogram[float64]:
		return histogramPoints(data.DataPoints)
	default:
		return nil
	}
}

func valuePoints[N int64 | float64](dps []metricdata.DataPoint[N]) []FTDCPoint {
	ret := make([]FTDCPoint, 0, len(dps))
	for _, dp := range dps {
		ret = append(ret, FTDCPoint{Attrs: dp.Attributes, Value: float64(dp.Value)})
	}
	return ret
}

func histogramPoints[N int64 | float64](dps []metricdata.HistogramDataPoint[N]) []FTDCPoint {
	ret := make([]FTDCPoint, 0, len(dps))
	for _, dp := range dps {
		ret = append(ret, FTDCPoint{Attrs: dp.Attributes, Count: dp.Count, Sum: float64(dp.Sum), histogram: true})
	}
	return ret
}

func genericName(metricName string, attrs attribute.Set) string {
	parts := []string{metricName}
	for _, kv := range attrs.ToSlice() {
		parts = append(parts, kv.Value.String())
	}
	return strings.Join(parts, ".")
}
