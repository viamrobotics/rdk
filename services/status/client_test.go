package status_test

import (
	"context"
	"net"
	"testing"

	"github.com/edaniels/golog"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/gps"
	"go.viam.com/rdk/component/imu"
	viamgrpc "go.viam.com/rdk/grpc"
	pb "go.viam.com/rdk/proto/api/service/status/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/status"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

func newServer(sMap map[resource.Name]interface{}) (pb.StatusServiceServer, error) {
	sSvc, err := subtype.New(sMap)
	if err != nil {
		return nil, err
	}
	return status.NewServer(sSvc), nil
}

func TestClient(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener1, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	injectStatus := &inject.StatusService{}
	sMap := map[resource.Name]interface{}{
		status.Name: injectStatus,
	}
	svc, err := subtype.New(sMap)
	test.That(t, err, test.ShouldBeNil)
	resourceSubtype := registry.ResourceSubtypeLookup(status.Subtype)
	resourceSubtype.RegisterRPCService(context.Background(), rpcServer, svc)

	go rpcServer.Serve(listener1)
	defer rpcServer.Stop()

	t.Run("failing client", func(t *testing.T) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = status.NewClient(cancelCtx, "", listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "canceled")
	})

	t.Run("working status service", func(t *testing.T) {
		client, err := status.NewClient(context.Background(), "", listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)

		iStatus := status.Status{Name: imu.Named("imu"), Status: map[string]interface{}{"abc": []float64{1.2, 2.3, 3.4}}}
		gStatus := status.Status{Name: gps.Named("gps"), Status: map[string]interface{}{"efg": []string{"hello"}}}
		aStatus := status.Status{Name: arm.Named("arm"), Status: struct{}{}}
		statusMap := map[resource.Name]status.Status{
			iStatus.Name: iStatus,
			gStatus.Name: gStatus,
			aStatus.Name: aStatus,
		}
		injectStatus.GetStatusFunc = func(ctx context.Context, resourceNames []resource.Name) ([]status.Status, error) {
			statuses := make([]status.Status, 0, len(resourceNames))
			for _, n := range resourceNames {
				statuses = append(statuses, statusMap[n])
			}
			return statuses, nil
		}
		expected := map[resource.Name]interface{}{
			iStatus.Name: map[string]interface{}{"abc": []interface{}{1.2, 2.3, 3.4}},
			gStatus.Name: map[string]interface{}{"efg": []interface{}{"hello"}},
			aStatus.Name: map[string]interface{}{},
		}
		resp, err := client.GetStatus(context.Background(), []resource.Name{aStatus.Name})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp), test.ShouldEqual, 1)
		test.That(t, resp[0].Status, test.ShouldResemble, expected[resp[0].Name])

		result := struct{}{}
		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &result})
		test.That(t, err, test.ShouldBeNil)
		err = decoder.Decode(resp[0].Status)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, result, test.ShouldResemble, aStatus.Status)

		resp, err = client.GetStatus(context.Background(), []resource.Name{iStatus.Name, gStatus.Name, aStatus.Name})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp), test.ShouldEqual, 3)

		observed := map[resource.Name]interface{}{
			resp[0].Name: resp[0].Status,
			resp[1].Name: resp[1].Status,
			resp[2].Name: resp[2].Status,
		}
		test.That(t, observed, test.ShouldResemble, expected)

		test.That(t, utils.TryClose(context.Background(), client), test.ShouldBeNil)
	})

	t.Run("failing status client", func(t *testing.T) {
		conn, err := viamgrpc.Dial(context.Background(), listener1.Addr().String(), logger)
		test.That(t, err, test.ShouldBeNil)
		client := resourceSubtype.RPCClient(context.Background(), conn, "", logger)
		client2, ok := client.(status.Service)
		test.That(t, ok, test.ShouldBeTrue)

		passedErr := errors.New("can't get status")
		injectStatus.GetStatusFunc = func(ctx context.Context, status []resource.Name) ([]status.Status, error) {
			return nil, passedErr
		}
		_, err = client2.GetStatus(context.Background(), []resource.Name{})
		test.That(t, err.Error(), test.ShouldContainSubstring, passedErr.Error())

		test.That(t, utils.TryClose(context.Background(), client2), test.ShouldBeNil)
	})
}

func TestClientDialerOption(t *testing.T) {
	logger := golog.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	gServer := grpc.NewServer()

	injectStatus := &inject.StatusService{}
	sMap := map[resource.Name]interface{}{
		status.Name: injectStatus,
	}
	server, err := newServer(sMap)
	test.That(t, err, test.ShouldBeNil)
	pb.RegisterStatusServiceServer(gServer, server)

	go gServer.Serve(listener)
	defer gServer.Stop()

	td := &testutils.TrackingDialer{Dialer: rpc.NewCachedDialer()}
	ctx := rpc.ContextWithDialer(context.Background(), td)
	client1, err := status.NewClient(ctx, "", listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)
	client2, err := status.NewClient(ctx, "", listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, td.NewConnections, test.ShouldEqual, 3)

	err = utils.TryClose(context.Background(), client1)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(context.Background(), client2)
	test.That(t, err, test.ShouldBeNil)
}
