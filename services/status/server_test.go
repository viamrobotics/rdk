package status_test

import (
	"context"
	"errors"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/gps"
	"go.viam.com/rdk/component/imu"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/service/status/v1"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/status"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

func newServer(sMap map[resource.Name]interface{}) (pb.StatusServiceServer, error) {
	sSvc, err := subtype.New(sMap)
	if err != nil {
		return nil, err
	}
	return status.NewServer(sSvc), nil
}

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
				"unable to convert status for \"rdk:component:arm/arm\" to a form acceptable to structpb.NewStruct: data of type int not a struct or a map-like object",
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
		aStatus := status.Status{Name: arm.Named("arm"), Status: status.DefaultStatus{Exists: true}}
		readings := []status.Status{aStatus}
		expected := map[resource.Name]interface{}{
			aStatus.Name: map[string]interface{}{"exists": true},
		}
		injectStatus.GetStatusFunc = func(ctx context.Context, status []resource.Name) ([]status.Status, error) {
			return readings, nil
		}
		req := &pb.GetStatusRequest{
			ResourceNames: []*commonpb.ResourceName{},
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
		aStatus := status.Status{Name: arm.Named("arm"), Status: status.DefaultStatus{Exists: true}}
		readings := []status.Status{iStatus, gStatus, aStatus}
		expected := map[resource.Name]interface{}{
			iStatus.Name: map[string]interface{}{"abc": []interface{}{1.2, 2.3, 3.4}},
			gStatus.Name: map[string]interface{}{"efg": []interface{}{"hello"}},
			aStatus.Name: map[string]interface{}{"exists": true},
		}
		injectStatus.GetStatusFunc = func(ctx context.Context, status []resource.Name) ([]status.Status, error) {
			return readings, nil
		}
		req := &pb.GetStatusRequest{
			ResourceNames: []*commonpb.ResourceName{},
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
}
