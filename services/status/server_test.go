package status_test

// import (
// 	"context"
// 	"errors"
// 	"testing"

// 	"go.viam.com/test"
// 	"google.golang.org/protobuf/types/known/structpb"

// 	"go.viam.com/rdk/component/gps"
// 	"go.viam.com/rdk/component/imu"
// 	commonpb "go.viam.com/rdk/proto/api/common/v1"
// 	pb "go.viam.com/rdk/proto/api/service/status/v1"
// 	"go.viam.com/rdk/protoutils"
// 	"go.viam.com/rdk/resource"
// 	"go.viam.com/rdk/services/status"
// 	"go.viam.com/rdk/subtype"
// 	"go.viam.com/rdk/testutils"
// 	"go.viam.com/rdk/testutils/inject"
// 	rutils "go.viam.com/rdk/utils"
// )

// func newServer(sMap map[resource.Name]interface{}) (pb.StatusServiceServer, error) {
// 	sSvc, err := subtype.New(sMap)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return status.NewServer(sSvc), nil
// }

// func TestServerGetStatus(t *testing.T) {
// 	t.Run("no status service", func(t *testing.T) {
// 		sMap := map[resource.Name]interface{}{}
// 		server, err := newServer(sMap)
// 		test.That(t, err, test.ShouldBeNil)
// 		_, err = server.GetStatus(context.Background(), &pb.GetStatusRequest{})
// 		test.That(t, err, test.ShouldBeError, errors.New("resource \"rdk:service:status\" not found"))
// 	})

// 	t.Run("not status service", func(t *testing.T) {
// 		sMap := map[resource.Name]interface{}{status.Name: "not status"}
// 		server, err := newServer(sMap)
// 		test.That(t, err, test.ShouldBeNil)
// 		_, err = server.GetStatus(context.Background(), &pb.GetStatusRequest{})
// 		test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("status.Service", "string"))
// 	})

// 	t.Run("failed GetStatus", func(t *testing.T) {
// 		injectStatus := &inject.StatusService{}
// 		sMap := map[resource.Name]interface{}{
// 			status.Name: injectStatus,
// 		}
// 		server, err := newServer(sMap)
// 		test.That(t, err, test.ShouldBeNil)
// 		passedErr := errors.New("can't get status")
// 		injectStatus.GetStatusFunc = func(ctx context.Context) ([]resource.Name, error) {
// 			return nil, passedErr
// 		}
// 		_, err = server.GetStatus(context.Background(), &pb.GetStatusRequest{})
// 		test.That(t, err, test.ShouldBeError, passedErr)
// 	})

// 	t.Run("working GetStatus", func(t *testing.T) {
// 		injectStatus := &inject.StatusService{}
// 		sMap := map[resource.Name]interface{}{
// 			status.Name: injectStatus,
// 		}
// 		server, err := newServer(sMap)
// 		test.That(t, err, test.ShouldBeNil)
// 		names := []resource.Name{gps.Named("gps"), imu.Named("imu")}
// 		injectStatus.GetStatusFunc = func(ctx context.Context) ([]resource.Name, error) {
// 			return names, nil
// 		}

// 		resp, err := server.GetStatus(context.Background(), &pb.GetStatusRequest{})
// 		test.That(t, err, test.ShouldBeNil)

// 		convertedNames := make([]resource.Name, 0, len(resp.SensorNames))
// 		for _, rn := range resp.SensorNames {
// 			convertedNames = append(convertedNames, protoutils.ResourceNameFromProto(rn))
// 		}
// 		test.That(t, testutils.NewResourceNameSet(convertedNames...), test.ShouldResemble, testutils.NewResourceNameSet(names...))
// 	})
// }

// func TestServerGetReadings(t *testing.T) {
// 	t.Run("no status service", func(t *testing.T) {
// 		sMap := map[resource.Name]interface{}{}
// 		server, err := newServer(sMap)
// 		test.That(t, err, test.ShouldBeNil)
// 		_, err = server.GetReadings(context.Background(), &pb.GetReadingsRequest{})
// 		test.That(t, err, test.ShouldBeError, errors.New("resource \"rdk:service:status\" not found"))
// 	})

// 	t.Run("not status service", func(t *testing.T) {
// 		sMap := map[resource.Name]interface{}{status.Name: "not status"}
// 		server, err := newServer(sMap)
// 		test.That(t, err, test.ShouldBeNil)
// 		_, err = server.GetReadings(context.Background(), &pb.GetReadingsRequest{})
// 		test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("status.Service", "string"))
// 	})

// 	t.Run("failed GetReadings", func(t *testing.T) {
// 		injectStatus := &inject.StatusService{}
// 		sMap := map[resource.Name]interface{}{
// 			status.Name: injectStatus,
// 		}
// 		server, err := newServer(sMap)
// 		test.That(t, err, test.ShouldBeNil)
// 		passedErr := errors.New("can't get readings")
// 		injectStatus.GetReadingsFunc = func(ctx context.Context, status []resource.Name) ([]status.Readings, error) {
// 			return nil, passedErr
// 		}
// 		req := &pb.GetReadingsRequest{
// 			SensorNames: []*commonpb.ResourceName{},
// 		}
// 		_, err = server.GetReadings(context.Background(), req)
// 		test.That(t, err, test.ShouldBeError, passedErr)
// 	})

// 	t.Run("working GetReadings", func(t *testing.T) {
// 		injectStatus := &inject.StatusService{}
// 		sMap := map[resource.Name]interface{}{
// 			status.Name: injectStatus,
// 		}
// 		server, err := newServer(sMap)
// 		test.That(t, err, test.ShouldBeNil)
// 		iReading := status.Readings{Name: imu.Named("imu"), Readings: []interface{}{1.2, 2.3, 3.4}}
// 		gReading := status.Readings{Name: gps.Named("gps"), Readings: []interface{}{4.5, 5.6, 6.7}}
// 		readings := []status.Readings{iReading, gReading}
// 		expected := map[resource.Name]interface{}{
// 			iReading.Name: iReading.Readings,
// 			gReading.Name: gReading.Readings,
// 		}
// 		injectStatus.GetReadingsFunc = func(ctx context.Context, status []resource.Name) ([]status.Readings, error) {
// 			return readings, nil
// 		}
// 		req := &pb.GetReadingsRequest{
// 			SensorNames: []*commonpb.ResourceName{},
// 		}

// 		resp, err := server.GetReadings(context.Background(), req)
// 		test.That(t, err, test.ShouldBeNil)
// 		test.That(t, len(resp.Readings), test.ShouldEqual, 2)

// 		conv := func(rs []*structpb.Value) []interface{} {
// 			r := make([]interface{}, 0, len(resp.Readings))
// 			for _, value := range rs {
// 				r = append(r, value.AsInterface())
// 			}
// 			return r
// 		}

// 		observed := map[resource.Name]interface{}{
// 			protoutils.ResourceNameFromProto(resp.Readings[0].Name): conv(resp.Readings[0].Readings),
// 			protoutils.ResourceNameFromProto(resp.Readings[1].Name): conv(resp.Readings[1].Readings),
// 		}
// 		test.That(t, observed, test.ShouldResemble, expected)
// 	})
// }
