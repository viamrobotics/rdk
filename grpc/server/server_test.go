package server_test

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/imu"
	"go.viam.com/rdk/grpc/server"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/robot/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
)

var emptyResources = &pb.ResourceNamesResponse{
	Resources: []*commonpb.ResourceName{},
}

var serverNewResource = resource.NewName(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeComponent,
	arm.SubtypeName,
	"",
)

var serverOneResourceResponse = []*commonpb.ResourceName{
	{
		Namespace: string(serverNewResource.Namespace),
		Type:      string(serverNewResource.ResourceType),
		Subtype:   string(serverNewResource.ResourceSubtype),
		Name:      serverNewResource.Name,
	},
}

func TestServer(t *testing.T) {
	t.Run("Metadata", func(t *testing.T) {
		injectRobot := &inject.Robot{}
		injectRobot.ResourceNamesFunc = func() []resource.Name { return []resource.Name{} }
		server := server.New(injectRobot)

		resourceResp, err := server.ResourceNames(context.Background(), &pb.ResourceNamesRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resourceResp, test.ShouldResemble, emptyResources)

		injectRobot.ResourceNamesFunc = func() []resource.Name { return []resource.Name{serverNewResource} }

		resourceResp, err = server.ResourceNames(context.Background(), &pb.ResourceNamesRequest{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resourceResp.Resources, test.ShouldResemble, serverOneResourceResponse)
	})
}

// DISCOVERY

func newServer(sMap map[resource.Name]interface{}) (pb.DiscoveryServiceServer, error) {
	sSvc, err := subtype.New(sMap)
	if err != nil {
		return nil, err
	}
	return discovery.NewServer(sSvc), nil
}

func TestServerGetDiscovery(t *testing.T) {
	t.Run("no discovery service", func(t *testing.T) {
		sMap := map[resource.Name]interface{}{}
		server, err := newServer(sMap)
		test.That(t, err, test.ShouldBeNil)
		_, err = server.Discover(context.Background(), &pb.DiscoverRequest{})
		test.That(t, err, test.ShouldBeError, rutils.NewResourceNotFoundError(discovery.Name))
	})

	t.Run("not discovery service", func(t *testing.T) {
		sMap := map[resource.Name]interface{}{discovery.Name: "not discovery"}
		server, err := newServer(sMap)
		test.That(t, err, test.ShouldBeNil)
		_, err = server.Discover(context.Background(), &pb.DiscoverRequest{})
		test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("discovery.Service", "string"))
	})

	t.Run("failed Discover", func(t *testing.T) {
		injectDiscovery := &inject.DiscoveryService{}
		sMap := map[resource.Name]interface{}{
			discovery.Name: injectDiscovery,
		}
		server, err := newServer(sMap)
		test.That(t, err, test.ShouldBeNil)
		passedErr := errors.New("can't get discovery")
		injectDiscovery.DiscoverFunc = func(ctx context.Context, keys []discovery.Key) ([]discovery.Discovery, error) {
			return nil, passedErr
		}
		_, err = server.Discover(context.Background(), &pb.DiscoverRequest{})
		test.That(t, err, test.ShouldBeError, passedErr)
	})

	t.Run("bad discovery response", func(t *testing.T) {
		injectDiscovery := &inject.DiscoveryService{}
		sMap := map[resource.Name]interface{}{
			discovery.Name: injectDiscovery,
		}
		server, err := newServer(sMap)
		test.That(t, err, test.ShouldBeNil)
		armKey := discovery.Key{arm.Named("arm").ResourceSubtype, "some arm"}
		armDiscovery := discovery.Discovery{Key: armKey, Discovered: 1}
		discoveries := []discovery.Discovery{armDiscovery}
		injectDiscovery.DiscoverFunc = func(ctx context.Context, keys []discovery.Key) ([]discovery.Discovery, error) {
			return discoveries, nil
		}
		req := &pb.DiscoverRequest{
			Keys: []*pb.Key{},
		}

		_, err = server.Discover(context.Background(), req)
		test.That(
			t,
			err,
			test.ShouldBeError,
			errors.New(
				"unable to convert discovery for {\"arm\" \"some arm\"} to a form acceptable to structpb.NewStruct: "+
					"data of type int and kind int not a struct or a map-like object",
			),
		)
	})

	t.Run("working one discovery", func(t *testing.T) {
		injectDiscovery := &inject.DiscoveryService{}
		sMap := map[resource.Name]interface{}{
			discovery.Name: injectDiscovery,
		}
		server, err := newServer(sMap)
		test.That(t, err, test.ShouldBeNil)
		armKey := discovery.Key{arm.Named("arm").ResourceSubtype, "some arm"}
		armDiscovery := discovery.Discovery{Key: armKey, Discovered: struct{}{}}
		discoveries := []discovery.Discovery{armDiscovery}
		injectDiscovery.DiscoverFunc = func(ctx context.Context, keys []discovery.Key) ([]discovery.Discovery, error) {
			return discoveries, nil
		}
		req := &pb.DiscoverRequest{
			Keys: []*pb.Key{
				{Subtype: string(armKey.SubtypeName), Model: armKey.Model},
			},
		}

		resp, err := server.Discover(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp.Discovery), test.ShouldEqual, 1)

		observed := resp.Discovery[0].Discovered.AsMap()
		expected := map[string]interface{}{}
		test.That(t, observed, test.ShouldResemble, expected)
	})

	t.Run("working many discoveries", func(t *testing.T) {
		injectDiscovery := &inject.DiscoveryService{}
		sMap := map[resource.Name]interface{}{
			discovery.Name: injectDiscovery,
		}
		server, err := newServer(sMap)
		test.That(t, err, test.ShouldBeNil)

		imuKey := discovery.Key{imu.Named("imu").ResourceSubtype, "some imu"}
		imuDiscovery := discovery.Discovery{Key: imuKey, Discovered: map[string]interface{}{"abc": []float64{1.2, 2.3, 3.4}}}
		gpsKey := discovery.Key{arm.Named("gps").ResourceSubtype, "some gps"}
		gpsDiscovery := discovery.Discovery{Key: gpsKey, Discovered: map[string]interface{}{"efg": []string{"hello"}}}
		armKey := discovery.Key{arm.Named("arm").ResourceSubtype, "some arm"}
		armDiscovery := discovery.Discovery{Key: armKey, Discovered: struct{}{}}

		discoveries := []discovery.Discovery{imuDiscovery, gpsDiscovery, armDiscovery}
		injectDiscovery.DiscoverFunc = func(ctx context.Context, keys []discovery.Key) ([]discovery.Discovery, error) {
			return discoveries, nil
		}

		req := &pb.DiscoverRequest{
			Keys: []*pb.Key{
				{Subtype: string(imuKey.SubtypeName), Model: imuKey.Model},
				{Subtype: string(gpsKey.SubtypeName), Model: gpsKey.Model},
				{Subtype: string(armKey.SubtypeName), Model: armKey.Model},
			},
		}

		resp, err := server.Discover(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp.Discovery), test.ShouldEqual, 3)

		test.That(t, resp.Discovery[0].Key.Subtype, test.ShouldResemble, string(imuKey.SubtypeName))
		test.That(t, resp.Discovery[0].Key.Model, test.ShouldResemble, imuKey.Model)
		test.That(t, resp.Discovery[0].Discovered.AsMap(), test.ShouldResemble, map[string]interface{}{"abc": []interface{}{1.2, 2.3, 3.4}})

		test.That(t, resp.Discovery[1].Key.Subtype, test.ShouldResemble, string(gpsKey.SubtypeName))
		test.That(t, resp.Discovery[1].Key.Model, test.ShouldResemble, gpsKey.Model)
		test.That(t, resp.Discovery[1].Discovered.AsMap(), test.ShouldResemble, map[string]interface{}{"efg": []interface{}{"hello"}})

		test.That(t, resp.Discovery[2].Key.Subtype, test.ShouldResemble, string(armKey.SubtypeName))
		test.That(t, resp.Discovery[2].Key.Model, test.ShouldResemble, armKey.Model)
		test.That(t, resp.Discovery[2].Discovered.AsMap(), test.ShouldResemble, map[string]interface{}{})
	})
}
