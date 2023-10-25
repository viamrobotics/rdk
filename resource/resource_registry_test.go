package resource_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jhump/protoreflect/grpcreflect"
	pb "go.viam.com/api/robot/v1"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/arm/fake"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/utils"
)

var (
	button      = "button"
	acme        = resource.NewName(resource.APINamespace("acme").WithComponentType(button), "button1")
	nav         = "navigation"
	testService = resource.NewName(resource.APINamespaceRDK.WithComponentType(nav), "nav1")
)

func TestComponentRegistry(t *testing.T) {
	logger := logging.NewTestLogger(t)
	rf := func(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger) (arm.Arm, error) {
		return &fake.Arm{Named: conf.ResourceName().AsNamed()}, nil
	}
	model := resource.Model{Name: "x"}
	test.That(t, func() {
		resource.Register(acme.API, model, resource.Registration[arm.Arm, resource.NoNativeConfig]{})
	}, test.ShouldPanic)
	resource.Register(acme.API, model, resource.Registration[arm.Arm, resource.NoNativeConfig]{Constructor: rf})

	resInfo, ok := resource.LookupRegistration(acme.API, model)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resInfo, test.ShouldNotBeNil)
	_, ok = resource.LookupRegistration(acme.API, resource.Model{Name: "z"})
	test.That(t, ok, test.ShouldBeFalse)
	res, err := resInfo.Constructor(context.Background(), nil, resource.Config{Name: "foo"}, logger)
	test.That(t, err, test.ShouldBeNil)
	resArm, err := resource.AsType[arm.Arm](res)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resArm.Name().Name, test.ShouldEqual, "foo")

	resource.Deregister(acme.API, model)
	_, ok = resource.LookupRegistration(acme.API, model)
	test.That(t, ok, test.ShouldBeFalse)

	modelName2 := resource.DefaultServiceModel
	test.That(t, func() {
		resource.Register(testService.API, modelName2, resource.Registration[arm.Arm, resource.NoNativeConfig]{})
	}, test.ShouldPanic)
	resource.Register(testService.API, modelName2, resource.Registration[arm.Arm, resource.NoNativeConfig]{Constructor: rf})

	resInfo, ok = resource.LookupRegistration(testService.API, modelName2)
	test.That(t, resInfo, test.ShouldNotBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	_, ok = resource.LookupRegistration(testService.API, resource.DefaultModelFamily.WithModel("z"))
	test.That(t, ok, test.ShouldBeFalse)
	res, err = resInfo.Constructor(context.Background(), nil, resource.Config{Name: "bar"}, logger)
	test.That(t, err, test.ShouldBeNil)
	resArm, err = resource.AsType[arm.Arm](res)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resArm.Name().Name, test.ShouldEqual, "bar")

	resource.Deregister(testService.API, modelName2)
	_, ok = resource.LookupRegistration(testService.API, modelName2)
	test.That(t, ok, test.ShouldBeFalse)
}

