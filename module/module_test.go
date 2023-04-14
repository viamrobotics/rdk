package module_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/edaniels/golog"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"go.uber.org/zap"
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
	"go.viam.com/rdk/examples/customresources/apis/summationapi"
	"go.viam.com/rdk/examples/customresources/models/mybase"
	"go.viam.com/rdk/examples/customresources/models/mygizmo"
	"go.viam.com/rdk/examples/customresources/models/mysum"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/services/shell"
	"go.viam.com/rdk/testutils/inject"
)

func TestAddModelFromRegistry(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	// Use 'foo.sock' for arbitrary module to test AddModelFromRegistry.
	m, err := module.NewModule(ctx, filepath.Join(t.TempDir(), "foo.sock"), logger)
	test.That(t, err, test.ShouldBeNil)

	invalidModel := resource.NewModel("non", "existent", "model")
	invalidServiceSubtype := resource.NewSubtype(
		resource.Namespace("fake"),
		resource.ResourceTypeService,
		resource.SubtypeName("nonexistentservice"))
	invalidComponentSubtype := resource.NewSubtype(
		resource.Namespace("fake"),
		resource.ResourceTypeComponent,
		resource.SubtypeName("nonexistentcomponent"))

	validServiceSubtype := summationapi.Subtype
	validComponentSubtype := gizmoapi.Subtype

	validServiceModel := mysum.Model
	validComponentModel := mygizmo.Model

	componentError := "component with API %s and model %s not yet registered"
	serviceError := "service with API %s and model %s not yet registered"
	testCases := []struct {
		subtype resource.Subtype
		model   resource.Model
		err     error
	}{
		// Invalid resource subtypes and models
		{
			invalidServiceSubtype,
			invalidModel,
			fmt.Errorf(serviceError, invalidServiceSubtype, invalidModel),
		},
		{
			invalidComponentSubtype,
			invalidModel,
			fmt.Errorf(componentError, invalidComponentSubtype, invalidModel),
		},
		// Valid resource subtypes and invalid models
		{
			validServiceSubtype,
			invalidModel,
			fmt.Errorf(serviceError, validServiceSubtype, invalidModel),
		},
		{
			validComponentSubtype,
			invalidModel,
			fmt.Errorf(componentError, validComponentSubtype, invalidModel),
		},
		// Mixed validity resource subtypes and models
		{
			validServiceSubtype,
			validComponentModel,
			fmt.Errorf(serviceError, validServiceSubtype, validComponentModel),
		},
		{
			validComponentSubtype,
			validServiceModel,
			fmt.Errorf(componentError, validComponentSubtype, validServiceModel),
		},
		// Valid resource subtypes and models.
		{
			validServiceSubtype,
			validServiceModel,
			nil,
		},
		{
			validComponentSubtype,
			validComponentModel,
			nil,
		},
	}

	for _, tc := range testCases {
		tc := tc
		tName := fmt.Sprintf("subtype: %s, model: %s", tc.subtype, tc.model)
		t.Run(tName, func(t *testing.T) {
			err := m.AddModelFromRegistry(ctx, tc.subtype, tc.model)
			if tc.err != nil {
				test.That(t, err, test.ShouldBeError, tc.err)
			} else {
				test.That(t, err, test.ShouldBeNil)
			}
		})
	}
}

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

type MockConfig struct {
	Motors []string `json:"motors"`
}

func (c *MockConfig) Validate(path string) ([]string, error) {
	if len(c.Motors) < 1 {
		return nil, errors.New("required attributes 'motors' not specified or empty")
	}
	return c.Motors, nil
}

