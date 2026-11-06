package web

import (
	"context"
	"io"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	echopb "go.viam.com/api/component/testecho/v1"
	robotpb "go.viam.com/api/robot/v1"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	rgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	genericservice "go.viam.com/rdk/services/generic"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/testutils/robottestutils"
)

type durationPoint struct {
	resourceName, method string
	code                 codes.Code
}

// collectDurations returns the number of recorded requests per resource, method, and status code.
func collectDurations(t *testing.T, reader *sdkmetric.ManualReader) map[durationPoint]uint64 {
	t.Helper()
	var rm metricdata.ResourceMetrics
	test.That(t, reader.Collect(context.Background(), &rm), test.ShouldBeNil)
	counts := map[durationPoint]uint64{}
	for _, scope := range rm.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name != RequestDurationMetric {
				continue
			}
			hist, ok := m.Data.(metricdata.Histogram[float64])
			test.That(t, ok, test.ShouldBeTrue)
			for _, dp := range hist.DataPoints {
				value := func(key attribute.Key) attribute.Value {
					v, _ := dp.Attributes.Value(key)
					return v
				}
				counts[durationPoint{
					resourceName: value(ResourceNameAttribute).AsString(),
					method:       value(semconv.RPCMethodKey).AsString(),
					code:         codes.Code(value(semconv.RPCGRPCStatusCodeKey).AsInt64()),
				}] += dp.Count
			}
		}
	}
	return counts
}

func TestRequestDurationMetric(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	prevProvider := otel.GetMeterProvider()
	otel.SetMeterProvider(provider)
	defer func() {
		otel.SetMeterProvider(prevProvider)
		test.That(t, provider.Shutdown(context.Background()), test.ShouldBeNil)
	}()

	echoAPI := resource.NewAPI("rdk", "component", "echo")
	resource.RegisterAPI(echoAPI, resource.APIRegistration[resource.Resource]{
		RPCServiceServerConstructor: func(apiResColl resource.APIResourceGetter[resource.Resource], logger logging.Logger) interface{} {
			return &echoServer{}
		},
		RPCServiceHandler: echopb.RegisterTestEchoServiceHandlerFromEndpoint,
		RPCServiceDesc:    &echopb.TestEchoService_ServiceDesc,
	})
	defer resource.DeregisterAPI(echoAPI)

	logger := logging.NewTestLogger(t)
	ctx, iRobot := setupRobotCtx(t)
	iRobot.(*inject.Robot).MachineStatusFunc = func(ctx context.Context) (robot.MachineStatus, error) {
		return robot.MachineStatus{}, nil
	}

	svc := New(iRobot, logger)
	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	test.That(t, svc.Start(ctx, options), test.ShouldBeNil)
	defer func() { test.That(t, svc.Close(ctx), test.ShouldBeNil) }()

	conn, err := rgrpc.Dial(context.Background(), addr, logger, rpc.WithWebRTCOptions(rpc.DialWebRTCOptions{Disable: true}))
	test.That(t, err, test.ShouldBeNil)
	defer func() { test.That(t, conn.Close(), test.ShouldBeNil) }()

	_, err = robotpb.NewRobotServiceClient(conn).GetMachineStatus(ctx, &robotpb.GetMachineStatusRequest{})
	test.That(t, err, test.ShouldBeNil)

	echoClient := echopb.NewTestEchoServiceClient(conn)
	for range 2 {
		_, err = echoClient.Echo(ctx, &echopb.EchoRequest{Name: "test1"})
		test.That(t, err, test.ShouldBeNil)
	}
	_, err = echoClient.Echo(ctx, &echopb.EchoRequest{Name: "test2"})
	test.That(t, err, test.ShouldBeNil)

	stream, err := echoClient.EchoMultiple(ctx, &echopb.EchoMultipleRequest{Name: "test1", Message: "ab"})
	test.That(t, err, test.ShouldBeNil)
	for {
		if _, err := stream.Recv(); err != nil {
			test.That(t, err, test.ShouldEqual, io.EOF)
			break
		}
	}

	generic, err := genericservice.NewClientFromConn(ctx, conn, "", genericservice.Named("generictest"), logger)
	test.That(t, err, test.ShouldBeNil)
	// The injected resource has no DoCommand, so the handler panics.
	_, err = generic.DoCommand(ctx, nil)
	test.That(t, status.Code(err), test.ShouldEqual, codes.Internal)
	iRobot.(*inject.Robot).FindBySimpleNameAndAPIFunc = func(s string, a resource.API) (resource.Resource, error) {
		return nil, resource.NewNotFoundError(resource.NewName(a, s))
	}
	_, err = generic.DoCommand(ctx, nil)
	test.That(t, status.Code(err), test.ShouldEqual, codes.Unknown)

	test.That(t, collectDurations(t, reader), test.ShouldResemble, map[durationPoint]uint64{
		{resourceName: "", method: "GetMachineStatus", code: codes.OK}:           1,
		{resourceName: "test1", method: "Echo", code: codes.OK}:                  2,
		{resourceName: "test2", method: "Echo", code: codes.OK}:                  1,
		{resourceName: "test1", method: "EchoMultiple", code: codes.OK}:          1,
		{resourceName: "generictest", method: "DoCommand", code: codes.Unknown}:  1,
		{resourceName: "generictest", method: "DoCommand", code: codes.Internal}: 1,
	})
}
