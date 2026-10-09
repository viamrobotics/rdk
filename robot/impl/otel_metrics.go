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
	resourceModelKey    = attribute.Key("viam.resource.model")
	moduleNameKey       = attribute.Key("viam.module.name")
)

var availabilityByName = map[string]resource.Availability{
	resource.AvailabilityReady.String():         resource.AvailabilityReady,
	resource.AvailabilityUninitialized.String(): resource.AvailabilityUninitialized,
	resource.AvailabilityRemoving.String():      resource.AvailabilityRemoving,
	resource.AvailabilityUnhealthy.String():     resource.AvailabilityUnhealthy,
}

// registerResourceMetrics reports 1 for each resource on its current availability, labeled with
// its model and serving module, and writes it to FTDC as "<resource>.State". The caller must
// Unregister the result before closing the manager.
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
		r.manager.modManagerLock.Lock()
		modManager := r.manager.moduleManager
		r.manager.modManagerLock.Unlock()
		for _, node := range r.manager.resources.Availability() {
			attrs := []attribute.KeyValue{
				otelmetrics.ResourceAPIKey.String(node.Name.API.String()),
				otelmetrics.ResourceNameKey.String(node.Name.ShortName()),
				resourceStateKey.String(node.Availability.String()),
			}
			if node.Model.Name != "" {
				attrs = append(attrs, resourceModelKey.String(node.Model.String()))
			}
			if modManager != nil {
				if module, ok := modManager.ModuleName(node.Name); ok {
					attrs = append(attrs, moduleNameKey.String(module))
				}
			}
			o.ObserveInt64(gauge, 1, metric.WithAttributes(attrs...))
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