// TestAttributeConversion tests that modular resource configs have attributes converted with a registred converter,
// and that validation then works on those converted attributes.
func TestAttributeConversion(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

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
	model := resource.NewModel("inject", "demo", "shell")
	modelWithReconfigure := resource.NewModel("inject", "demo", "shellWithReconfigure")

	var (
		createConf1, reconfigConf1, reconfigConf2 config.Service
		createDeps1, reconfigDeps1, reconfigDeps2 registry.Dependencies
	)

	// register the non-reconfigurable one
	registry.RegisterService(shell.Subtype, model, registry.Service{
		Constructor: func(ctx context.Context, deps registry.Dependencies, cfg config.Service, logger *zap.SugaredLogger) (interface{}, error) {
			createConf1 = cfg
			createDeps1 = deps
			return &inject.ShellService{}, nil
		},
	})
	defer func() {
		registry.DeregisterService(shell.Subtype, model)
	}()

	config.RegisterServiceAttributeMapConverter(
		shell.Subtype,
		model,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf MockConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&MockConfig{},
	)
	test.That(t, m.AddModelFromRegistry(ctx, shell.Subtype, model), test.ShouldBeNil)

	// register the reconfigurable version
	registry.RegisterService(shell.Subtype, modelWithReconfigure, registry.Service{
		Constructor: func(ctx context.Context, deps registry.Dependencies, cfg config.Service, logger *zap.SugaredLogger) (interface{}, error) {
			injectable := &inject.ShellServiceWithReconfigure{}
			injectable.ReconfigureFunc = func(ctx context.Context, cfg config.Service, deps registry.Dependencies) error {
				reconfigConf2 = cfg
				reconfigDeps2 = deps
				return nil
			}
			reconfigConf1 = cfg
			reconfigDeps1 = deps
			return injectable, nil
		},
	})
	defer func() {
		// Deregister the mock summation service
		registry.DeregisterService(shell.Subtype, modelWithReconfigure)
	}()

	config.RegisterServiceAttributeMapConverter(
		shell.Subtype,
		modelWithReconfigure,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf MockConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&MockConfig{},
	)
	test.That(t, m.AddModelFromRegistry(ctx, shell.Subtype, modelWithReconfigure), test.ShouldBeNil)

	test.That(t, m.Start(ctx), test.ShouldBeNil)
	conn, err := grpc.Dial(
		"unix://"+addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStreamInterceptor(grpc_retry.StreamClientInterceptor()),
		grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor()),
	)
	test.That(t, err, test.ShouldBeNil)

	client := pb.NewModuleServiceClient(conn)
	m.SetReady(true)
	readyResp, err := client.Ready(ctx, &pb.ReadyRequest{ParentAddress: parentAddr})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, readyResp.Ready, test.ShouldBeTrue)

	mockConf := &v1.ComponentConfig{
		Name:  "mymock1",
		Api:   shell.Subtype.String(),
		Model: model.String(),
	}
	mockReconfigConf := &v1.ComponentConfig{
		Name:  "mymock2",
		Api:   shell.Subtype.String(),
		Model: modelWithReconfigure.String(),
	}

	//nolint:dupl
	t.Run("non-reconfigurable creation", func(t *testing.T) {
		mockAttrs, err := protoutils.StructToStructPb(MockConfig{
			Motors: []string{motor.Named("motor1").String()},
		})
		test.That(t, err, test.ShouldBeNil)

		mockConf.Attributes = mockAttrs

		validateResp, err := m.ValidateConfig(ctx, &pb.ValidateConfigRequest{
			Config: mockConf,
		})
		test.That(t, err, test.ShouldBeNil)

		deps := validateResp.Dependencies
		test.That(t, deps, test.ShouldResemble, []string{"rdk:component:motor/motor1"})

		_, err = m.AddResource(ctx, &pb.AddResourceRequest{
			Config: mockConf, Dependencies: deps,
		})
		test.That(t, err, test.ShouldBeNil)

		_, ok := createDeps1[motor.Named("motor1")]
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, createConf1.Attributes.StringSlice("motors"), test.ShouldResemble, []string{motor.Named("motor1").String()})

		mc, ok := createConf1.ConvertedAttributes.(*MockConfig)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, mc.Motors, test.ShouldResemble, []string{motor.Named("motor1").String()})
	})

	//nolint:dupl
	t.Run("non-reconfigurable recreation", func(t *testing.T) {
		mockAttrs, err := protoutils.StructToStructPb(MockConfig{
			Motors: []string{motor.Named("motor2").String()},
		})
		test.That(t, err, test.ShouldBeNil)

		mockConf.Attributes = mockAttrs

		validateResp, err := m.ValidateConfig(ctx, &pb.ValidateConfigRequest{
			Config: mockConf,
		})
		test.That(t, err, test.ShouldBeNil)

		deps := validateResp.Dependencies
		test.That(t, deps, test.ShouldResemble, []string{"rdk:component:motor/motor2"})

		_, err = m.ReconfigureResource(ctx, &pb.ReconfigureResourceRequest{
			Config: mockConf, Dependencies: deps,
		})
		test.That(t, err, test.ShouldBeNil)

		_, ok := createDeps1[motor.Named("motor2")]
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, createConf1.Attributes.StringSlice("motors"), test.ShouldResemble, []string{motor.Named("motor2").String()})

		mc, ok := createConf1.ConvertedAttributes.(*MockConfig)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, mc.Motors, test.ShouldResemble, []string{motor.Named("motor2").String()})
	})

	t.Run("reconfigurable creation", func(t *testing.T) {
		mockAttrs, err := protoutils.StructToStructPb(MockConfig{
			Motors: []string{motor.Named("motor1").String()},
		})
		test.That(t, err, test.ShouldBeNil)

		mockReconfigConf.Attributes = mockAttrs
		mockReconfigConf.Model = modelWithReconfigure.String()

		validateResp, err := m.ValidateConfig(ctx, &pb.ValidateConfigRequest{
			Config: mockReconfigConf,
		})
		test.That(t, err, test.ShouldBeNil)

		deps := validateResp.Dependencies
		test.That(t, deps, test.ShouldResemble, []string{"rdk:component:motor/motor1"})

		_, err = m.AddResource(ctx, &pb.AddResourceRequest{
			Config: mockReconfigConf, Dependencies: deps,
		})
		test.That(t, err, test.ShouldBeNil)

		_, ok := reconfigDeps1[motor.Named("motor1")]
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, reconfigConf1.Attributes.StringSlice("motors"), test.ShouldResemble, []string{motor.Named("motor1").String()})

		mc, ok := reconfigConf1.ConvertedAttributes.(*MockConfig)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, mc.Motors, test.ShouldResemble, []string{motor.Named("motor1").String()})
	})

	t.Run("reconfigurable reconfiguration", func(t *testing.T) {
		mockAttrs, err := protoutils.StructToStructPb(MockConfig{
			Motors: []string{motor.Named("motor2").String()},
		})
		test.That(t, err, test.ShouldBeNil)

		mockReconfigConf.Attributes = mockAttrs

		validateResp, err := m.ValidateConfig(ctx, &pb.ValidateConfigRequest{
			Config: mockReconfigConf,
		})
		test.That(t, err, test.ShouldBeNil)

		deps := validateResp.Dependencies
		test.That(t, deps, test.ShouldResemble, []string{"rdk:component:motor/motor2"})

		_, err = m.ReconfigureResource(ctx, &pb.ReconfigureResourceRequest{
			Config: mockReconfigConf, Dependencies: deps,
		})
		test.That(t, err, test.ShouldBeNil)

		_, ok := reconfigDeps2[motor.Named("motor2")]
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, reconfigConf2.Attributes.StringSlice("motors"), test.ShouldResemble, []string{motor.Named("motor2").String()})

		mc, ok := reconfigConf2.ConvertedAttributes.(*MockConfig)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, mc.Motors, test.ShouldResemble, []string{motor.Named("motor2").String()})

		// and as a final confirmation, check that original values weren't modified
		_, ok = reconfigDeps1[motor.Named("motor1")]
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, reconfigConf1.Attributes.StringSlice("motors"), test.ShouldResemble, []string{motor.Named("motor1").String()})

		mc, ok = reconfigConf1.ConvertedAttributes.(*MockConfig)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, mc.Motors, test.ShouldResemble, []string{motor.Named("motor1").String()})
	})

	err = conn.Close()
	test.That(t, err, test.ShouldBeNil)

	m.Close(ctx)

	err = myRobot.Close(ctx)
	test.That(t, err, test.ShouldBeNil)
}
