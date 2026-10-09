package robotimpl

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"go.viam.com/rdk/internal/otelmetrics"
	"go.viam.com/rdk/resource"
)

const (
	resourceStateMetric = "viam.resource.state"
	resourceStateKey    = attribute.Key("viam.resource.state")
)

var availabilityByName = map[string]resource.Availability{
	resource.AvailabilityReady.String():         resource.AvailabilityReady,
	resource.AvailabilityUninitialized.String(): resource.AvailabilityUninitialized,
	resource.AvailabilityRemoving.String():      resource.AvailabilityRemoving,
	resource.AvailabilityUnhealthy.String():     resource.AvailabilityUnhealthy,
}

// registerResourceMetrics reports 1 for each resource on its current availability, and writes it
// to FTDC as "<resource>.State". The caller must Unregister the result before closing the manager.
func (r *localRobot) registerResourceMetrics(metrics *otelmetrics.Metrics) (metric.Registration, error) {
	meter := metrics.MeterProvider().Meter("go.viam.com/rdk/robot/impl")
	gauge, err := meter.Int64ObservableGauge(resourceStateMetric,
		metric.WithDescription("Resources in the resource graph; each reports 1 on its current state."),
		metric.WithUnit("{resource}"),
	)
	if err != nil {
		return nil, err
	}
	metrics.MapFTDC(resourceStateMetric, resourceStateFTDCKey)
	return meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
		for name, availability := range r.manager.resources.Availability() {
			o.ObserveInt64(gauge, 1, metric.WithAttributes(
				otelmetrics.ResourceAPIKey.String(name.API.String()),
				otelmetrics.ResourceNameKey.String(name.ShortName()),
				resourceStateKey.String(availability.String()),
			))
		}
		return nil
	}, gauge)
}

func resourceStateFTDCKey(p otelmetrics.FTDCPoint, add func(string, float64)) {
	api, _ := p.Attrs.Value(otelmetrics.ResourceAPIKey)
	name, _ := p.Attrs.Value(otelmetrics.ResourceNameKey)
	state, _ := p.Attrs.Value(resourceStateKey)
	add(api.AsString()+"/"+name.AsString()+".State", float64(availabilityByName[state.AsString()]))
}
