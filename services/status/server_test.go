package status_test

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
	pb "go.viam.com/rdk/proto/api/service/status/v1"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/status"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

func TestServerGetStatus(t *testing.T) {
	t.Run("no status service", func(t *testing.T) {
		sMap := map[resource.Name]interface{}{}
		server, err := newServer(sMap)
		test.That(t, err, test.ShouldBeNil)
		_, err = server.GetStatus(context.Background(), &pb.GetStatusRequest{})
		test.That(t, err, test.ShouldBeError, rutils.NewResourceNotFoundError(status.Name))
	})

	t.Run("not status service", func(t *testing.T) {
		sMap := map[resource.Name]interface{}{status.Name: "not status"}
		server, err := newServer(sMap)
		test.That(t, err, test.ShouldBeNil)
		_, err = server.GetStatus(context.Background(), &pb.GetStatusRequest{})
		test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("status.Service", "string"))
	})

	t.Run("failed GetStatus", func(t *testing.T) {
		injectStatus := &inject.StatusService{}
		sMap := map[resource.Name]interface{}{
			status.Name: injectStatus,
		}
		server, err := newServer(sMap)
		test.That(t, err, test.ShouldBeNil)
		passedErr := errors.New("can't get status")
		injectStatus.GetStatusFunc = func(ctx context.Context, resourceNames []resource.Name) ([]status.Status, error) {
			return nil, passedErr
		}
		_, err = server.GetStatus(context.Background(), &pb.GetStatusRequest{})
		test.That(t, err, test.ShouldBeError, passedErr)
	})

	t.Run("bad status response", func(t *testing.T) {
		injectStatus := &inject.StatusService{}
		sMap := map[resource.Name]interface{}{
			status.Name: injectStatus,
		}
		server, err := newServer(sMap)
		test.That(t, err, test.ShouldBeNil)
		aStatus := status.Status{Name: arm.Named("arm"), Status: 1}
		readings := []status.Status{aStatus}
		injectStatus.GetStatusFunc = func(ctx context.Context, status []resource.Name) ([]status.Status, error) {
			return readings, nil
		}
		req := &pb.GetStatusRequest{
			ResourceNames: []*commonpb.ResourceName{},
		}

		_, err = server.GetStatus(context.Background(), req)
		test.That(
			t,
			err,
			test.ShouldBeError,
			errors.New(
				"unable to convert status for \"rdk:component:arm/arm\" to a form acceptable to structpb.NewStruct: "+
					"data of type int and kind int not a struct or a map-like object",
			),
		)
	})

	t.Run("working one status", func(t *testing.T) {
		injectStatus := &inject.StatusService{}
		sMap := map[resource.Name]interface{}{
			status.Name: injectStatus,
		}
		server, err := newServer(sMap)
		test.That(t, err, test.ShouldBeNil)
		aStatus := status.Status{Name: arm.Named("arm"), Status: struct{}{}}
		readings := []status.Status{aStatus}
		expected := map[resource.Name]interface{}{
			aStatus.Name: map[string]interface{}{},
		}
		injectStatus.GetStatusFunc = func(ctx context.Context, resourceNames []resource.Name) ([]status.Status, error) {
			test.That(
				t,
				testutils.NewResourceNameSet(resourceNames...),
				test.ShouldResemble,
				testutils.NewResourceNameSet(aStatus.Name),
			)
			return readings, nil
		}
		req := &pb.GetStatusRequest{
			ResourceNames: []*commonpb.ResourceName{protoutils.ResourceNameToProto(aStatus.Name)},
		}

		resp, err := server.GetStatus(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp.Status), test.ShouldEqual, 1)

		observed := map[resource.Name]interface{}{
			protoutils.ResourceNameFromProto(resp.Status[0].Name): resp.Status[0].Status.AsMap(),
		}
		test.That(t, observed, test.ShouldResemble, expected)
	})

	t.Run("working many statuses", func(t *testing.T) {
		injectStatus := &inject.StatusService{}
		sMap := map[resource.Name]interface{}{
			status.Name: injectStatus,
		}
		server, err := newServer(sMap)
		test.That(t, err, test.ShouldBeNil)
		iStatus := status.Status{Name: imu.Named("imu"), Status: map[string]interface{}{"abc": []float64{1.2, 2.3, 3.4}}}
		gStatus := status.Status{Name: gps.Named("gps"), Status: map[string]interface{}{"efg": []string{"hello"}}}
		aStatus := status.Status{Name: arm.Named("arm"), Status: struct{}{}}
		statuses := []status.Status{iStatus, gStatus, aStatus}
		expected := map[resource.Name]interface{}{
			iStatus.Name: map[string]interface{}{"abc": []interface{}{1.2, 2.3, 3.4}},
			gStatus.Name: map[string]interface{}{"efg": []interface{}{"hello"}},
			aStatus.Name: map[string]interface{}{},
		}
		injectStatus.GetStatusFunc = func(ctx context.Context, resourceNames []resource.Name) ([]status.Status, error) {
			test.That(
				t,
				testutils.NewResourceNameSet(resourceNames...),
				test.ShouldResemble,
				testutils.NewResourceNameSet(iStatus.Name, gStatus.Name, aStatus.Name),
			)
			return statuses, nil
		}
		req := &pb.GetStatusRequest{
			ResourceNames: []*commonpb.ResourceName{
				protoutils.ResourceNameToProto(iStatus.Name),
				protoutils.ResourceNameToProto(gStatus.Name),
				protoutils.ResourceNameToProto(aStatus.Name),
			},
		}

		resp, err := server.GetStatus(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp.Status), test.ShouldEqual, 3)

		observed := map[resource.Name]interface{}{
			protoutils.ResourceNameFromProto(resp.Status[0].Name): resp.Status[0].Status.AsMap(),
			protoutils.ResourceNameFromProto(resp.Status[1].Name): resp.Status[1].Status.AsMap(),
			protoutils.ResourceNameFromProto(resp.Status[2].Name): resp.Status[2].Status.AsMap(),
		}
		test.That(t, observed, test.ShouldResemble, expected)
	})

	t.Run("failed StreamStatus", func(t *testing.T) {
		injectStatus := &inject.StatusService{}
		sMap := map[resource.Name]interface{}{
			status.Name: injectStatus,
		}
		server, err := newServer(sMap)
		test.That(t, err, test.ShouldBeNil)
		err1 := errors.New("whoops")
		injectStatus.GetStatusFunc = func(ctx context.Context, resourceNames []resource.Name) ([]status.Status, error) {
			return nil, err1
		}

		cancelCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		messageCh := make(chan *pb.StreamStatusResponse)
		streamServer := &statusStreamServer{
			ctx:       cancelCtx,
			messageCh: messageCh,
		}
		err = server.StreamStatus(&pb.StreamStatusRequest{Every: durationpb.New(time.Second)}, streamServer)
		test.That(t, err, test.ShouldEqual, err1)
	})

	t.Run("failed StreamStatus server send", func(t *testing.T) {
		injectStatus := &inject.StatusService{}
		sMap := map[resource.Name]interface{}{
			status.Name: injectStatus,
		}
		server, err := newServer(sMap)
		test.That(t, err, test.ShouldBeNil)
		injectStatus.GetStatusFunc = func(ctx context.Context, resourceNames []resource.Name) ([]status.Status, error) {
			return []status.Status{{arm.Named("arm"), struct{}{}}}, nil
		}

		cancelCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		messageCh := make(chan *pb.StreamStatusResponse)
		streamServer := &statusStreamServer{
			ctx:       cancelCtx,
			messageCh: messageCh,
			fail:      true,
		}
		dur := 100 * time.Millisecond
		err = server.StreamStatus(&pb.StreamStatusRequest{Every: durationpb.New(dur)}, streamServer)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "send fail")
	})

	t.Run("timed out StreamStatus", func(t *testing.T) {
		injectStatus := &inject.StatusService{}
		sMap := map[resource.Name]interface{}{
			status.Name: injectStatus,
		}
		server, err := newServer(sMap)
		test.That(t, err, test.ShouldBeNil)
		injectStatus.GetStatusFunc = func(ctx context.Context, resourceNames []resource.Name) ([]status.Status, error) {
			return []status.Status{{arm.Named("arm"), struct{}{}}}, nil
		}

		timeoutCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		streamServer := &statusStreamServer{
			ctx:       timeoutCtx,
			messageCh: nil,
		}
		dur := 100 * time.Millisecond

		streamErr := server.StreamStatus(&pb.StreamStatusRequest{Every: durationpb.New(dur)}, streamServer)
		test.That(t, streamErr, test.ShouldResemble, context.DeadlineExceeded)
	})

	t.Run("working StreamStatus", func(t *testing.T) {
		injectStatus := &inject.StatusService{}
		sMap := map[resource.Name]interface{}{
			status.Name: injectStatus,
		}
		server, err := newServer(sMap)
		test.That(t, err, test.ShouldBeNil)
		injectStatus.GetStatusFunc = func(ctx context.Context, resourceNames []resource.Name) ([]status.Status, error) {
			return []status.Status{{arm.Named("arm"), struct{}{}}}, nil
		}

		cancelCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		messageCh := make(chan *pb.StreamStatusResponse)
		streamServer := &statusStreamServer{
			ctx:       cancelCtx,
			messageCh: messageCh,
			fail:      false,
		}
		dur := 100 * time.Millisecond
		var streamErr error
		start := time.Now()
		done := make(chan struct{})
		go func() {
			streamErr = server.StreamStatus(&pb.StreamStatusRequest{Every: durationpb.New(dur)}, streamServer)
			close(done)
		}()
		expectedStatus, err := structpb.NewStruct(map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		var messages []*pb.StreamStatusResponse
		messages = append(messages, <-messageCh)
		messages = append(messages, <-messageCh)
		messages = append(messages, <-messageCh)
		test.That(t, messages, test.ShouldResemble, []*pb.StreamStatusResponse{
			{Status: []*pb.Status{{Name: protoutils.ResourceNameToProto(arm.Named("arm")), Status: expectedStatus}}},
			{Status: []*pb.Status{{Name: protoutils.ResourceNameToProto(arm.Named("arm")), Status: expectedStatus}}},
			{Status: []*pb.Status{{Name: protoutils.ResourceNameToProto(arm.Named("arm")), Status: expectedStatus}}},
		})
		test.That(t, time.Since(start), test.ShouldBeGreaterThanOrEqualTo, 3*dur)
		test.That(t, time.Since(start), test.ShouldBeLessThanOrEqualTo, 6*dur)
		cancel()
		<-done
		test.That(t, streamErr, test.ShouldEqual, context.Canceled)
	})
}

type statusStreamServer struct {
	grpc.ServerStream // not set
	ctx               context.Context
	messageCh         chan<- *pb.StreamStatusResponse
	fail              bool
}

func (x *statusStreamServer) Context() context.Context {
	return x.ctx
}

func (x *statusStreamServer) Send(m *pb.StreamStatusResponse) error {
	if x.fail {
		return errors.New("send fail")
	}
	if x.messageCh == nil {
		return nil
	}
	x.messageCh <- m
	return nil
}
