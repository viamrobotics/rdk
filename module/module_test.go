package module_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	v1 "go.viam.com/api/app/v1"
	armpb "go.viam.com/api/component/arm/v1"
	pb "go.viam.com/api/module/v1"
	robotpb "go.viam.com/api/robot/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/components/motor/fake"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	"go.viam.com/rdk/examples/customresources/apis/summationapi"
	"go.viam.com/rdk/examples/customresources/models/mybase"
	"go.viam.com/rdk/examples/customresources/models/mydiscovery"
	"go.viam.com/rdk/examples/customresources/models/mygizmo"
	"go.viam.com/rdk/examples/customresources/models/mysum"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/robot/framesystem"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/robot/server"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/services/discovery"
	"go.viam.com/rdk/services/shell"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

func TestAddModelFromRegistry(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	// Use 'foo.sock' for arbitrary module to test AddModelFromRegistry.
	m, err := module.NewModule(ctx, filepath.Join(t.TempDir(), "foo.sock"), logger)
	test.That(t, err, test.ShouldBeNil)

	invalidModel := resource.NewModel("non", "existent", "model")
	invalidServiceAPI := resource.APINamespace("fake").WithServiceType("nonexistentservice")
	invalidComponentAPI := resource.APINamespace("fake").WithComponentType("nonexistentcomponent")

	validServiceAPI := summationapi.API
	validComponentAPI := gizmoapi.API

	validServiceModel := mysum.Model
	validComponentModel := mygizmo.Model

	resourceError := "resource with API %s and model %s not yet registered"
	testCases := []struct {
		api   resource.API
		model resource.Model
		err   error
	}{
		// Invalid resource APIs and models
		{
			invalidServiceAPI,
			invalidModel,
			fmt.Errorf(resourceError, invalidServiceAPI, invalidModel),
		},
		{
			invalidComponentAPI,
			invalidModel,
			fmt.Errorf(resourceError, invalidComponentAPI, invalidModel),
		},
		// Valid resource APIs and invalid models
		{
			validServiceAPI,
			invalidModel,
			fmt.Errorf(resourceError, validServiceAPI, invalidModel),
		},
		{
			validComponentAPI,
			invalidModel,
			fmt.Errorf(resourceError, validComponentAPI, invalidModel),
		},
		// Mixed validity resource APIs and models
		{
			validServiceAPI,
			validComponentModel,
			fmt.Errorf(resourceError, validServiceAPI, validComponentModel),
		},
		{
			validComponentAPI,
			validServiceModel,
			fmt.Errorf(resourceError, validComponentAPI, validServiceModel),
		},
		// Valid resource APIs and models.
		{
			validServiceAPI,
			validServiceModel,
			nil,
		},
		{
			validComponentAPI,
			validComponentModel,
			nil,
		},
	}

	for _, tc := range testCases {
		tc := tc
		tName := fmt.Sprintf("api: %s, model: %s", tc.api, tc.model)
		t.Run(tName, func(t *testing.T) {
			err := m.AddModelFromRegistry(ctx, tc.api, tc.model)
			if tc.err != nil {
				test.That(t, err, test.ShouldBeError, tc.err)
			} else {
				test.That(t, err, test.ShouldBeNil)
			}
		})
	}
}

