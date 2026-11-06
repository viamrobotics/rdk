package web

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/utils/ssync"
)

const (
	// RequestDurationMetric is a histogram of Viam API request durations in milliseconds, labeled by
	// resource, service, method, and gRPC status code. For streams, it is the stream's lifetime.
	// otelgrpc's rpc.server.duration covers the same requests but cannot be labeled by resource,
	// because otelgrpc fixes its attributes before the request message is decoded.
	RequestDurationMetric = "viam.resource.request.duration"

	// ResourceNameAttribute is the short name of the resource a request targets. It is omitted for
	// requests that do not target a resource.
	ResourceNameAttribute = attribute.Key("viam.resource.name")
)

// errHandlerPanicked records a panicking handler with the status goutils' recovery interceptor
// returns to the client.
var errHandlerPanicked = status.Error(codes.Internal, "handler panicked")

type requestDurationKey struct {
	requestKey string
	code       codes.Code
}

type requestDuration struct {
	histogram metric.Float64Histogram
	// attrs caches each series' attributes so steady-state requests do not allocate them.
	attrs ssync.Map[requestDurationKey, metric.RecordOption]
}

// initMetrics creates the request instruments from the global MeterProvider, which is a no-op
// unless metrics export is enabled.
func (rc *RequestCounter) initMetrics() {
	histogram, err := otel.Meter("go.viam.com/rdk/robot/web").Float64Histogram(RequestDurationMetric,
		metric.WithUnit("ms"),
		metric.WithDescription("Duration of Viam API requests."))
	if err != nil {
		rc.logger.Warnw("Failed to create request metric", "metric", RequestDurationMetric, "err", err)
		return
	}
	rc.duration = &requestDuration{histogram: histogram}
}

// record records a finished request to method on resourceName ("" for none). requestKey must
// uniquely identify the resource and method; see buildRCKey.
func (d *requestDuration) record(
	ctx context.Context, requestKey, resourceName string, method apiMethod, elapsed time.Duration, err error,
) {
	if d == nil {
		return
	}
	key := requestDurationKey{requestKey, status.Code(err)}
	opt, ok := d.attrs.Load(key)
	if !ok {
		kvs := []attribute.KeyValue{
			semconv.RPCService(method.service),
			semconv.RPCMethod(method.name),
			semconv.RPCGRPCStatusCodeKey.Int(int(key.code)),
		}
		if resourceName != "" {
			kvs = append(kvs, ResourceNameAttribute.String(resourceName))
		}
		opt, _ = d.attrs.LoadOrStore(key, metric.WithAttributeSet(attribute.NewSet(kvs...)))
	}
	d.histogram.Record(ctx, float64(elapsed)/float64(time.Millisecond), opt)
}
