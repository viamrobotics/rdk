package web

import (
	"context"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/semconv/v1.43.0/rpcconv"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/internal/otelmetrics"
)

const (
	callDurationMetric    = "rpc.server.call.duration"
	activeRequestsMetric  = "viam.rpc.server.active_requests"
	sentBytesMetric       = "viam.rpc.server.sent_bytes"
	limitedRequestsMetric = "viam.rpc.server.limited_requests"
	limitKeyAttr          = attribute.Key("viam.rpc.limit_key")
)

type requestInstruments struct {
	duration rpcconv.ServerCallDuration
	active   metric.Int64UpDownCounter
	sent     metric.Int64Counter
}

func newRequestInstruments(meter metric.Meter) (requestInstruments, error) {
	duration, err := rpcconv.NewServerCallDuration(meter)
	if err != nil {
		return requestInstruments{}, err
	}
	active, err := meter.Int64UpDownCounter(activeRequestsMetric,
		metric.WithDescription("RPCs that have started and not finished."),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return requestInstruments{}, err
	}
	sent, err := meter.Int64Counter(sentBytesMetric,
		metric.WithDescription("Bytes of response messages sent."),
		metric.WithUnit("By"),
	)
	return requestInstruments{duration: duration, active: active, sent: sent}, err
}

func noopRequestInstruments() requestInstruments {
	inst, _ := newRequestInstruments(noop.NewMeterProvider().Meter("")) //nolint:errcheck
	return inst
}

// RegisterMetrics records request metrics through metrics and writes them to FTDC under "web."
// keys. It must run before the web service starts. The caller must Unregister the result.
func (rc *RequestCounter) RegisterMetrics(metrics *otelmetrics.Metrics) (metric.Registration, error) {
	meter := metrics.MeterProvider().Meter("go.viam.com/rdk/robot/web")
	inst, err := newRequestInstruments(meter)
	if err != nil {
		return nil, err
	}
	limited, err := meter.Int64ObservableGauge(limitedRequestsMetric,
		metric.WithDescription("In-flight requests counted against each per-resource request limit."),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, err
	}
	rc.inst = inst
	metrics.MapFTDC(callDurationMetric, rc.durationFTDC)
	metrics.MapFTDC(activeRequestsMetric, activeFTDC)
	metrics.MapFTDC(sentBytesMetric, sentFTDC)
	metrics.MapFTDC(limitedRequestsMetric, limitedFTDC)
	return meter.RegisterCallback(func(_ context.Context, o metric.Observer) error {
		for key, count := range rc.inFlightRequests.Range {
			o.ObserveInt64(limited, count.Load(), metric.WithAttributes(limitKeyAttr.String(key)))
		}
		return nil
	}, limited)
}

// requestAttrs labels an RPC with its method and, for Viam APIs, the resource named in req. The
// resource name also goes on the RPC's span.
func requestAttrs(ctx context.Context, fullMethod string, api apiMethod, req any) []attribute.KeyValue {
	attrs := []attribute.KeyValue{semconv.RPCMethod(strings.TrimPrefix(fullMethod, "/"))}
	if api.shortPath == "" || req == nil {
		return attrs
	}
	if name := api.getResourceName(req); name != "" {
		attr := otelmetrics.ResourceNameKey.String(name)
		trace.SpanFromContext(ctx).SetAttributes(attr)
		attrs = append(attrs, attr)
	}
	return attrs
}