func TestModuleFunctions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("todo: get this working on win")
	}
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	gizmoConf := &v1.ComponentConfig{
		Name: "gizmo1", Api: "acme:component:gizmo", Model: "acme:demo:mygizmo",
	}
	myBaseAttrs, err := protoutils.StructToStructPb(mybase.Config{
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

	cfg := &config.Config{Components: []resource.Config{
		{
			Name:                "motor1",
			API:                 resource.NewAPI("rdk", "component", "motor"),
			Model:               resource.DefaultModelFamily.WithModel("fake"),
			ConvertedAttributes: &fake.Config{},
		},
		{
			Name:                "motor2",
			API:                 resource.NewAPI("rdk", "component", "motor"),
			Model:               resource.DefaultModelFamily.WithModel("fake"),
			ConvertedAttributes: &fake.Config{},
		},
	}}

	myRobot, err := robotimpl.RobotFromConfig(ctx, cfg, nil, logger)
	test.That(t, err, test.ShouldBeNil)

	parentAddrs, err := myRobot.ModuleAddresses()
	test.That(t, err, test.ShouldBeNil)

	addr := filepath.ToSlash(filepath.Join(filepath.Dir(parentAddrs.UnixAddr), "mod.sock"))
	m, err := module.NewModule(ctx, addr, logger)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, m.AddModelFromRegistry(ctx, gizmoapi.API, mygizmo.Model), test.ShouldBeNil)
	test.That(t, m.AddModelFromRegistry(ctx, base.API, mybase.Model), test.ShouldBeNil)
	test.That(t, m.AddModelFromRegistry(ctx, discovery.API, mydiscovery.Model), test.ShouldBeNil)

	test.That(t, m.Start(ctx), test.ShouldBeNil)

	//nolint:staticcheck
	conn, err := grpc.Dial(
		"unix://"+addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStreamInterceptor(grpc_retry.StreamClientInterceptor()),
		grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor()),
	)
	test.That(t, err, test.ShouldBeNil)

	client := pb.NewModuleServiceClient(rpc.GrpcOverHTTPClientConn{ClientConn: conn})

	m.SetReady(false)

	resp, err := client.Ready(ctx, &pb.ReadyRequest{ParentAddress: parentAddrs.UnixAddr})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Ready, test.ShouldBeFalse)

	m.SetReady(true)

	resp, err = client.Ready(ctx, &pb.ReadyRequest{ParentAddress: parentAddrs.UnixAddr})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp.Ready, test.ShouldBeTrue)

	t.Run("HandlerMap", func(t *testing.T) {
		// test the raw return
		handlers := resp.GetHandlermap().GetHandlers()
		test.That(t, "acme", test.ShouldBeIn,
			handlers[0].Subtype.Subtype.Namespace, handlers[1].Subtype.Subtype.Namespace, handlers[2].Subtype.Subtype.Namespace)
		test.That(t, "rdk", test.ShouldBeIn,
			handlers[0].Subtype.Subtype.Namespace, handlers[1].Subtype.Subtype.Namespace, handlers[2].Subtype.Subtype.Namespace)
		test.That(t, "component", test.ShouldBeIn,
			handlers[0].Subtype.Subtype.Type, handlers[1].Subtype.Subtype.Type, handlers[2].Subtype.Subtype.Type)
		test.That(t, "service", test.ShouldBeIn,
			handlers[0].Subtype.Subtype.Type, handlers[1].Subtype.Subtype.Type, handlers[2].Subtype.Subtype.Type)
		test.That(t, "gizmo", test.ShouldBeIn,
			handlers[0].Subtype.Subtype.Subtype, handlers[1].Subtype.Subtype.Subtype, handlers[2].Subtype.Subtype.Subtype)
		test.That(t, "base", test.ShouldBeIn,
			handlers[0].Subtype.Subtype.Subtype, handlers[1].Subtype.Subtype.Subtype, handlers[2].Subtype.Subtype.Subtype)
		test.That(t, "discovery", test.ShouldBeIn,
			handlers[0].Subtype.Subtype.Subtype, handlers[1].Subtype.Subtype.Subtype, handlers[2].Subtype.Subtype.Subtype)
		test.That(t, "acme:demo:mygizmo", test.ShouldBeIn,
			handlers[0].GetModels()[0], handlers[1].GetModels()[0], handlers[2].GetModels()[0])
		test.That(t, "acme:demo:mybase", test.ShouldBeIn,
			handlers[0].GetModels()[0], handlers[1].GetModels()[0], handlers[2].GetModels()[0])
		test.That(t, "acme:demo:mydiscovery", test.ShouldBeIn,
			handlers[0].GetModels()[0], handlers[1].GetModels()[0], handlers[2].GetModels()[0])

		// convert from proto
		hmap, err := module.NewHandlerMapFromProto(ctx, resp.GetHandlermap(), rpc.GrpcOverHTTPClientConn{ClientConn: conn})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(hmap), test.ShouldEqual, 3)

		for k, v := range hmap {
			test.That(t, k.API, test.ShouldBeIn, gizmoapi.API, base.API, discovery.API)
			switch k.API {
			case gizmoapi.API:
				test.That(t, mygizmo.Model, test.ShouldResemble, v[0])
			case discovery.API:
				test.That(t, mydiscovery.Model, test.ShouldResemble, v[0])
			default:
				test.That(t, mybase.Model, test.ShouldResemble, v[0])
			}
		}

		// convert back to proto
		handlers2 := hmap.ToProto().GetHandlers()
		test.That(t, "acme", test.ShouldBeIn,
			handlers2[0].Subtype.Subtype.Namespace, handlers2[1].Subtype.Subtype.Namespace, handlers2[2].Subtype.Subtype.Namespace)
		test.That(t, "rdk", test.ShouldBeIn,
			handlers2[0].Subtype.Subtype.Namespace, handlers2[1].Subtype.Subtype.Namespace, handlers2[2].Subtype.Subtype.Namespace)
		test.That(t, "component", test.ShouldBeIn,
			handlers2[0].Subtype.Subtype.Type, handlers2[1].Subtype.Subtype.Type, handlers2[2].Subtype.Subtype.Type)
		test.That(t, "service", test.ShouldBeIn,
			handlers2[0].Subtype.Subtype.Type, handlers2[1].Subtype.Subtype.Type, handlers2[2].Subtype.Subtype.Type)
		test.That(t, "gizmo", test.ShouldBeIn,
			handlers2[0].Subtype.Subtype.Subtype, handlers2[1].Subtype.Subtype.Subtype, handlers2[2].Subtype.Subtype.Subtype)
		test.That(t, "base", test.ShouldBeIn,
			handlers2[0].Subtype.Subtype.Subtype, handlers2[1].Subtype.Subtype.Subtype, handlers2[2].Subtype.Subtype.Subtype)
		test.That(t, "discovery", test.ShouldBeIn,
			handlers2[0].Subtype.Subtype.Subtype, handlers2[1].Subtype.Subtype.Subtype, handlers2[2].Subtype.Subtype.Subtype)
		test.That(t, "acme:demo:mygizmo", test.ShouldBeIn,
			handlers2[0].GetModels()[0], handlers2[1].GetModels()[0], handlers2[2].GetModels()[0])
		test.That(t, "acme:demo:mybase", test.ShouldBeIn,
			handlers2[0].GetModels()[0], handlers2[1].GetModels()[0], handlers2[2].GetModels()[0])
		test.That(t, "acme:demo:mydiscovery", test.ShouldBeIn,
			handlers2[0].GetModels()[0], handlers2[1].GetModels()[0], handlers2[2].GetModels()[0])
	})

	t.Run("GetParentResource", func(t *testing.T) {
		motor1, err := m.GetParentResource(ctx, motor.Named("motor1"))
		test.That(t, err, test.ShouldBeNil)

		rMotor, ok := motor1.(motor.Motor)
		test.That(t, ok, test.ShouldBeTrue)

		err = rMotor.Stop(ctx, nil)
		test.That(t, err, test.ShouldBeNil)

		err = rMotor.Close(ctx)
		test.That(t, err, test.ShouldBeNil)

		// Test that GetParentResource will refresh resources on the parent
		cfg.Components = append(cfg.Components, resource.Config{
			Name:                "motor2",
			API:                 resource.NewAPI("rdk", "component", "motor"),
			Model:               resource.DefaultModelFamily.WithModel("fake"),
			ConvertedAttributes: &fake.Config{},
		})
		myRobot.Reconfigure(ctx, cfg)
		_, err = m.GetParentResource(ctx, motor.Named("motor2"))
		test.That(t, err, test.ShouldBeNil)
	})

	var gClient gizmoapi.Gizmo
	t.Run("AddResource", func(t *testing.T) {
		_, err = m.AddResource(ctx, &pb.AddResourceRequest{Config: gizmoConf})
		test.That(t, err, test.ShouldBeNil)

		gClient = gizmoapi.NewClientFromConn(rpc.GrpcOverHTTPClientConn{ClientConn: conn}, "", gizmoapi.Named("gizmo1"), logger)

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
		attrs, err := structpb.NewStruct(rutils.AttributeMap{"arg1": "test"})
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
		test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

		// Test generic echo
		testCmd := map[string]interface{}{"foo": "bar"}
		retCmd, err := gClient.DoCommand(context.Background(), testCmd)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not found")
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

	err = gClient.Close(ctx)
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

func (c *MockConfig) Validate(path string) ([]string, []string, error) {
	if len(c.Motors) < 1 {
		return nil, nil, errors.New("required attributes 'motors' not specified or empty")
	}
	return c.Motors, nil, nil
}

// TestAttributeConversion tests that modular resource configs have attributes converted with a registered converter,
// and that validation then works on those converted attributes.
func TestAttributeConversion(t *testing.T) {
	type testHarness struct {
		m                    *module.Module
		mockConf             *v1.ComponentConfig
		mockReconfigConf     *v1.ComponentConfig
		createConf1          *resource.Config
		reconfigConf1        *resource.Config
		reconfigConf2        *resource.Config
		createDeps1          *resource.Dependencies
		reconfigDeps1        *resource.Dependencies
		reconfigDeps2        *resource.Dependencies
		modelWithReconfigure resource.Model
	}

	setupTest := func(t *testing.T) (*testHarness, func()) {
		ctx := context.Background()
		logger := logging.NewTestLogger(t)

		cfg := &config.Config{Components: []resource.Config{
			{
				Name:                "motor1",
				API:                 resource.NewAPI("rdk", "component", "motor"),
				Model:               resource.DefaultModelFamily.WithModel("fake"),
				ConvertedAttributes: &fake.Config{},
			},
			{
				Name:                "motor2",
				API:                 resource.NewAPI("rdk", "component", "motor"),
				Model:               resource.DefaultModelFamily.WithModel("fake"),
				ConvertedAttributes: &fake.Config{},
			},
		}}

		myRobot, err := robotimpl.RobotFromConfig(ctx, cfg, nil, logger)
		test.That(t, err, test.ShouldBeNil)

		parentAddrs, err := myRobot.ModuleAddresses()
		test.That(t, err, test.ShouldBeNil)

		addr := filepath.ToSlash(filepath.Join(filepath.Dir(parentAddrs.UnixAddr), "mod.sock"))
		m, err := module.NewModule(ctx, addr, logger)
		test.That(t, err, test.ShouldBeNil)
		model := resource.NewModel("inject", "demo", "shell")
		modelWithReconfigure := resource.NewModel("inject", "demo", "shellWithReconfigure")

		var (
			createConf1, reconfigConf1, reconfigConf2 resource.Config
			createDeps1, reconfigDeps1, reconfigDeps2 resource.Dependencies
		)

		// register the non-reconfigurable one
		resource.RegisterService(shell.API, model, resource.Registration[shell.Service, *MockConfig]{
			Constructor: func(
				ctx context.Context, deps resource.Dependencies, cfg resource.Config, logger logging.Logger,
			) (shell.Service, error) {
				createConf1 = cfg
				createDeps1 = deps
				return &inject.ShellService{}, nil
			},
		})
		test.That(t, m.AddModelFromRegistry(ctx, shell.API, model), test.ShouldBeNil)

		// register the reconfigurable version
		resource.RegisterService(shell.API, modelWithReconfigure, resource.Registration[shell.Service, *MockConfig]{
			Constructor: func(
				ctx context.Context, deps resource.Dependencies, cfg resource.Config, logger logging.Logger,
			) (shell.Service, error) {
				injectable := &inject.ShellService{}
				injectable.ReconfigureFunc = func(ctx context.Context, deps resource.Dependencies, cfg resource.Config) error {
					reconfigConf2 = cfg
					reconfigDeps2 = deps
					return nil
				}
				reconfigConf1 = cfg
				reconfigDeps1 = deps
				return injectable, nil
			},
		})
		test.That(t, m.AddModelFromRegistry(ctx, shell.API, modelWithReconfigure), test.ShouldBeNil)

		test.That(t, m.Start(ctx), test.ShouldBeNil)
		//nolint:staticcheck
		conn, err := grpc.Dial(
			"unix://"+addr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithStreamInterceptor(grpc_retry.StreamClientInterceptor()),
			grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor()),
		)
		test.That(t, err, test.ShouldBeNil)

		client := pb.NewModuleServiceClient(conn)
		m.SetReady(true)
		readyResp, err := client.Ready(ctx, &pb.ReadyRequest{ParentAddress: parentAddrs.UnixAddr})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, readyResp.Ready, test.ShouldBeTrue)

		mockConf := &v1.ComponentConfig{
			Name:  "mymock1",
			Api:   shell.API.String(),
			Model: model.String(),
		}
		mockReconfigConf := &v1.ComponentConfig{
			Name:  "mymock2",
			Api:   shell.API.String(),
			Model: modelWithReconfigure.String(),
		}

		return &testHarness{
				m:                    m,
				mockConf:             mockConf,
				mockReconfigConf:     mockReconfigConf,
				createConf1:          &createConf1,
				reconfigConf1:        &reconfigConf1,
				reconfigConf2:        &reconfigConf2,
				createDeps1:          &createDeps1,
				reconfigDeps1:        &reconfigDeps1,
				reconfigDeps2:        &reconfigDeps2,
				modelWithReconfigure: modelWithReconfigure,
			}, func() {
				resource.Deregister(shell.API, model)
				resource.Deregister(shell.API, modelWithReconfigure)
				test.That(t, conn.Close(), test.ShouldBeNil)
				m.Close(ctx)
				test.That(t, myRobot.Close(ctx), test.ShouldBeNil)
			}
	}

	t.Run("non-reconfigurable creation", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("todo: get this working on win")
		}
		ctx := context.Background()

		th, teardown := setupTest(t)
		defer teardown()

		mockAttrs, err := protoutils.StructToStructPb(MockConfig{
			Motors: []string{motor.Named("motor1").String()},
		})
		test.That(t, err, test.ShouldBeNil)

		th.mockConf.Attributes = mockAttrs

		validateResp, err := th.m.ValidateConfig(ctx, &pb.ValidateConfigRequest{
			Config: th.mockConf,
		})
		test.That(t, err, test.ShouldBeNil)

		deps := validateResp.Dependencies
		test.That(t, deps, test.ShouldResemble, []string{"rdk:component:motor/motor1"})

		_, err = th.m.AddResource(ctx, &pb.AddResourceRequest{
			Config:       th.mockConf,
			Dependencies: deps,
		})
		test.That(t, err, test.ShouldBeNil)

		_, ok := (*th.createDeps1)[motor.Named("motor1")]
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, th.createConf1.Attributes.StringSlice("motors"), test.ShouldResemble, []string{motor.Named("motor1").String()})

		mc, ok := th.createConf1.ConvertedAttributes.(*MockConfig)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, mc.Motors, test.ShouldResemble, []string{motor.Named("motor1").String()})
	})

	t.Run("non-reconfigurable recreation", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("todo: get this working on win")
		}
		ctx := context.Background()

		th, teardown := setupTest(t)
		defer teardown()

		mockAttrs, err := protoutils.StructToStructPb(MockConfig{
			Motors: []string{motor.Named("motor1").String()},
		})
		test.That(t, err, test.ShouldBeNil)

		th.mockConf.Attributes = mockAttrs

		validateResp, err := th.m.ValidateConfig(ctx, &pb.ValidateConfigRequest{
			Config: th.mockConf,
		})
		test.That(t, err, test.ShouldBeNil)

		deps := validateResp.Dependencies
		test.That(t, deps, test.ShouldResemble, []string{"rdk:component:motor/motor1"})

		_, err = th.m.AddResource(ctx, &pb.AddResourceRequest{
			Config:       th.mockConf,
			Dependencies: deps,
		})
		test.That(t, err, test.ShouldBeNil)

		mockAttrs, err = protoutils.StructToStructPb(MockConfig{
			Motors: []string{motor.Named("motor2").String()},
		})
		test.That(t, err, test.ShouldBeNil)

		th.mockConf.Attributes = mockAttrs

		validateResp, err = th.m.ValidateConfig(ctx, &pb.ValidateConfigRequest{
			Config: th.mockConf,
		})
		test.That(t, err, test.ShouldBeNil)

		deps = validateResp.Dependencies
		test.That(t, deps, test.ShouldResemble, []string{"rdk:component:motor/motor2"})

		_, err = th.m.ReconfigureResource(ctx, &pb.ReconfigureResourceRequest{
			Config:       th.mockConf,
			Dependencies: deps,
		})
		test.That(t, err, test.ShouldBeNil)

		_, ok := (*th.createDeps1)[motor.Named("motor2")]
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, th.createConf1.Attributes.StringSlice("motors"), test.ShouldResemble, []string{motor.Named("motor2").String()})

		mc, ok := th.createConf1.ConvertedAttributes.(*MockConfig)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, mc.Motors, test.ShouldResemble, []string{motor.Named("motor2").String()})
	})

	t.Run("reconfigurable creation", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("todo: get this working on win")
		}
		ctx := context.Background()

		th, teardown := setupTest(t)
		defer teardown()

		mockAttrs, err := protoutils.StructToStructPb(MockConfig{
			Motors: []string{motor.Named("motor1").String()},
		})
		test.That(t, err, test.ShouldBeNil)

		th.mockReconfigConf.Attributes = mockAttrs
		th.mockReconfigConf.Model = th.modelWithReconfigure.String()

		validateResp, err := th.m.ValidateConfig(ctx, &pb.ValidateConfigRequest{
			Config: th.mockReconfigConf,
		})
		test.That(t, err, test.ShouldBeNil)

		deps := validateResp.Dependencies
		test.That(t, deps, test.ShouldResemble, []string{"rdk:component:motor/motor1"})

		_, err = th.m.AddResource(ctx, &pb.AddResourceRequest{
			Config:       th.mockReconfigConf,
			Dependencies: deps,
		})
		test.That(t, err, test.ShouldBeNil)

		_, ok := (*th.reconfigDeps1)[motor.Named("motor1")]
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, th.reconfigConf1.Attributes.StringSlice("motors"), test.ShouldResemble, []string{motor.Named("motor1").String()})

		mc, ok := th.reconfigConf1.ConvertedAttributes.(*MockConfig)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, mc.Motors, test.ShouldResemble, []string{motor.Named("motor1").String()})
	})

	// also check that associated resource configs are processed correctly
	t.Run("reconfigurable reconfiguration", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("todo: get this working on win")
		}
		ctx := context.Background()

		th, teardown := setupTest(t)
		defer teardown()

		mockAttrs, err := protoutils.StructToStructPb(MockConfig{
			Motors: []string{motor.Named("motor1").String()},
		})
		test.That(t, err, test.ShouldBeNil)

		th.mockReconfigConf.Attributes = mockAttrs

		// TODO(RSDK-6022): use datamanager.DataCaptureConfigs once resource.Name can be properly marshalled/unmarshalled
		dummySvcCfg := rutils.AttributeMap{
			"capture_methods": []interface{}{map[string]interface{}{"name": "rdk:service:shell/mymock2", "method": "Something"}},
		}
		mockServiceCfg, err := protoutils.StructToStructPb(dummySvcCfg)
		test.That(t, err, test.ShouldBeNil)

		th.mockReconfigConf.ServiceConfigs = append(th.mockReconfigConf.ServiceConfigs, &v1.ResourceLevelServiceConfig{
			Type:       datamanager.API.String(),
			Attributes: mockServiceCfg,
		})

		th.mockReconfigConf.Model = th.modelWithReconfigure.String()

		validateResp, err := th.m.ValidateConfig(ctx, &pb.ValidateConfigRequest{
			Config: th.mockReconfigConf,
		})
		test.That(t, err, test.ShouldBeNil)

		deps := validateResp.Dependencies
		test.That(t, deps, test.ShouldResemble, []string{"rdk:component:motor/motor1"})

		_, err = th.m.AddResource(ctx, &pb.AddResourceRequest{
			Config:       th.mockReconfigConf,
			Dependencies: deps,
		})
		test.That(t, err, test.ShouldBeNil)

		mockAttrs, err = protoutils.StructToStructPb(MockConfig{
			Motors: []string{motor.Named("motor2").String()},
		})
		test.That(t, err, test.ShouldBeNil)

		th.mockReconfigConf.Attributes = mockAttrs

		dummySvcCfg2 := rutils.AttributeMap{
			"capture_methods": []interface{}{map[string]interface{}{"name": "rdk:service:shell/mymock2", "method": "Something2"}},
		}
		mockServiceCfg, err = protoutils.StructToStructPb(dummySvcCfg2)
		test.That(t, err, test.ShouldBeNil)

		th.mockReconfigConf.ServiceConfigs = append([]*v1.ResourceLevelServiceConfig{}, &v1.ResourceLevelServiceConfig{
			Type:       datamanager.API.String(),
			Attributes: mockServiceCfg,
		})

		validateResp, err = th.m.ValidateConfig(ctx, &pb.ValidateConfigRequest{
			Config: th.mockReconfigConf,
		})
		test.That(t, err, test.ShouldBeNil)

		deps = validateResp.Dependencies
		test.That(t, deps, test.ShouldResemble, []string{"rdk:component:motor/motor2"})

		_, err = th.m.ReconfigureResource(ctx, &pb.ReconfigureResourceRequest{
			Config:       th.mockReconfigConf,
			Dependencies: deps,
		})
		test.That(t, err, test.ShouldBeNil)

		_, ok := (*th.reconfigDeps2)[motor.Named("motor2")]
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, th.reconfigConf2.Attributes.StringSlice("motors"), test.ShouldResemble, []string{motor.Named("motor2").String()})

		mc, ok := th.reconfigConf2.ConvertedAttributes.(*MockConfig)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, mc.Motors, test.ShouldResemble, []string{motor.Named("motor2").String()})

		svcCfg := th.reconfigConf2.AssociatedResourceConfigs[0].Attributes
		test.That(t, svcCfg, test.ShouldResemble, dummySvcCfg2)

		convSvcCfg, ok := th.reconfigConf2.AssociatedResourceConfigs[0].ConvertedAttributes.(*datamanager.AssociatedConfig)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, convSvcCfg.CaptureMethods[0].Name, test.ShouldResemble, shell.Named("mymock2"))
		test.That(t, convSvcCfg.CaptureMethods[0].Method, test.ShouldEqual, "Something2")

		// and as a final confirmation, check that original values weren't modified
		_, ok = (*th.reconfigDeps1)[motor.Named("motor1")]
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, th.reconfigConf1.Attributes.StringSlice("motors"), test.ShouldResemble, []string{motor.Named("motor1").String()})

		mc, ok = th.reconfigConf1.ConvertedAttributes.(*MockConfig)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, mc.Motors, test.ShouldResemble, []string{motor.Named("motor1").String()})

		svcCfg = th.reconfigConf1.AssociatedResourceConfigs[0].Attributes
		test.That(t, svcCfg, test.ShouldResemble, dummySvcCfg)

		convSvcCfg, ok = th.reconfigConf1.AssociatedResourceConfigs[0].ConvertedAttributes.(*datamanager.AssociatedConfig)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, convSvcCfg.CaptureMethods[0].Name, test.ShouldResemble, shell.Named("mymock2"))
		test.That(t, convSvcCfg.CaptureMethods[0].Method, test.ShouldEqual, "Something")
	})
}

