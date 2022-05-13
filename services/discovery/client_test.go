package discovery_test

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/edaniels/golog"
	// "github.com/mitchellh/mapstructure"
	// "github.com/pkg/errors"
	"go.viam.com/test"
	// "go.viam.com/utils"
	"go.viam.com/utils/rpc"
	// "google.golang.org/grpc"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/gps"
	"go.viam.com/rdk/component/imu"

	// viamgrpc "go.viam.com/rdk/grpc"
	// pb "go.viam.com/rdk/proto/api/service/discovery/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/discovery"
	"go.viam.com/rdk/subtype"

	// "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	injectDiscovery := &inject.DiscoveryService{}
	sMap := map[resource.Name]interface{}{discovery.Name: injectDiscovery}

	svc, err := subtype.New(sMap)
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype := registry.ResourceSubtypeLookup(discovery.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, svc)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	t.Run("failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = discovery.NewClient(cancelCtx, "", listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	t.Run("working discovery service", func(t *testing.T) {
		client, err := discovery.NewClient(context.Background(), "", listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)

		imuKey := discovery.Key{imu.Named("imu").ResourceSubtype, "some imu"}
		imuDiscovery := discovery.Discovery{Key: imuKey, Discovered: map[string]interface{}{"abc": []float64{1.2, 2.3, 3.4}}}
		fmt.Println(imuKey)
		gpsKey := discovery.Key{gps.Named("gps").ResourceSubtype, "some gps"}
		gpsDiscovery := discovery.Discovery{Key: gpsKey, Discovered: map[string]interface{}{"efg": []string{"hello"}}}
		fmt.Println(gpsKey)
		armKey := discovery.Key{arm.Named("arm").ResourceSubtype, "some arm"}
		armDiscovery := discovery.Discovery{Key: armKey, Discovered: struct{}{}}
		fmt.Println(armKey)

		injectDiscovery.DiscoverFunc = func(ctx context.Context, keys []discovery.Key) ([]discovery.Discovery, error) {
			discoveries := []discovery.Discovery{}

			discoverByKey := func(key discovery.Key) *discovery.Discovery {
				switch key {
				case imuKey:
					return &imuDiscovery
				case gpsKey:
					return &gpsDiscovery
				case armKey:
					fmt.Printf("Found arm key! %#v\n", armDiscovery)
					return &armDiscovery
				}
				return nil
			}
			for _, key := range keys {
				if discovery := discoverByKey(key); discovery != nil {
					fmt.Printf("Appending arm discovery! %#v\n", *discovery)
					discoveries = append(discoveries, *discovery)
				}
			}
			fmt.Printf("discoveries! %#v\n", discoveries)
			return discoveries, nil
		}
		resp, err := client.Discover(context.Background(), []discovery.Key{armKey})
		fmt.Printf("response: %#v\n", resp)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp), test.ShouldEqual, 1)
		test.That(t, resp[0], test.ShouldResemble, armDiscovery)

		// result := struct{}{}
		// decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &result})
		// test.That(t, err, test.ShouldBeNil)
		// err = decoder.Decode(resp[0].Discovery)
		// test.That(t, err, test.ShouldBeNil)
		// test.That(t, result, test.ShouldResemble, aDiscovery.Discovery)
		//
		// resp, err = client.GetDiscovery(context.Background(), []resource.Name{iDiscovery.Name, gDiscovery.Name, aDiscovery.Name})
		// test.That(t, err, test.ShouldBeNil)
		// test.That(t, len(resp), test.ShouldEqual, 3)
		//
		// observed := map[resource.Name]interface{}{
		// 	resp[0].Name: resp[0].Discovery,
		// 	resp[1].Name: resp[1].Discovery,
		// 	resp[2].Name: resp[2].Discovery,
		// }
		// test.That(t, observed, test.ShouldResemble, expected)
		//
		// test.That(t, utils.TryClose(context.Background(), client), test.ShouldBeNil)
	})

	// t.Run("failing discovery client", func(t *testing.T) {
	// 	conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
	// 	test.That(t, err, test.ShouldBeNil)
	// 	client := resourceSubtype.RPCClient(context.Background(), conn, "", logger)
	// 	client2, ok := client.(discovery.Service)
	// 	test.That(t, ok, test.ShouldBeTrue)
	//
	// 	passedErr := errors.New("can't get discovery")
	// 	injectDiscovery.GetDiscoveryFunc = func(ctx context.Context, discovery []resource.Name) ([]discovery.Discovery, error) {
	// 		return nil, passedErr
	// 	}
	// 	_, err = client2.GetDiscovery(context.Background(), []resource.Name{})
	// 	test.That(t, err.Error(), test.ShouldContainSubstring, passedErr.Error())
	//
	// 	test.That(t, utils.TryClose(context.Background(), client2), test.ShouldBeNil)
	// })
}

// func TestClientDialerOption(t *testing.T) {
// 	logger := golog.NewTestLogger(t)
// 	listener, err := net.Listen("tcp", "localhost:0")
// 	test.That(t, err, test.ShouldBeNil)
// 	gServer := grpc.NewServer()
//
// 	injectDiscovery := &inject.DiscoveryService{}
// 	sMap := map[resource.Name]interface{}{
// 		discovery.Name: injectDiscovery,
// 	}
// 	server, err := newServer(sMap)
// 	test.That(t, err, test.ShouldBeNil)
// 	pb.RegisterDiscoveryServiceServer(gServer, server)
//
// 	go gServer.Serve(listener)
// 	defer gServer.Stop()
//
// 	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
// 	ctx := rpc.ContextWithDialer(context.Background(), td)
// 	client1, err := discovery.NewClient(ctx, "", listener.Addr().String(), logger)
// 	test.That(t, err, test.ShouldBeNil)
// 	test.That(t, td.NewConnections, test.ShouldEqual, 3)
// 	client2, err := discovery.NewClient(ctx, "", listener.Addr().String(), logger)
// 	test.That(t, err, test.ShouldBeNil)
// 	test.That(t, td.NewConnections, test.ShouldEqual, 3)
//
// 	err = utils.TryClose(context.Background(), client1)
// 	test.That(t, err, test.ShouldBeNil)
// 	err = utils.TryClose(context.Background(), client2)
// 	test.That(t, err, test.ShouldBeNil)
// }