func TestResourceAPIRegistry(t *testing.T) {
	statf := func(context.Context, arm.Arm) (interface{}, error) {
		return nil, errors.New("one")
	}
	var capColl resource.APIResourceCollection[arm.Arm]

	sf := func(apiResColl resource.APIResourceCollection[arm.Arm]) interface{} {
		capColl = apiResColl
		return 5
	}
	rcf := func(_ context.Context, _ rpc.ClientConn, _ string, name resource.Name, _ logging.Logger) (arm.Arm, error) {
		return capColl.Resource(name.ShortName())
	}

	test.That(t, func() {
		resource.RegisterAPI(acme.API, resource.APIRegistration[arm.Arm]{
			Status:                      statf,
			RPCServiceServerConstructor: sf,
			RPCServiceDesc:              &pb.RobotService_ServiceDesc,
		})
	}, test.ShouldPanic)
	test.That(t, func() {
		resource.RegisterAPIWithAssociation(acme.API, resource.APIRegistration[arm.Arm]{
			Status:                      statf,
			RPCServiceServerConstructor: sf,
			RPCServiceDesc:              &pb.RobotService_ServiceDesc,
		}, resource.AssociatedConfigRegistration[resource.AssociatedNameUpdater]{})
	}, test.ShouldPanic)
	resource.RegisterAPI(acme.API, resource.APIRegistration[arm.Arm]{
		Status:                      statf,
		RPCServiceServerConstructor: sf,
		RPCServiceHandler:           pb.RegisterRobotServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.RobotService_ServiceDesc,
	})
	apiInfo, ok, err := resource.LookupAPIRegistration[arm.Arm](acme.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, apiInfo, test.ShouldNotBeNil)
	_, err = apiInfo.Status(nil, &fake.Arm{Named: arm.Named("foo").AsNamed()})
	test.That(t, err, test.ShouldBeError, errors.New("one"))
	coll, err := resource.NewAPIResourceCollection(arm.API, map[resource.Name]arm.Arm{
		arm.Named("foo"): &fake.Arm{Named: arm.Named("foo").AsNamed()},
	})
	test.That(t, err, test.ShouldBeNil)
	svcServer := apiInfo.RPCServiceServerConstructor(coll)
	test.That(t, svcServer, test.ShouldNotBeNil)
	test.That(t, apiInfo.RPCClient, test.ShouldBeNil)

	api2 := resource.APINamespace("acme2").WithComponentType(button)
	_, ok, err = resource.LookupAPIRegistration[arm.Arm](api2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeFalse)

	resource.RegisterAPI(api2, resource.APIRegistration[arm.Arm]{
		RPCServiceServerConstructor: sf,
		RPCClient:                   rcf,
		RPCServiceDesc:              &pb.RobotService_ServiceDesc,
		RPCServiceHandler:           pb.RegisterRobotServiceHandlerFromEndpoint,
	})
	apiInfo, ok, err = resource.LookupAPIRegistration[arm.Arm](api2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, apiInfo.Status, test.ShouldBeNil)
	svcServer = apiInfo.RPCServiceServerConstructor(coll)
	test.That(t, svcServer, test.ShouldNotBeNil)
	res, err := apiInfo.RPCClient(nil, nil, "", arm.Named("foo"), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res.Name().Name, test.ShouldEqual, "foo")
	test.That(t, apiInfo.RPCServiceDesc, test.ShouldEqual, &pb.RobotService_ServiceDesc)

	reflectSvcDesc, err := grpcreflect.LoadServiceDescriptor(apiInfo.RPCServiceDesc)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, apiInfo.ReflectRPCServiceDesc, test.ShouldResemble, reflectSvcDesc)

	api3 := resource.APINamespace("acme3").WithComponentType(button)
	_, ok, err = resource.LookupAPIRegistration[arm.Arm](api3)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeFalse)

	resource.RegisterAPI(api3, resource.APIRegistration[arm.Arm]{RPCClient: rcf})
	apiInfo, ok, err = resource.LookupAPIRegistration[arm.Arm](api3)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, apiInfo, test.ShouldNotBeNil)
	res, err = apiInfo.RPCClient(nil, nil, "", arm.Named("foo"), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res.Name().Name, test.ShouldEqual, "foo")

	api4 := resource.APINamespace("acme4").WithComponentType(button)
	_, ok, err = resource.LookupAPIRegistration[arm.Arm](api4)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeFalse)
	test.That(t, func() {
		resource.RegisterAPI(api4, resource.APIRegistration[arm.Arm]{
			RPCServiceServerConstructor: sf,
			RPCClient:                   rcf,
			RPCServiceHandler:           pb.RegisterRobotServiceHandlerFromEndpoint,
		})
	}, test.ShouldPanic)
	test.That(t, func() {
		resource.RegisterAPIWithAssociation(api4, resource.APIRegistration[arm.Arm]{
			RPCServiceServerConstructor: sf,
			RPCClient:                   rcf,
			RPCServiceHandler:           pb.RegisterRobotServiceHandlerFromEndpoint,
		}, resource.AssociatedConfigRegistration[resource.AssociatedNameUpdater]{})
	}, test.ShouldPanic)

	resource.DeregisterAPI(api3)
	_, ok, err = resource.LookupAPIRegistration[arm.Arm](api3)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeFalse)
}

type someType struct {
	Field1  string `json:"field1"`
	capName resource.Name
}

func (st *someType) UpdateResourceNames(updater func(old resource.Name) resource.Name) {
	st.capName = updater(arm.Named("foo"))
}