// setupLocalModule sets up a module without a parent connection.
func setupLocalModule(t *testing.T, ctx context.Context, logger logging.Logger) *module.Module {
	t.Helper()

	// Use 'foo.sock' for arbitrary module to test AddModelFromRegistry.
	m, err := module.NewModule(ctx, filepath.Join(t.TempDir(), "foo.sock"), logger)
	test.That(t, err, test.ShouldBeNil)
	t.Cleanup(func() {
		m.Close(ctx)
	})

	// Hit Ready as a way to close m.pcFailed, so that AddResource can proceed. Set NoModuleParentEnvVar so that parent connection
	// will not be attempted.
	test.That(t, os.Setenv(module.NoModuleParentEnvVar, "true"), test.ShouldBeNil)
	t.Cleanup(func() {
		test.That(t, os.Unsetenv(module.NoModuleParentEnvVar), test.ShouldBeNil)
	})

	_, err = m.Ready(ctx, &pb.ReadyRequest{})
	test.That(t, err, test.ShouldBeNil)
	return m
}

// TestModuleAddResource tests that modular resources gets closed on add if context is done before resource finished creation.
func TestModuleAddResource(t *testing.T) {
	type testHarness struct {
		ctx context.Context

		m              *module.Module
		cfg            *v1.ComponentConfig
		constructCount int
		closeCount     int
	}

	setupTest := func(t *testing.T, shouldCancel bool) *testHarness {
		var th testHarness

		ctx, cancelFunc := context.WithCancel(context.Background())
		t.Cleanup(func() { cancelFunc() })
		th.ctx = ctx

		modelName := utils.RandomAlphaString(5)
		model := resource.DefaultModelFamily.WithModel(modelName)

		resource.RegisterService(shell.API, model, resource.Registration[shell.Service, *MockConfig]{
			Constructor: func(
				ctx context.Context, deps resource.Dependencies, cfg resource.Config, logger logging.Logger,
			) (shell.Service, error) {
				th.constructCount++
				if shouldCancel {
					cancelFunc()
					<-ctx.Done()
				}
				return &inject.ShellService{
					CloseFunc: func(ctx context.Context) error { th.closeCount++; return nil },
				}, nil
			},
		})
		t.Cleanup(func() {
			resource.Deregister(shell.API, model)
		})

		th.m = setupLocalModule(t, ctx, logging.NewTestLogger(t))
		test.That(t, th.m.AddModelFromRegistry(ctx, shell.API, model), test.ShouldBeNil)

		th.cfg = &v1.ComponentConfig{Name: "mymock", Api: shell.API.String(), Model: model.String()}
		return &th
	}

	t.Run("add resource normally", func(t *testing.T) {
		th := setupTest(t, false)
		_, err := th.m.AddResource(th.ctx, &pb.AddResourceRequest{Config: th.cfg})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, th.constructCount, test.ShouldEqual, 1)
		test.That(t, th.closeCount, test.ShouldEqual, 0)
	})

	t.Run("cancel ctx during resource add", func(t *testing.T) {
		th := setupTest(t, true)
		_, err := th.m.AddResource(th.ctx, &pb.AddResourceRequest{Config: th.cfg})
		test.That(t, err, test.ShouldBeError, context.Canceled)
		test.That(t, th.constructCount, test.ShouldEqual, 1)
		test.That(t, th.closeCount, test.ShouldEqual, 1)
	})
}

