package robotimpl

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// registerMetrics exports request counts and resource states through meter. A failure is logged
// and leaves the robot running without that metric.
func (r *localRobot) registerMetrics(meter metric.Meter) {
	if reg, err := r.webSvc.RequestCounter().RegisterMetrics(meter); err != nil {
		r.logger.Warnw("Failed to register request metrics", "err", err)
	} else {
		r.metricRegistrations = append(r.metricRegistrations, reg)
	}
	if reg, err := r.registerResourceStates(meter); err != nil {
		r.logger.Warnw("Failed to register resource state metrics", "err", err)
	} else {
		r.metricRegistrations = append(r.metricRegistrations, reg)
	}
}

// registerResourceStates reports 1 for each resource in the graph, labeled with its state.
func (r *localRobot) registerResourceStates(meter metric.Meter) (metric.Registration, error) {
	states, err := meter.Int64ObservableGauge("viam.resource.state",
		metric.WithDescription("Resources in the resource graph, by state."),
		metric.WithUnit("{resource}"))
	if err != nil {
		return nil, err
	}
	return meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
		for _, status := range r.manager.resources.Status() {
			o.ObserveInt64(states, 1, metric.WithAttributes(
				attribute.String("viam.resource.api", status.Name.API.String()),
				attribute.String("viam.resource.name", status.Name.ShortName()),
				attribute.String("viam.resource.state", status.State.String()),
			))
		}
		return nil
	}, states)
}
