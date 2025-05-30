// Package statz used to wrap OpenCensus metric collection
package statz

import (
	"fmt"
	"regexp"

	"github.com/edaniels/golog"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"

	"go.viam.com/utils/perf/statz/internal"
	"go.viam.com/utils/perf/statz/units"
)

// MetricConfig defines a single metric at program startup and holds all associated metadata.
type MetricConfig struct {
	Description string
	Unit        units.Unit
	Labels      []Label
}

const (
	maxNameLength      = 150
	nameRegex          = "[a-zA-Z0-9/\\._]+"
	maxLabelNameLength = 100
	labelNameRegex     = "[a-zA-Z][a-zA-Z0-9_]*"
)

// Examples

//// Int64 Counters - Create a counter at the package level.
//
// var uploadCounter = statz.NewCounter2[string, bool]("datasync/uploaded", statz.MetricConfig{
// 		Description: "The number of requests",
// 		Unit:        units.Dimensionless,
// 		Labels: []statz.Label{
// 			{Name: "type", Description: "The data type (file|binary|tabular)."},
// 			{Name: "status", Description: "If the upload was Successful."},
// 		},
//  })
//
// Usage:
// uploadCounter.Inc(“uploadType”, false|true)
//

// NewCounter0 creates a new counter metric with 0 labels.
func NewCounter0(name string, cfg MetricConfig) Counter0 {
	return Counter0{
		wrapper: createCounterWrapper(name, cfg),
	}
}

// NewCounter1 creates a new counter metric with 1 labels.
func NewCounter1[T1 labelContraint](name string, cfg MetricConfig) Counter1[T1] {
	return Counter1[T1]{
		wrapper: createCounterWrapper(name, cfg),
	}
}

// NewCounter2 creates a new counter metric with 2 labels.
func NewCounter2[T1, T2 labelContraint](name string, cfg MetricConfig) Counter2[T1, T2] {
	return Counter2[T1, T2]{
		wrapper: createCounterWrapper(name, cfg),
	}
}

// NewCounter3 creates a new counter metric with 3 labels.
func NewCounter3[T1, T2, T3 labelContraint](name string, cfg MetricConfig) Counter3[T1, T2, T3] {
	return Counter3[T1, T2, T3]{
		wrapper: createCounterWrapper(name, cfg),
	}
}

// NewCounter4 creates a new counter metric with 4 labels.
func NewCounter4[T1, T2, T3, T4 labelContraint](name string, cfg MetricConfig,
) Counter4[T1, T2, T3, T4] {
	return Counter4[T1, T2, T3, T4]{
		wrapper: createCounterWrapper(name, cfg),
	}
}

// NewCounter5 creates a new counter metric with 5 labels.
func NewCounter5[T1, T2, T3, T4, T5 labelContraint](name string, cfg MetricConfig,
) Counter5[T1, T2, T3, T4, T5] {
	return Counter5[T1, T2, T3, T4, T5]{
		wrapper: createCounterWrapper(name, cfg),
	}
}

//// Int64 Summations - Create a summation at the package level.
//
// var uploadsInFlightCounter = statz.NewSummation1[string]("datasync/uploads_in_flight", statz.MetricConfig{
// 		Description: "The number of requests in flight",
// 		Unit:        units.Dimensionless,
// 		Labels: []statz.Label{
// 			{Name: "type", Description: "The data type (file|binary|tabular)."},
// 		},
//  })
//
// Usage:
// uploadsInFlightCounter.IncBy(“uploadType”, 5)
// uploadsInFlightCounter.IncBy(“uploadType”, -5)
//

// NewSummation0 creates a new summation metric with 0 labels.
func NewSummation0(name string, cfg MetricConfig) Summation0 {
	return Summation0{
		wrapper: createSummationWrapper(name, cfg),
	}
}

// NewSummation1 creates a new summation metric with 1 labels.
func NewSummation1[T1 labelContraint](name string, cfg MetricConfig) Summation1[T1] {
	return Summation1[T1]{
		wrapper: createSummationWrapper(name, cfg),
	}
}

// NewSummation2 creates a new summation metric with 2 labels.
func NewSummation2[T1, T2 labelContraint](name string, cfg MetricConfig) Summation2[T1, T2] {
	return Summation2[T1, T2]{
		wrapper: createSummationWrapper(name, cfg),
	}
}

// NewSummation3 creates a new summation metric with 3 labels.
func NewSummation3[T1, T2, T3 labelContraint](name string, cfg MetricConfig) Summation3[T1, T2, T3] {
	return Summation3[T1, T2, T3]{
		wrapper: createSummationWrapper(name, cfg),
	}
}

// NewSummation4 creates a new summation metric with 4 labels.
func NewSummation4[T1, T2 labelContraint,
	T3, T4 labelContraint](name string, cfg MetricConfig,
) Summation4[T1, T2, T3, T4] {
	return Summation4[T1, T2, T3, T4]{
		wrapper: createSummationWrapper(name, cfg),
	}
}

//// Int64 Gauges - Create a gauge at the package level.
//
// var uploadsInFlightCounter = statz.NewGauge1[string]("datasync/uploads_in_flight", statz.MetricConfig{
// 		Description: "The number of requests in flight",
// 		Unit:        units.Dimensionless,
// 		Labels: []statz.Label{
// 			{Name: "type", Description: "The data type (file|binary|tabular)."},
// 		},
//  })
//
// Usage:
// uploadsInFlightCounter.Set(“uploadType”, 5)
//

// NewGauge0 creates a new gauge metric with 0 labels.
func NewGauge0(name string, cfg MetricConfig) Gauge0 {
	return Gauge0{
		wrapper: createGaugeWrapper(name, cfg),
	}
}

