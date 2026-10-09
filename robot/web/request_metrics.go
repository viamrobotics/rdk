package web

import (
	"context"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/multierr"
)

// RegisterMetrics reports this counter's calls, errors, time spent and bytes sent, per resource
// and method, through meter on each collection. Unregister the result to stop.
func (rc *RequestCounter) RegisterMetrics(meter metric.Meter) (metric.Registration, error) {
	requests, err1 := meter.Int64ObservableCounter("viam.rpc.server.requests",
		metric.WithDescription("Viam API requests started; a stream counts once."),
		metric.WithUnit("{request}"))
	failures, err2 := meter.Int64ObservableCounter("viam.rpc.server.errors",
		metric.WithDescription("Unary Viam API requests whose handler returned an error."),
		metric.WithUnit("{request}"))
	timeSpent, err3 := meter.Float64ObservableCounter("viam.rpc.server.time_spent",
		metric.WithDescription("Time spent serving unary Viam API requests, each truncated to whole milliseconds."),
		metric.WithUnit("s"))
	sent, err4 := meter.Int64ObservableCounter("viam.rpc.server.sent",
		metric.WithDescription("Bytes of Viam API response messages sent."),
		metric.WithUnit("By"))
	if err := multierr.Combine(err1, err2, err3, err4); err != nil {
		return nil, err
	}
	return meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
		for key, stats := range rc.requestKeyToStats.Range {
			opt := metric.WithAttributes(requestKeyAttrs(key)...)
			o.ObserveInt64(requests, stats.count.Load(), opt)
			o.ObserveInt64(failures, stats.errorCnt.Load(), opt)
			o.ObserveFloat64(timeSpent, float64(stats.timeSpent.Load())/1000, opt)
			o.ObserveInt64(sent, stats.dataSent.Load(), opt)
		}
		return nil
	}, requests, failures, timeSpent, sent)
}

// requestKeyAttrs splits a request key, "<resource>.<Service>/<Method>" or "<Service>/<Method>".
func requestKeyAttrs(key string) []attribute.KeyValue {
	if name, method, ok := strings.Cut(key, "."); ok {
		return []attribute.KeyValue{attribute.String("viam.resource.name", name), attribute.String("viam.rpc.method", method)}
	}
	return []attribute.KeyValue{attribute.String("viam.rpc.method", key)}
}