func TestModuleSocketAddrTruncation(t *testing.T) {
	// correct path on windows
	fixPath := func(path string) string { return strings.ReplaceAll(path, "/", string(filepath.Separator)) }

	// test with a short base path
	path, err := module.CreateSocketAddress("/tmp", "my-cool-module")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, path, test.ShouldEqual, fixPath("/tmp/my-cool-module.sock"))

	// test exactly 103
	path, err = module.CreateSocketAddress(
		"/tmp",
		// 103 - len("/tmp/") - len(".sock")
		strings.Repeat("a", 93),
	)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, path, test.ShouldHaveLength, 103)
	test.That(t, path, test.ShouldEqual,
		fixPath("/tmp/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.sock"),
	)

	// test 104 chars
	path, err = module.CreateSocketAddress(
		"/tmp",
		// 103 - len("/tmp/") - len(".sock") + 1 more character to trigger truncation
		strings.Repeat("a", 94),
	)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, path, test.ShouldHaveLength, 103)
	// test that creating a new socket address with the same name produces the same truncated address
	test.That(t, path, test.ShouldEndWith, "-QUEUU.sock")

	// test with an extra-long base path
	_, err = module.CreateSocketAddress(
		// 103 - len("/a.sock") + 1 more character to trigger truncation
		strings.Repeat("a", 98),
		"a",
	)
	test.That(t, fmt.Sprint(err), test.ShouldContainSubstring, "module socket base path")
}