// NewGauge1 creates a new gauge metric with 1 labels.
func NewGauge1[T1 labelContraint](name string, cfg MetricConfig) Gauge1[T1] {
	return Gauge1[T1]{
		wrapper: createGaugeWrapper(name, cfg),
	}
}

// NewGauge2 creates a new gauge metric with 2 labels.
func NewGauge2[T1, T2 labelContraint](name string, cfg MetricConfig) Gauge2[T1, T2] {
	return Gauge2[T1, T2]{
		wrapper: createGaugeWrapper(name, cfg),
	}
}

// NewGauge3 creates a new gauge metric with 3 labels.
func NewGauge3[T1, T2, T3 labelContraint](name string, cfg MetricConfig) Gauge3[T1, T2, T3] {
	return Gauge3[T1, T2, T3]{
		wrapper: createGaugeWrapper(name, cfg),
	}
}

// NewGauge4 creates a new gauge metric with 4 labels.
func NewGauge4[T1, T2 labelContraint,
	T3, T4 labelContraint](name string, cfg MetricConfig,
) Gauge4[T1, T2, T3, T4] {
	return Gauge4[T1, T2, T3, T4]{
		wrapper: createGaugeWrapper(name, cfg),
	}
}

//// Float64 Distribution - Create a distribution at the package level.
//
// var uploadLatency = statz.Distribution2[string, bool]("datasync/uploaded_latency", statz.MetricConfig{
// 		Description: "The latency of the upload",
// 		Unit:        units.Milliseconds,
// 		Labels: []statz.Label{
// 			{Name: "type", Description: "The data type (file|binary|tabular)."},
// 			{Name: "status", Description: "If the upload was Successful."},
// 		},
//  })
//
// Usage:
// uploadLatency.Observe(110.2, “uploadType”, false|true)
//

// NewDistribution0 creates a new distribution metric with 0 labels.
func NewDistribution0(name string, cfg MetricConfig, distribution Distribution) Distribution0 {
	return Distribution0{
		wrapper: createocDistributionWrapper(name, distribution, cfg),
	}
}

// NewDistribution1 creates a new distribution metric with 1 labels.
func NewDistribution1[T1 labelContraint](name string, cfg MetricConfig, distribution Distribution) Distribution1[T1] {
	return Distribution1[T1]{
		wrapper: createocDistributionWrapper(name, distribution, cfg),
	}
}

// NewDistribution2 creates a new distribution metric with 2 labels.
func NewDistribution2[T1, T2 labelContraint](name string,
	cfg MetricConfig, distribution Distribution,
) Distribution2[T1, T2] {
	return Distribution2[T1, T2]{
		wrapper: createocDistributionWrapper(name, distribution, cfg),
	}
}

// NewDistribution3 creates a new distribution metric with 3 labels.
func NewDistribution3[T1, T2, T3 labelContraint](name string,
	cfg MetricConfig, distribution Distribution,
) Distribution3[T1, T2, T3] {
	return Distribution3[T1, T2, T3]{
		wrapper: createocDistributionWrapper(name, distribution, cfg),
	}
}

// NewDistribution4 creates a new distribution metric with 4 labels.
func NewDistribution4[T1, T2, T3, T4 labelContraint](name string,
	cfg MetricConfig, distribution Distribution,
) Distribution4[T1, T2, T3, T4] {
	return Distribution4[T1, T2, T3, T4]{
		wrapper: createocDistributionWrapper(name, distribution, cfg),
	}
}

func createAndRegisterOpenCensusMetric(name string, measure stats.Measure, agg *view.Aggregation, cfg MetricConfig) *opencensusStatsData {
	// Register with statz global
	internal.RegisterMetric(name)

	if err := validateMetricName(name); err != nil {
		golog.Global().Panicf("Failed to register metric name not valid: %s", err)
		return nil
	}

	for _, l := range cfg.Labels {
		if err := validateMetricLabel(l); err != nil {
			golog.Global().Panicf("Failed to register metric label not valid: %s", err)
			return nil
		}
	}

	tagKeys := tagKeysFromConfig(&cfg)

	// We do this twice to ensure the ordering of the key
	// is not changed when we use it in labelKeys. OpenCensus
	// seems to reorder the TagKeys and we cannot reliably use it.
	tagKeysForLabels := tagKeysFromConfig(&cfg)

	ocData := &opencensusStatsData{
		View: &view.View{
			Name:        name,
			Measure:     measure,
			Description: cfg.Description,
			Aggregation: agg,
			TagKeys:     tagKeys,
		},
		labelKeys: tagKeysForLabels,
	}

	// Register the views it is imperative that this step exists
	if err := view.Register(ocData.View); err != nil {
		golog.Global().Fatalf("Failed to register the views: %v", err)
	}

	return ocData
}

func validateMetricName(name string) error {
	if len(name) > maxNameLength {
		return fmt.Errorf("metric names must be less than %d characters", maxNameLength)
	}

	if match, err := regexp.MatchString(nameRegex, name); err != nil {
		golog.Global().Panic("Regex failed, this should not happen")
	} else if !match {
		return fmt.Errorf("metric name '%s' must be valud regex '%s'", name, nameRegex)
	}

	return nil
}

func validateMetricLabel(l Label) error {
	if len(l.Name) > maxLabelNameLength {
		return fmt.Errorf("label names must be less than %d characters", maxNameLength)
	}

	if match, err := regexp.MatchString(labelNameRegex, l.Name); err != nil {
		golog.Global().Panic("Regex failed, this should not happen")
	} else if !match {
		return fmt.Errorf("label name '%s' must be valud regex '%s'", l.Name, labelNameRegex)
	}

	return nil
}
