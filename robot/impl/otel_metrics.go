package robotimpl

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type resourceCountKey struct {
	api   string
	state string
}

// registerResourceMetrics reports the number of resources in the graph by API and state through
// the global MeterProvider. The caller must Unregister the result before closing the manager.
func (r *localRobot) registerResourceMetrics() (metric.Registration, error) {
	meter := otel.Meter("go.viam.com/rdk/robot/impl")
	gauge, err := meter.Int64ObservableGauge("viam.resource.count",
		metric.WithDescription("Resources in the resource graph, by API and state."),
		metric.WithUnit("{resource}"),
	)
	if err != nil {
		return nil, err
	}
	return meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
		counts := map[resourceCountKey]int64{}
		for _, status := range r.manager.resources.Status() {
			counts[resourceCountKey{status.Name.API.String(), status.State.String()}]++
		}
		for key, count := range counts {
			o.ObserveInt64(gauge, count, metric.WithAttributes(
				attribute.String("viam.resource.api", key.api),
				attribute.String("viam.resource.state", key.state),
			))
		}
		return nil
	}, gauge)
}
