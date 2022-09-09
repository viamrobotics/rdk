package sensors_test

import (
	"context"
	"errors"
	"testing"

	"go.viam.com/test"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/components/movementsensor"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/service/sensors/v1"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/sensors"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

func newServer(sMap map[resource.Name]interface{}) (pb.SensorsServiceServer, error) {
	sSvc, err := subtype.New(sMap)
	if err != nil {
		return nil, err
	}
	return sensors.NewServer(sSvc), nil
}

func TestServerGetSensors(t *testing.T) {
	t.Run("no sensors service", func(t *testing.T) {
		sMap := map[resource.Name]interface{}{}
		server, err := newServer(sMap)
		test.That(t, err, test.ShouldBeNil)
		_, err = server.GetSensors(context.Background(), &pb.GetSensorsRequest{})
		test.That(t, err, test.ShouldBeError, errors.New("resource \"rdk:service:sensors/\" not found"))
	})

	t.Run("not sensors service", func(t *testing.T) {
		sMap := map[resource.Name]interface{}{sensors.Named(testSvcName1): "not sensors"}
		server, err := newServer(sMap)
		test.That(t, err, test.ShouldBeNil)
		_, err = server.GetSensors(context.Background(), &pb.GetSensorsRequest{Name: testSvcName1})
		test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("sensors.Service", "string"))
	})

	t.Run("failed GetSensors", func(t *testing.T) {
		injectSensors := &inject.SensorsService{}
		sMap := map[resource.Name]interface{}{
			sensors.Named(testSvcName1): injectSensors,
		}
		server, err := newServer(sMap)
		test.That(t, err, test.ShouldBeNil)
		passedErr := errors.New("can't get sensors")
		injectSensors.GetSensorsFunc = func(ctx context.Context) ([]resource.Name, error) {
			return nil, passedErr
		}
		_, err = server.GetSensors(context.Background(), &pb.GetSensorsRequest{Name: testSvcName1})
		test.That(t, err, test.ShouldBeError, passedErr)
	})

	t.Run("working GetSensors", func(t *testing.T) {
		injectSensors := &inject.SensorsService{}
		sMap := map[resource.Name]interface{}{
			sensors.Named(testSvcName1): injectSensors,
		}
		server, err := newServer(sMap)
		test.That(t, err, test.ShouldBeNil)
		names := []resource.Name{movementsensor.Named("gps"), movementsensor.Named("imu")}
		injectSensors.GetSensorsFunc = func(ctx context.Context) ([]resource.Name, error) {
			return names, nil
		}

		resp, err := server.GetSensors(context.Background(), &pb.GetSensorsRequest{Name: testSvcName1})
		test.That(t, err, test.ShouldBeNil)

		convertedNames := make([]resource.Name, 0, len(resp.SensorNames))
		for _, rn := range resp.SensorNames {
			convertedNames = append(convertedNames, protoutils.ResourceNameFromProto(rn))
		}
		test.That(t, testutils.NewResourceNameSet(convertedNames...), test.ShouldResemble, testutils.NewResourceNameSet(names...))
	})
}

func TestServerGetReadings(t *testing.T) {
	t.Run("no sensors service", func(t *testing.T) {
		sMap := map[resource.Name]interface{}{}
		server, err := newServer(sMap)
		test.That(t, err, test.ShouldBeNil)
		_, err = server.GetReadings(context.Background(), &pb.GetReadingsRequest{})
		test.That(t, err, test.ShouldBeError, errors.New("resource \"rdk:service:sensors/\" not found"))
	})

	t.Run("not sensors service", func(t *testing.T) {
		sMap := map[resource.Name]interface{}{sensors.Named(testSvcName1): "not sensors"}
		server, err := newServer(sMap)
		test.That(t, err, test.ShouldBeNil)
		_, err = server.GetReadings(context.Background(), &pb.GetReadingsRequest{Name: testSvcName1})
		test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("sensors.Service", "string"))
	})

	t.Run("failed GetReadings", func(t *testing.T) {
		injectSensors := &inject.SensorsService{}
		sMap := map[resource.Name]interface{}{
			sensors.Named(testSvcName1): injectSensors,
		}
		server, err := newServer(sMap)
		test.That(t, err, test.ShouldBeNil)
		passedErr := errors.New("can't get readings")
		injectSensors.GetReadingsFunc = func(ctx context.Context, sensors []resource.Name) ([]sensors.Readings, error) {
			return nil, passedErr
		}
		req := &pb.GetReadingsRequest{
			Name:        testSvcName1,
			SensorNames: []*commonpb.ResourceName{},
		}
		_, err = server.GetReadings(context.Background(), req)
		test.That(t, err, test.ShouldBeError, passedErr)
	})

	t.Run("working GetReadings", func(t *testing.T) {
		injectSensors := &inject.SensorsService{}
		sMap := map[resource.Name]interface{}{
			sensors.Named(testSvcName1): injectSensors,
		}
		server, err := newServer(sMap)
		test.That(t, err, test.ShouldBeNil)
		iReading := sensors.Readings{Name: movementsensor.Named("imu"), Readings: map[string]interface{}{"a": 1.2, "b": 2.3, "c": 3.4}}
		gReading := sensors.Readings{Name: movementsensor.Named("gps"), Readings: map[string]interface{}{"a": 4.5, "b": 5.6, "c": 6.7}}
		readings := []sensors.Readings{iReading, gReading}
		expected := map[resource.Name]interface{}{
			iReading.Name: iReading.Readings,
			gReading.Name: gReading.Readings,
		}
		injectSensors.GetReadingsFunc = func(ctx context.Context, sensors []resource.Name) ([]sensors.Readings, error) {
			return readings, nil
		}
		req := &pb.GetReadingsRequest{
			Name:        testSvcName1,
			SensorNames: []*commonpb.ResourceName{},
		}

		resp, err := server.GetReadings(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp.Readings), test.ShouldEqual, 2)

		conv := func(rs map[string]*structpb.Value) map[string]interface{} {
			r := map[string]interface{}{}
			for k, value := range rs {
				r[k] = value.AsInterface()
			}
			return r
		}

		observed := map[resource.Name]interface{}{
			protoutils.ResourceNameFromProto(resp.Readings[0].Name): conv(resp.Readings[0].Readings),
			protoutils.ResourceNameFromProto(resp.Readings[1].Name): conv(resp.Readings[1].Readings),
		}
		test.That(t, observed, test.ShouldResemble, expected)
	})
}