func TestNewFrameSystemClient(t *testing.T) {
	logger := logging.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()

	testAPI := resource.APINamespaceRDK.WithComponentType(arm.SubtypeName)

	testName := resource.NewName(testAPI, "arm1")

	expectedInputs := referenceframe.FrameSystemInputs{
		testName.ShortName(): []referenceframe.Input{{0}, {math.Pi}, {-math.Pi}, {0}, {math.Pi}, {-math.Pi}},
	}
	injectArm := &inject.Arm{
		JointPositionsFunc: func(ctx context.Context, extra map[string]any) ([]referenceframe.Input, error) {
			return expectedInputs[testName.ShortName()], nil
		},
		KinematicsFunc: func(ctx context.Context) (referenceframe.Model, error) {
			return referenceframe.ParseModelJSONFile(rutils.ResolveFile("components/arm/example_kinematics/ur5e.json"), "")
		},
	}

	resourceNames := []resource.Name{testName}
	resources := map[resource.Name]arm.Arm{testName: injectArm}
	injectRobot := &inject.Robot{
		ResourceNamesFunc:  func() []resource.Name { return resourceNames },
		ResourceByNameFunc: func(n resource.Name) (resource.Resource, error) { return resources[n], nil },
		MachineStatusFunc: func(ctx context.Context) (robot.MachineStatus, error) {
			return robot.MachineStatus{State: robot.StateRunning}, nil
		},
		ResourceRPCAPIsFunc: func() []resource.RPCAPI { return nil },
	}

	armSvc, err := resource.NewAPIResourceCollection(arm.API, resources)
	test.That(t, err, test.ShouldBeNil)
	gServer.RegisterService(&armpb.ArmService_ServiceDesc, arm.NewRPCServiceServer(armSvc))
	robotpb.RegisterRobotServiceServer(gServer, server.New(injectRobot))

	go gServer.Serve(listener)
	defer gServer.Stop()

	client, err := client.New(context.Background(), listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, client.Close(context.Background()), test.ShouldBeNil)
	}()

	inputs, err := client.CurrentInputs(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(inputs), test.ShouldEqual, 1)
	test.That(t, inputs, test.ShouldResemble, expectedInputs)

	fsc := module.NewFrameSystemClient(client)
	fsCurrentInputs, err := fsc.CurrentInputs(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fsCurrentInputs, test.ShouldResemble, inputs)
}

