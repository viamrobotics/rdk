package discovery_test

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/discovery/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/discovery"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
)

const (
	testDiscoveryName = "discovery1"
	failDiscoveryName = "discovery2"
)

var (
	errDoFailed       = errors.New("do failed")
	errDiscoverFailed = errors.New("discover failed")
)

// this was taken from proto_conversions_test and represents all of the information that a discovery service can provide about a component.
func createTestComponent(name string) resource.Config {
	testOrientation, _ := spatial.NewOrientationConfig(spatial.NewZeroOrientation())

	testFrame := &referenceframe.LinkConfig{
		Parent:      "world",
		Translation: r3.Vector{X: 1, Y: 2, Z: 3},
		Orientation: testOrientation,
		Geometry:    &spatial.GeometryConfig{Type: "box", X: 1, Y: 2, Z: 1},
	}

	testComponent := resource.Config{
		Name:      name,
		API:       resource.NewAPI("some-namespace", "component", "some-type"),
		Model:     resource.DefaultModelFamily.WithModel("some-model"),
		DependsOn: []string{"dep1", "dep2"},
		Attributes: utils.AttributeMap{
			"attr1": 1,
			"attr2": "attr-string",
		},
		AssociatedResourceConfigs: []resource.AssociatedResourceConfig{
			// these will get rewritten in tests to simulate API data
			{
				// will resemble the same but become "foo:bar:baz"
				API: resource.NewAPI("foo", "bar", "baz"),
				Attributes: utils.AttributeMap{
					"attr1": 1,
				},
			},
			{
				// will stay the same
				API: resource.APINamespaceRDK.WithServiceType("some-type-2"),
				Attributes: utils.AttributeMap{
					"attr1": 2,
				},
			},
			{
				// will resemble the same but be just "some-type-3"
				API: resource.APINamespaceRDK.WithServiceType("some-type-3"),
				Attributes: utils.AttributeMap{
					"attr1": 3,
				},
			},
		},
		Frame:            testFrame,
		LogConfiguration: &resource.LogConfig{Level: logging.DEBUG},
	}
	return testComponent
}

func newServer() (pb.DiscoveryServiceServer, *inject.DiscoveryService, *inject.DiscoveryService, error) {
	injectDiscovery := inject.NewDiscoveryService(testDiscoveryName)
	injectDiscovery2 := inject.NewDiscoveryService(failDiscoveryName)
	resourceMap := map[resource.Name]discovery.Service{
		discovery.Named(testDiscoveryName): injectDiscovery,
		discovery.Named(failDiscoveryName): injectDiscovery2,
	}
	injectSvc, err := resource.NewAPIResourceCollection(discovery.API, resourceMap)
	if err != nil {
		return nil, nil, nil, err
	}
	return discovery.NewRPCServiceServer(injectSvc).(pb.DiscoveryServiceServer), injectDiscovery, injectDiscovery2, nil
}

