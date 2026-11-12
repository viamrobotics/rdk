package module_test

import (
	"context"
	"net"
	"testing"

	"github.com/fullstorydev/grpcurl"
	pb "go.viam.com/api/module/v1"
	robotpb "go.viam.com/api/robot/v1"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// dialBareServer connects to a gRPC server that serves none of the resource APIs, standing in
// for a module whose proto registry cannot provide the requested service descriptor.
func dialBareServer(t *testing.T, withReflection bool) rpc.ClientConn {
	t.Helper()
	lis, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	test.That(t, err, test.ShouldBeNil)
	srv := grpc.NewServer()
	if withReflection {
		reflection.Register(srv)
	}
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	test.That(t, err, test.ShouldBeNil)
	t.Cleanup(func() { test.That(t, conn.Close(), test.ShouldBeNil) })
	return rpc.GrpcOverHTTPClientConn{ClientConn: conn}
}

func TestNewHandlerMapFromProto(t *testing.T) {
	ctx := context.Background()

	motorModel := resource.NewModel("acme", "test", "mymotor")
	motorHandler := &pb.HandlerDefinition{
		Subtype: &robotpb.ResourceRPCSubtype{
			Subtype:      protoutils.ResourceNameToProto(resource.Name{API: motor.API}),
			ProtoService: "viam.component.motor.v1.MotorService",
		},
		Models: []string{motorModel.String()},
	}
	// Must not be registered by any package linked into this test binary.
	foreignAPI := resource.APINamespace("acme").WithComponentType("widget")
	_, registered := resource.LookupGenericAPIRegistration(foreignAPI)
	test.That(t, registered, test.ShouldBeFalse)
	foreignHandler := &pb.HandlerDefinition{
		Subtype: &robotpb.ResourceRPCSubtype{
			Subtype:      protoutils.ResourceNameToProto(resource.Name{API: foreignAPI}),
			ProtoService: "acme.component.widget.v1.WidgetService",
		},
		Models: []string{"acme:test:mywidget"},
	}

	t.Run("registered API does not need the module to serve its descriptor", func(t *testing.T) {
		hMap, err := module.NewHandlerMapFromProto(ctx,
			&pb.HandlerMap{Handlers: []*pb.HandlerDefinition{motorHandler}}, dialBareServer(t, true))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, hMap, test.ShouldResemble, module.HandlerMap{
			{API: motor.API}: {motorModel},
		})
	})

	t.Run("registered API does not need reflection at all", func(t *testing.T) {
		hMap, err := module.NewHandlerMapFromProto(ctx,
			&pb.HandlerMap{Handlers: []*pb.HandlerDefinition{motorHandler}}, dialBareServer(t, false))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, hMap, test.ShouldResemble, module.HandlerMap{
			{API: motor.API}: {motorModel},
		})
	})

	t.Run("unregistered API still needs a descriptor", func(t *testing.T) {
		hMap, err := module.NewHandlerMapFromProto(ctx,
			&pb.HandlerMap{Handlers: []*pb.HandlerDefinition{motorHandler, foreignHandler}}, dialBareServer(t, true))
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "acme.component.widget.v1.WidgetService")
		test.That(t, hMap, test.ShouldResemble, module.HandlerMap{
			{API: motor.API}: {motorModel},
		})
	})

	t.Run("unregistered API without reflection", func(t *testing.T) {
		hMap, err := module.NewHandlerMapFromProto(ctx,
			&pb.HandlerMap{Handlers: []*pb.HandlerDefinition{foreignHandler}}, dialBareServer(t, false))
		test.That(t, err, test.ShouldBeError, grpcurl.ErrReflectionNotSupported)
		test.That(t, hMap, test.ShouldBeNil)
	})
}
