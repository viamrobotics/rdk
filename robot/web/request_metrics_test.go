package web

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	commonpb "go.viam.com/api/common/v1"
	armpb "go.viam.com/api/component/arm/v1"
	"go.viam.com/test"
	googlegrpc "google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	"go.viam.com/rdk/logging"
)

func TestRequestCounterMetrics(t *testing.T) {
	ctx := context.Background()
	rc := &RequestCounter{logger: logging.NewTestLogger(t)}
	rc.ensureLimit()
	reader := sdkmetric.NewManualReader()
	reg, err := rc.RegisterMetrics(sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)).Meter("test"))
	test.That(t, err, test.ShouldBeNil)
	t.Cleanup(func() { test.That(t, reg.Unregister(), test.ShouldBeNil) })

	info := &googlegrpc.UnaryServerInfo{FullMethod: "/viam.component.arm.v1.ArmService/GetEndPosition"}
	req := &armpb.GetEndPositionRequest{Name: "arm1"}
	resp := &armpb.GetEndPositionResponse{Pose: &commonpb.Pose{X: 1}}
	_, err = rc.UnaryInterceptor(ctx, req, info, func(context.Context, any) (any, error) { return resp, nil })
	test.That(t, err, test.ShouldBeNil)
	_, err = rc.UnaryInterceptor(ctx, req, info, func(context.Context, any) (any, error) { return nil, errors.New("failed") })
	test.That(t, err, test.ShouldNotBeNil)

	var rm metricdata.ResourceMetrics
	test.That(t, reader.Collect(ctx, &rm), test.ShouldBeNil)
	want := attribute.NewSet(
		attribute.String("viam.resource.name", "arm1"),
		attribute.String("viam.rpc.method", "ArmService/GetEndPosition"),
	)
	got := map[string]int64{}
	for _, m := range rm.ScopeMetrics[0].Metrics {
		if sum, ok := m.Data.(metricdata.Sum[int64]); ok {
			for _, dp := range sum.DataPoints {
				if dp.Attributes.Equals(&want) {
					got[m.Name] = dp.Value
				}
			}
		}
	}
	test.That(t, got, test.ShouldResemble, map[string]int64{
		"viam.rpc.server.requests": 2,
		"viam.rpc.server.errors":   1,
		"viam.rpc.server.sent":     int64(proto.Size(resp)),
	})
}