func TestFrameSystemFromDependencies(t *testing.T) {
	type testHarness struct {
		ctx context.Context

		m              *module.Module
		cfg            *v1.ComponentConfig
		constructCount int
	}

	var th testHarness

	ctx, cancelFunc := context.WithCancel(context.Background())
	t.Cleanup(func() { cancelFunc() })
	th.ctx = ctx

	modelName := utils.RandomAlphaString(5)
	model := resource.DefaultModelFamily.WithModel(modelName)

	resource.RegisterService(shell.API, model, resource.Registration[shell.Service, *MockConfig]{
		Constructor: func(
			ctx context.Context, deps resource.Dependencies, cfg resource.Config, logger logging.Logger,
		) (shell.Service, error) {
			th.constructCount++
			fsc, err := framesystem.FromDependencies(deps)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, fsc.Name(), test.ShouldResemble, framesystem.PublicServiceName)

			return &inject.ShellService{}, nil
		},
	})
	t.Cleanup(func() {
		resource.Deregister(shell.API, model)
	})

	th.m = setupLocalModule(t, ctx, logging.NewTestLogger(t))
	test.That(t, th.m.AddModelFromRegistry(ctx, shell.API, model), test.ShouldBeNil)

	th.cfg = &v1.ComponentConfig{Name: "mymock", Api: shell.API.String(), Model: model.String()}

	_, err := th.m.AddResource(th.ctx, &pb.AddResourceRequest{Config: th.cfg})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, th.constructCount, test.ShouldEqual, 1)
}
