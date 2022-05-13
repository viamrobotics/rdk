package discovery_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.viam.com/test"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/gps"
	"go.viam.com/rdk/component/imu"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/service/discovery/v1"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/discovery"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

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
		_, err = server.GetDiscovery(context.Background(), &pb.GetDiscoveryRequest{})
		test.That(t, err, test.ShouldBeError, rutils.NewResourceNotFoundError(discovery.Name))
	})

	t.Run("not discovery service", func(t *testing.T) {
		sMap := map[resource.Name]interface{}{discovery.Name: "not discovery"}
		server, err := newServer(sMap)
		test.That(t, err, test.ShouldBeNil)
		_, err = server.GetDiscovery(context.Background(), &pb.GetDiscoveryRequest{})
		test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("discovery.Service", "string"))
	})

	t.Run("failed GetDiscovery", func(t *testing.T) {
		injectDiscovery := &inject.DiscoveryService{}
		sMap := map[resource.Name]interface{}{
			discovery.Name: injectDiscovery,
		}
		server, err := newServer(sMap)
		test.That(t, err, test.ShouldBeNil)
		passedErr := errors.New("can't get discovery")
		injectDiscovery.GetDiscoveryFunc = func(ctx context.Context, resourceNames []resource.Name) ([]discovery.Discovery, error) {
			return nil, passedErr
		}
		_, err = server.GetDiscovery(context.Background(), &pb.GetDiscoveryRequest{})
		test.That(t, err, test.ShouldBeError, passedErr)
	})

	t.Run("bad discovery response", func(t *testing.T) {
		injectDiscovery := &inject.DiscoveryService{}
		sMap := map[resource.Name]interface{}{
			discovery.Name: injectDiscovery,
		}
		server, err := newServer(sMap)
		test.That(t, err, test.ShouldBeNil)
		aDiscovery := discovery.Discovery{Name: arm.Named("arm"), Discovery: 1}
		readings := []discovery.Discovery{aDiscovery}
		injectDiscovery.GetDiscoveryFunc = func(ctx context.Context, discovery []resource.Name) ([]discovery.Discovery, error) {
			return readings, nil
		}
		req := &pb.GetDiscoveryRequest{
			ResourceNames: []*commonpb.ResourceName{},
		}

		_, err = server.GetDiscovery(context.Background(), req)
		test.That(
			t,
			err,
			test.ShouldBeError,
			errors.New(
				"unable to convert discovery for \"rdk:component:arm/arm\" to a form acceptable to structpb.NewStruct: "+
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
		aDiscovery := discovery.Discovery{Name: arm.Named("arm"), Discovery: struct{}{}}
		readings := []discovery.Discovery{aDiscovery}
		expected := map[resource.Name]interface{}{
			aDiscovery.Name: map[string]interface{}{},
		}
		injectDiscovery.GetDiscoveryFunc = func(ctx context.Context, resourceNames []resource.Name) ([]discovery.Discovery, error) {
			test.That(
				t,
				testutils.NewResourceNameSet(resourceNames...),
				test.ShouldResemble,
				testutils.NewResourceNameSet(aDiscovery.Name),
			)
			return readings, nil
		}
		req := &pb.GetDiscoveryRequest{
			ResourceNames: []*commonpb.ResourceName{protoutils.ResourceNameToProto(aDiscovery.Name)},
		}

		resp, err := server.GetDiscovery(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp.Discovery), test.ShouldEqual, 1)

		observed := map[resource.Name]interface{}{
			protoutils.ResourceNameFromProto(resp.Discovery[0].Name): resp.Discovery[0].Discovery.AsMap(),
		}
		test.That(t, observed, test.ShouldResemble, expected)
	})

	t.Run("working many discoveries", func(t *testing.T) {
		injectDiscovery := &inject.DiscoveryService{}
		sMap := map[resource.Name]interface{}{
			discovery.Name: injectDiscovery,
		}
		server, err := newServer(sMap)
		test.That(t, err, test.ShouldBeNil)
		iDiscovery := discovery.Discovery{Name: imu.Named("imu"), Discovery: map[string]interface{}{"abc": []float64{1.2, 2.3, 3.4}}}
		gDiscovery := discovery.Discovery{Name: gps.Named("gps"), Discovery: map[string]interface{}{"efg": []string{"hello"}}}
		aDiscovery := discovery.Discovery{Name: arm.Named("arm"), Discovery: struct{}{}}
		discoveries := []discovery.Discovery{iDiscovery, gDiscovery, aDiscovery}
		expected := map[resource.Name]interface{}{
			iDiscovery.Name: map[string]interface{}{"abc": []interface{}{1.2, 2.3, 3.4}},
			gDiscovery.Name: map[string]interface{}{"efg": []interface{}{"hello"}},
			aDiscovery.Name: map[string]interface{}{},
		}
		injectDiscovery.GetDiscoveryFunc = func(ctx context.Context, resourceNames []resource.Name) ([]discovery.Discovery, error) {
			test.That(
				t,
				testutils.NewResourceNameSet(resourceNames...),
				test.ShouldResemble,
				testutils.NewResourceNameSet(iDiscovery.Name, gDiscovery.Name, aDiscovery.Name),
			)
			return discoveries, nil
		}
		req := &pb.GetDiscoveryRequest{
			ResourceNames: []*commonpb.ResourceName{
				protoutils.ResourceNameToProto(iDiscovery.Name),
				protoutils.ResourceNameToProto(gDiscovery.Name),
				protoutils.ResourceNameToProto(aDiscovery.Name),
			},
		}

		resp, err := server.GetDiscovery(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp.Discovery), test.ShouldEqual, 3)

		observed := map[resource.Name]interface{}{
			protoutils.ResourceNameFromProto(resp.Discovery[0].Name): resp.Discovery[0].Discovery.AsMap(),
			protoutils.ResourceNameFromProto(resp.Discovery[1].Name): resp.Discovery[1].Discovery.AsMap(),
			protoutils.ResourceNameFromProto(resp.Discovery[2].Name): resp.Discovery[2].Discovery.AsMap(),
		}
		test.That(t, observed, test.ShouldResemble, expected)
	})

	t.Run("failed StreamDiscovery", func(t *testing.T) {
		injectDiscovery := &inject.DiscoveryService{}
		sMap := map[resource.Name]interface{}{
			discovery.Name: injectDiscovery,
		}
		server, err := newServer(sMap)
		test.That(t, err, test.ShouldBeNil)
		err1 := errors.New("whoops")
		injectDiscovery.GetDiscoveryFunc = func(ctx context.Context, resourceNames []resource.Name) ([]discovery.Discovery, error) {
			return nil, err1
		}

		cancelCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		messageCh := make(chan *pb.StreamDiscoveryResponse)
		streamServer := &discoveryStreamServer{
			ctx:       cancelCtx,
			messageCh: messageCh,
		}
		err = server.StreamDiscovery(&pb.StreamDiscoveryRequest{Every: durationpb.New(time.Second)}, streamServer)
		test.That(t, err, test.ShouldEqual, err1)
	})

	t.Run("failed StreamDiscovery server send", func(t *testing.T) {
		injectDiscovery := &inject.DiscoveryService{}
		sMap := map[resource.Name]interface{}{
			discovery.Name: injectDiscovery,
		}
		server, err := newServer(sMap)
		test.That(t, err, test.ShouldBeNil)
		injectDiscovery.GetDiscoveryFunc = func(ctx context.Context, resourceNames []resource.Name) ([]discovery.Discovery, error) {
			return []discovery.Discovery{{arm.Named("arm"), struct{}{}}}, nil
		}

		cancelCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		messageCh := make(chan *pb.StreamDiscoveryResponse)
		streamServer := &discoveryStreamServer{
			ctx:       cancelCtx,
			messageCh: messageCh,
			fail:      true,
		}
		dur := 100 * time.Millisecond
		err = server.StreamDiscovery(&pb.StreamDiscoveryRequest{Every: durationpb.New(dur)}, streamServer)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "send fail")
	})

	t.Run("timed out StreamDiscovery", func(t *testing.T) {
		injectDiscovery := &inject.DiscoveryService{}
		sMap := map[resource.Name]interface{}{
			discovery.Name: injectDiscovery,
		}
		server, err := newServer(sMap)
		test.That(t, err, test.ShouldBeNil)
		injectDiscovery.GetDiscoveryFunc = func(ctx context.Context, resourceNames []resource.Name) ([]discovery.Discovery, error) {
			return []discovery.Discovery{{arm.Named("arm"), struct{}{}}}, nil
		}

		timeoutCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		streamServer := &discoveryStreamServer{
			ctx:       timeoutCtx,
			messageCh: nil,
		}
		dur := 100 * time.Millisecond

		streamErr := server.StreamDiscovery(&pb.StreamDiscoveryRequest{Every: durationpb.New(dur)}, streamServer)
		test.That(t, streamErr, test.ShouldResemble, context.DeadlineExceeded)
	})

	t.Run("working StreamDiscovery", func(t *testing.T) {
		injectDiscovery := &inject.DiscoveryService{}
		sMap := map[resource.Name]interface{}{
			discovery.Name: injectDiscovery,
		}
		server, err := newServer(sMap)
		test.That(t, err, test.ShouldBeNil)
		injectDiscovery.GetDiscoveryFunc = func(ctx context.Context, resourceNames []resource.Name) ([]discovery.Discovery, error) {
			return []discovery.Discovery{{arm.Named("arm"), struct{}{}}}, nil
		}

		cancelCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		messageCh := make(chan *pb.StreamDiscoveryResponse)
		streamServer := &discoveryStreamServer{
			ctx:       cancelCtx,
			messageCh: messageCh,
			fail:      false,
		}
		dur := 100 * time.Millisecond
		var streamErr error
		start := time.Now()
		done := make(chan struct{})
		go func() {
			streamErr = server.StreamDiscovery(&pb.StreamDiscoveryRequest{Every: durationpb.New(dur)}, streamServer)
			close(done)
		}()
		expectedDiscovery, err := structpb.NewStruct(map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		var messages []*pb.StreamDiscoveryResponse
		messages = append(messages, <-messageCh)
		messages = append(messages, <-messageCh)
		messages = append(messages, <-messageCh)
		test.That(t, messages, test.ShouldResemble, []*pb.StreamDiscoveryResponse{
			{Discovery: []*pb.Discovery{{Name: protoutils.ResourceNameToProto(arm.Named("arm")), Discovery: expectedDiscovery}}},
			{Discovery: []*pb.Discovery{{Name: protoutils.ResourceNameToProto(arm.Named("arm")), Discovery: expectedDiscovery}}},
			{Discovery: []*pb.Discovery{{Name: protoutils.ResourceNameToProto(arm.Named("arm")), Discovery: expectedDiscovery}}},
		})
		test.That(t, time.Since(start), test.ShouldBeGreaterThanOrEqualTo, 3*dur)
		test.That(t, time.Since(start), test.ShouldBeLessThanOrEqualTo, 6*dur)
		cancel()
		<-done
		test.That(t, streamErr, test.ShouldEqual, context.Canceled)
	})
}

type discoveryStreamServer struct {
	grpc.ServerStream // not set
	ctx               context.Context
	messageCh         chan<- *pb.StreamDiscoveryResponse
	fail              bool
}

func (x *discoveryStreamServer) Context() context.Context {
	return x.ctx
}

func (x *discoveryStreamServer) Send(m *pb.StreamDiscoveryResponse) error {
	if x.fail {
		return errors.New("send fail")
	}
	if x.messageCh == nil {
		return nil
	}
	x.messageCh <- m
	return nil
}
