package module_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/edaniels/golog"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	v1 "go.viam.com/api/app/v1"
	pb "go.viam.com/api/module/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/protoutils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/motor"
	_ "go.viam.com/rdk/components/motor/fake"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	"go.viam.com/rdk/examples/customresources/models/mybase"
	"go.viam.com/rdk/examples/customresources/models/mygizmo"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
	robotimpl "go.viam.com/rdk/robot/impl"
)

func TestModuleFunctions(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	gizmoConf := &v1.ComponentConfig{
		Name: "gizmo1", Api: "acme:component:gizmo", Model: "acme:demo:mygizmo",
	}
	myBaseAttrs, err := protoutils.StructToStructPb(mybase.MyBaseConfig{
		LeftMotor:  "motor1",
		RightMotor: "motor2",
	})
	test.That(t, err, test.ShouldBeNil)
	myBaseConf := &v1.ComponentConfig{
		Name:       "mybase1",
		Api:        "rdk:component:base",
		Model:      "acme:demo:mybase",
		Attributes: myBaseAttrs,
	}
	// myBaseConf2 is missing required attributes "motorL" and "motorR" and should
	// cause Validation error.
	badMyBaseConf := &v1.ComponentConfig{
		Name:  "mybase2",
		Api:   "rdk:component:base",
		Model: "acme:demo:mybase",
	}

	cfg := &config.Config{Components: []config.Component{
		{
			Name:  "motor1",
			API:   resource.NewSubtype("rdk", "component", "motor"),
			Model: resource.NewDefaultModel("fake"),
		},
		{
			Name:  "motor2",
			API:   resource.NewSubtype("rdk", "component", "motor"),
			Model: resource.NewDefaultModel("fake"),
		},
	}}

	myRobot, err := robotimpl.RobotFromConfig(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	parentAddr, err := myRobot.ModuleAddress()
	test.That(t, err, test.ShouldBeNil)

	addr := filepath.ToSlash(filepath.Join(filepath.Dir(parentAddr), "mod.sock"))
	m, err := module.NewModule(ctx, addr, logger)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, m.AddModelFromRegistry(ctx, gizmoapi.Subtype, mygizmo.Model), test.ShouldBeNil)
	test.That(t, m.AddModelFromRegistry(ctx, base.Subtype, mybase.Model), test.ShouldBeNil)

	test.That(t, m.Start(ctx), test.ShouldBeNil)

	conn, err := grpc.Dial(
		"unix://"+addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStreamInterceptor(grpc_retry.StreamClientInterceptor()),
		grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor()),
	)
	test.That(t, err, test.ShouldBeNil)

	client := pb.NewModuleServiceClient(conn)

	m.SetReady(false)

	resp, err := client.Ready(ctx, &pb.ReadyRequest{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Ready, test.ShouldBeFalse)

	m.SetReady(true)

	resp, err = client.Ready(ctx, &pb.ReadyRequest{ParentAddress: parentAddr})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Ready, test.ShouldBeTrue)

	t.Run("HandlerMap", func(t *testing.T) {
		// test the raw return
		handlers := resp.GetHandlermap().GetHandlers()
		test.That(t, "acme", test.ShouldBeIn, handlers[0].Subtype.Subtype.Namespace, handlers[1].Subtype.Subtype.Namespace)
		test.That(t, "rdk", test.ShouldBeIn, handlers[0].Subtype.Subtype.Namespace, handlers[1].Subtype.Subtype.Namespace)
		test.That(t, "component", test.ShouldBeIn, handlers[0].Subtype.Subtype.Type, handlers[1].Subtype.Subtype.Type)
		test.That(t, "gizmo", test.ShouldBeIn, handlers[0].Subtype.Subtype.Subtype, handlers[1].Subtype.Subtype.Subtype)
		test.That(t, "base", test.ShouldBeIn, handlers[0].Subtype.Subtype.Subtype, handlers[1].Subtype.Subtype.Subtype)
		test.That(t, "acme:demo:mygizmo", test.ShouldBeIn, handlers[0].GetModels()[0], handlers[1].GetModels()[0])
		test.That(t, "acme:demo:mybase", test.ShouldBeIn, handlers[0].GetModels()[0], handlers[1].GetModels()[0])

		// convert from proto
		hmap, err := module.NewHandlerMapFromProto(ctx, resp.GetHandlermap(), conn)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(hmap), test.ShouldEqual, 2)

		for k, v := range hmap {
			test.That(t, k.Subtype, test.ShouldBeIn, gizmoapi.Subtype, base.Subtype)
			if k.Subtype == gizmoapi.Subtype {
				test.That(t, mygizmo.Model, test.ShouldResemble, v[0])
			} else {
				test.That(t, mybase.Model, test.ShouldResemble, v[0])
			}
		}

		// convert back to proto
		handlers2 := hmap.ToProto().GetHandlers()
		test.That(t, "acme", test.ShouldBeIn, handlers2[0].Subtype.Subtype.Namespace, handlers2[1].Subtype.Subtype.Namespace)
		test.That(t, "rdk", test.ShouldBeIn, handlers2[0].Subtype.Subtype.Namespace, handlers2[1].Subtype.Subtype.Namespace)
		test.That(t, "component", test.ShouldBeIn, handlers2[0].Subtype.Subtype.Type, handlers2[1].Subtype.Subtype.Type)
		test.That(t, "gizmo", test.ShouldBeIn, handlers2[0].Subtype.Subtype.Subtype, handlers2[1].Subtype.Subtype.Subtype)
		test.That(t, "base", test.ShouldBeIn, handlers2[0].Subtype.Subtype.Subtype, handlers2[1].Subtype.Subtype.Subtype)
		test.That(t, "acme:demo:mygizmo", test.ShouldBeIn, handlers2[0].GetModels()[0], handlers2[1].GetModels()[0])
		test.That(t, "acme:demo:mybase", test.ShouldBeIn, handlers2[0].GetModels()[0], handlers2[1].GetModels()[0])
	})

	t.Run("GetParentResource", func(t *testing.T) {
		motor1, err := m.GetParentResource(ctx, motor.Named("motor1"))
		test.That(t, err, test.ShouldBeNil)

		rMotor, ok := motor1.(motor.Motor)
		test.That(t, ok, test.ShouldBeTrue)

		err = rMotor.Stop(ctx, nil)
		test.That(t, err, test.ShouldBeNil)

		err = utils.TryClose(ctx, rMotor)
		test.That(t, err, test.ShouldBeNil)

		// Test that GetParentResource will refresh resources on the parent
		cfg.Components = append(cfg.Components, config.Component{
			Name:  "motor2",
			API:   resource.NewSubtype("rdk", "component", "motor"),
			Model: resource.NewDefaultModel("fake"),
		})
		myRobot.Reconfigure(ctx, cfg)
		_, err = m.GetParentResource(ctx, motor.Named("motor2"))
		test.That(t, err, test.ShouldBeNil)
	})

	var gClient gizmoapi.Gizmo
	t.Run("AddResource", func(t *testing.T) {
		_, err = m.AddResource(ctx, &pb.AddResourceRequest{Config: gizmoConf})
		test.That(t, err, test.ShouldBeNil)

		gClient = gizmoapi.NewClientFromConn(conn, "gizmo1", logger)

		ret, err := gClient.DoOne(ctx, "test")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ret, test.ShouldBeFalse)

		ret, err = gClient.DoOne(ctx, "")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ret, test.ShouldBeTrue)

		// Test generic echo
		testCmd := map[string]interface{}{"foo": "bar"}
		retCmd, err := gClient.DoCommand(context.Background(), testCmd)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, retCmd, test.ShouldResemble, testCmd)
	})

	t.Run("ReconfigureResource", func(t *testing.T) {
		attrs, err := structpb.NewStruct(config.AttributeMap{"arg1": "test"})
		test.That(t, err, test.ShouldBeNil)
		gizmoConf.Attributes = attrs

		_, err = m.ReconfigureResource(ctx, &pb.ReconfigureResourceRequest{Config: gizmoConf})
		test.That(t, err, test.ShouldBeNil)

		ret, err := gClient.DoOne(ctx, "test")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ret, test.ShouldBeTrue)

		ret, err = gClient.DoOne(ctx, "")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ret, test.ShouldBeFalse)

		// Test generic echo
		testCmd := map[string]interface{}{"foo": "bar"}
		retCmd, err := gClient.DoCommand(context.Background(), testCmd)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, retCmd, test.ShouldResemble, testCmd)
	})

	t.Run("RemoveResource", func(t *testing.T) {
		_, err = m.RemoveResource(ctx, &pb.RemoveResourceRequest{Name: gizmoConf.Api + "/" + gizmoConf.Name})
		test.That(t, err, test.ShouldBeNil)

		_, err := gClient.DoOne(ctx, "test")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no Gizmo with name (gizmo1)")

		// Test generic echo
		testCmd := map[string]interface{}{"foo": "bar"}
		retCmd, err := gClient.DoCommand(context.Background(), testCmd)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no Gizmo with name (gizmo1)")
		test.That(t, retCmd, test.ShouldBeNil)
	})

	t.Run("Validate", func(t *testing.T) {
		resp, err := m.ValidateConfig(ctx, &pb.ValidateConfigRequest{Config: myBaseConf})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp.Dependencies, test.ShouldNotBeNil)
		test.That(t, resp.Dependencies[0], test.ShouldResemble, "motor1")
		test.That(t, resp.Dependencies[1], test.ShouldResemble, "motor2")

		_, err = m.ValidateConfig(ctx, &pb.ValidateConfigRequest{Config: badMyBaseConf})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldResemble,
			`error validating resource: expected "motorL" attribute for mybase "mybase2"`)
	})

	err = utils.TryClose(ctx, gClient)
	test.That(t, err, test.ShouldBeNil)

	err = conn.Close()
	test.That(t, err, test.ShouldBeNil)

	m.Close(ctx)

	err = myRobot.Close(ctx)
	test.That(t, err, test.ShouldBeNil)
}