func TestResourceAPIRegistryWithAssociation(t *testing.T) {
	statf := func(context.Context, arm.Arm) (interface{}, error) {
		return nil, errors.New("one")
	}
	sf := func(apiResColl resource.APIResourceCollection[arm.Arm]) interface{} {
		return nil
	}

	someName := resource.NewName(resource.APINamespace(uuid.NewString()).WithComponentType(button), "button1")
	resource.RegisterAPIWithAssociation(someName.API, resource.APIRegistration[arm.Arm]{
		Status:                      statf,
		RPCServiceServerConstructor: sf,
		RPCServiceHandler:           pb.RegisterRobotServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.RobotService_ServiceDesc,
	}, resource.AssociatedConfigRegistration[*someType]{})
	reg, ok := resource.LookupAssociatedConfigRegistration(someName.API)
	test.That(t, ok, test.ShouldBeTrue)
	assoc, err := reg.AttributeMapConverter(utils.AttributeMap{"field1": "hey"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, assoc.(*someType).Field1, test.ShouldEqual, "hey")
	test.That(t, assoc.(*someType).capName, test.ShouldResemble, resource.Name{})
	assoc.UpdateResourceNames(func(n resource.Name) resource.Name {
		return arm.Named(n.String()) // odd but whatever
	})
	test.That(t, assoc.(*someType).capName, test.ShouldResemble, arm.Named(arm.Named("foo").String()))
}

func TestDiscoveryFunctions(t *testing.T) {
	df := func(ctx context.Context, logger logging.Logger) (interface{}, error) {
		return []resource.Discovery{}, nil
	}
	validAPIQuery := resource.NewDiscoveryQuery(acme.API, resource.Model{Name: "some model"})
	_, ok := resource.LookupRegistration(validAPIQuery.API, validAPIQuery.Model)
	test.That(t, ok, test.ShouldBeFalse)

	rf := func(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger) (arm.Arm, error) {
		return &fake.Arm{Named: conf.ResourceName().AsNamed()}, nil
	}

	resource.Register(validAPIQuery.API, validAPIQuery.Model, resource.Registration[arm.Arm, resource.NoNativeConfig]{
		Constructor: rf,
		Discover:    df,
	})

	reg, ok := resource.LookupRegistration(validAPIQuery.API, validAPIQuery.Model)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, reg.Discover, test.ShouldEqual, df)
}

func TestTransformAttributeMap(t *testing.T) {
	type myType struct {
		A          string            `json:"a"`
		B          string            `json:"b"`
		Attributes map[string]string `json:"attributes"`
	}

	attrs := utils.AttributeMap{
		"a": "1",
		"b": "2",
		"c": "3",
		"d": "4",
		"e": 5,
	}
	transformed, err := resource.TransformAttributeMap[*myType](attrs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformed, test.ShouldResemble, &myType{
		A: "1",
		B: "2",
		Attributes: map[string]string{
			"c": "3",
			"d": "4",
		},
	})

	transformed, err = resource.TransformAttributeMap[*myType](attrs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformed, test.ShouldResemble, &myType{
		A: "1",
		B: "2",
		Attributes: map[string]string{
			"c": "3",
			"d": "4",
		},
	})

	type myExtendedType struct {
		A          string             `json:"a"`
		B          string             `json:"b"`
		Attributes utils.AttributeMap `json:"attributes"`
	}

	transformedExt, err := resource.TransformAttributeMap[*myExtendedType](attrs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformedExt, test.ShouldResemble, &myExtendedType{
		A: "1",
		B: "2",
		Attributes: utils.AttributeMap{
			"c": "3",
			"d": "4",
			"e": 5,
		},
	})

	transformedExt, err = resource.TransformAttributeMap[*myExtendedType](attrs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, transformedExt, test.ShouldResemble, &myExtendedType{
		A: "1",
		B: "2",
		Attributes: utils.AttributeMap{
			"c": "3",
			"d": "4",
			"e": 5,
		},
	})
}

func TestDependencyNotReadyError(t *testing.T) {
	toe := &resource.DependencyNotReadyError{"toe", errors.New("turf toe")}
	foot := &resource.DependencyNotReadyError{"foot", toe}
	leg := &resource.DependencyNotReadyError{"leg", foot}
	human := &resource.DependencyNotReadyError{"human", leg}

	test.That(t, strings.Count(human.Error(), "\\"), test.ShouldEqual, 0)
	test.That(t, human.PrettyPrint(), test.ShouldEqual, `Dependency "human" is not ready yet
  - Because "leg" is not ready yet
    - Because "foot" is not ready yet
      - Because "toe" is not ready yet
        - Because "turf toe"`)
}