func (rc *RequestCounter) begin(ctx context.Context, attrs []attribute.KeyValue) {
	rc.inst.active.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// finish records the duration before it decrements the active count, so FTDC never sees a
// finished request missing from both.
func (rc *RequestCounter) finish(ctx context.Context, attrs []attribute.KeyValue, start time.Time, sent int, err error) {
	opt := metric.WithAttributes(attrs...)
	rc.inst.sent.Add(ctx, int64(sent), opt)
	rc.inst.duration.Record(ctx, time.Since(start).Seconds(), rpcconv.SystemNameGRPC,
		append(attrs[:len(attrs):len(attrs)], semconv.RPCResponseStatusCode(statusName(statusCode(err))))...)
	rc.inst.active.Add(ctx, -1, opt)
}

var statusNames = [...]string{
	codes.OK:                 "OK",
	codes.Canceled:           "CANCELLED",
	codes.Unknown:            "UNKNOWN",
	codes.InvalidArgument:    "INVALID_ARGUMENT",
	codes.DeadlineExceeded:   "DEADLINE_EXCEEDED",
	codes.NotFound:           "NOT_FOUND",
	codes.AlreadyExists:      "ALREADY_EXISTS",
	codes.PermissionDenied:   "PERMISSION_DENIED",
	codes.ResourceExhausted:  "RESOURCE_EXHAUSTED",
	codes.FailedPrecondition: "FAILED_PRECONDITION",
	codes.Aborted:            "ABORTED",
	codes.OutOfRange:         "OUT_OF_RANGE",
	codes.Unimplemented:      "UNIMPLEMENTED",
	codes.Internal:           "INTERNAL",
	codes.Unavailable:        "UNAVAILABLE",
	codes.DataLoss:           "DATA_LOSS",
	codes.Unauthenticated:    "UNAUTHENTICATED",
}

// statusCode returns the code a client receives when a handler returns err, as grpc-go derives it.
func statusCode(err error) codes.Code {
	if s, ok := status.FromError(err); ok {
		return s.Code()
	}
	return status.FromContextError(err).Code()
}

// statusName returns the canonical gRPC name of code, as otelgrpc reports it.
func statusName(code codes.Code) string {
	if int(code) < len(statusNames) {
		return statusNames[code]
	}
	return code.String()
}

// requestFTDCKey returns the FTDC key "web.<resource>.<Service>/<Method>", or "web.<Service>/<Method>"
// for requests without a resource. Only Viam APIs have FTDC keys.
func requestFTDCKey(attrs attribute.Set) (string, bool) {
	method, _ := attrs.Value(semconv.RPCMethodKey)
	api := extractViamAPI("/" + method.AsString())
	if api.shortPath == "" {
		return "", false
	}
	if name, ok := attrs.Value(otelmetrics.ResourceNameKey); ok {
		return "web." + name.AsString() + "." + api.shortPath, true
	}
	return "web." + api.shortPath, true
}

// addRequestKeys writes all four keys of a request, so each exists once any of its metrics does.
// The count is started requests: finished ones plus active ones.
func addRequestKeys(add func(string, float64), key string, count, errors, timeSpentMs, sent float64) {
	add(key, count)
	add(key+".errorCnt", errors)
	add(key+".timeSpent", timeSpentMs)
	add(key+".dataSentBytes", sent)
}

func (rc *RequestCounter) durationFTDC(p otelmetrics.FTDCPoint, add func(string, float64)) {
	key, ok := requestFTDCKey(p.Attrs)
	if !ok {
		return
	}
	var errors float64
	switch code, _ := p.Attrs.Value(semconv.RPCResponseStatusCodeKey); code.AsString() {
	case statusNames[codes.OK]:
	case statusNames[codes.Canceled]:
		// Clients end streams by cancelling them, so only a cancelled unary call is an error.
		method, _ := p.Attrs.Value(semconv.RPCMethodKey)
		if _, stream := rc.streamMethods.Load(method.AsString()); !stream {
			errors = float64(p.Count)
		}
	default:
		errors = float64(p.Count)
	}
	addRequestKeys(add, key, float64(p.Count), errors, p.Sum*1000, 0)
}

func activeFTDC(p otelmetrics.FTDCPoint, add func(string, float64)) {
	if key, ok := requestFTDCKey(p.Attrs); ok {
		addRequestKeys(add, key, p.Value, 0, 0, 0)
	}
}

func sentFTDC(p otelmetrics.FTDCPoint, add func(string, float64)) {
	if key, ok := requestFTDCKey(p.Attrs); ok {
		addRequestKeys(add, key, 0, 0, 0, p.Value)
	}
}

func limitedFTDC(p otelmetrics.FTDCPoint, add func(string, float64)) {
	key, _ := p.Attrs.Value(limitKeyAttr)
	add("web."+key.AsString()+".inFlightRequests", p.Value)
}