func TestDiscoveryServiceServer(t *testing.T) {
	discoveryServer, workingDiscovery, failingDiscovery, err := newServer()
	test.That(t, err, test.ShouldBeNil)
	testComponents := []resource.Config{createTestComponent("component-1"), createTestComponent("component-2")}

	t.Run("Test DiscoverResources", func(t *testing.T) {
		workingDiscovery.DiscoverResourcesFunc = func(ctx context.Context, extra map[string]any) ([]resource.Config, error) {
			return testComponents, nil
		}
		failingDiscovery.DiscoverResourcesFunc = func(ctx context.Context, extra map[string]any) ([]resource.Config, error) {
			return nil, errDiscoverFailed
		}
		resp, err := discoveryServer.DiscoverResources(context.Background(), &pb.DiscoverResourcesRequest{Name: testDiscoveryName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp.GetDiscoveries()), test.ShouldEqual, len(testComponents))
		for index, proto := range resp.GetDiscoveries() {
			expected := testComponents[index]
			test.That(t, proto.Name, test.ShouldEqual, expected.Name)
			actual, err := config.ComponentConfigFromProto(proto)
			test.That(t, err, test.ShouldBeNil)
			validateComponent(t, *actual, expected)
		}

		respFail, err := discoveryServer.DiscoverResources(context.Background(), &pb.DiscoverResourcesRequest{Name: failDiscoveryName})
		test.That(t, err, test.ShouldEqual, errDiscoverFailed)
		test.That(t, respFail, test.ShouldBeNil)
	})
	t.Run("Test nil DiscoverResources response", func(t *testing.T) {
		failingDiscovery.DiscoverResourcesFunc = func(ctx context.Context, extra map[string]any) ([]resource.Config, error) {
			return nil, nil
		}
		resp, err := discoveryServer.DiscoverResources(context.Background(), &pb.DiscoverResourcesRequest{Name: failDiscoveryName})
		test.That(t, err, test.ShouldEqual, discovery.ErrNilResponse)
		test.That(t, resp, test.ShouldEqual, nil)
	})

	t.Run("Test DoCommand", func(t *testing.T) {
		workingDiscovery.DoFunc = func(
			ctx context.Context,
			cmd map[string]interface{},
		) (
			map[string]interface{},
			error,
		) {
			return cmd, nil
		}
		failingDiscovery.DoFunc = func(
			ctx context.Context,
			cmd map[string]interface{},
		) (
			map[string]interface{},
			error,
		) {
			return nil, errDoFailed
		}

		commandStruct, err := protoutils.StructToStructPb(testutils.TestCommand)
		test.That(t, err, test.ShouldBeNil)

		req := commonpb.DoCommandRequest{Name: testDiscoveryName, Command: commandStruct}
		resp, err := discoveryServer.DoCommand(context.Background(), &req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resp, test.ShouldNotBeNil)
		test.That(t, resp.Result.AsMap()["cmd"], test.ShouldEqual, testutils.TestCommand["cmd"])
		test.That(t, resp.Result.AsMap()["data"], test.ShouldEqual, testutils.TestCommand["data"])

		req = commonpb.DoCommandRequest{Name: failDiscoveryName, Command: commandStruct}
		resp, err = discoveryServer.DoCommand(context.Background(), &req)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, errDoFailed.Error())
		test.That(t, resp, test.ShouldBeNil)
	})
}

// this was taken from proto_conversion_test.go.
func validateComponent(t *testing.T, actual, expected resource.Config) {
	t.Helper()
	test.That(t, actual.Name, test.ShouldEqual, expected.Name)
	test.That(t, actual.API, test.ShouldResemble, expected.API)
	test.That(t, actual.Model, test.ShouldResemble, expected.Model)
	test.That(t, actual.DependsOn, test.ShouldResemble, expected.DependsOn)
	test.That(t, actual.Attributes.Int("attr1", 0), test.ShouldEqual, expected.Attributes.Int("attr1", -1))
	test.That(t, actual.Attributes.String("attr2"), test.ShouldEqual, expected.Attributes.String("attr2"))

	test.That(t, actual.AssociatedResourceConfigs, test.ShouldHaveLength, 3)
	test.That(t, actual.AssociatedResourceConfigs[0].API, test.ShouldResemble, expected.AssociatedResourceConfigs[0].API)
	test.That(t,
		actual.AssociatedResourceConfigs[0].Attributes.Int("attr1", 0),
		test.ShouldEqual,
		expected.AssociatedResourceConfigs[0].Attributes.Int("attr1", -1))
	test.That(t,
		actual.AssociatedResourceConfigs[1].API,
		test.ShouldResemble,
		expected.AssociatedResourceConfigs[1].API)
	test.That(t,
		actual.AssociatedResourceConfigs[1].Attributes.Int("attr1", 0),
		test.ShouldEqual,
		expected.AssociatedResourceConfigs[1].Attributes.Int("attr1", -1))
	test.That(t,
		actual.AssociatedResourceConfigs[2].API,
		test.ShouldResemble,
		expected.AssociatedResourceConfigs[2].API)
	test.That(t,
		actual.AssociatedResourceConfigs[2].Attributes.Int("attr1", 0),
		test.ShouldEqual,
		expected.AssociatedResourceConfigs[2].Attributes.Int("attr1", -1))

	f1, err := actual.Frame.ParseConfig()
	test.That(t, err, test.ShouldBeNil)
	f2, err := expected.Frame.ParseConfig()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, f1, test.ShouldResemble, f2)
}
