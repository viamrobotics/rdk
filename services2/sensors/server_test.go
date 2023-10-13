package sensors_test

import (
	"context"
	"errors"
	"testing"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/sensors/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/components/movementsensor"
	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/sensors"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

func newServer(sMap map[resource.Name]sensors.Service) (pb.SensorsServiceServer, error) {
	coll, err := resource.NewAPIResourceCollection(sensors.API, sMap)
	if err != nil {
		return nil, err
	}
	return sensors.NewRPCServiceServer(coll).(pb.SensorsServiceServer), nil
}

func TestServerGetSensors(t *testing.T) {
	t.Run("no sensors service", func(t *testing.T) {
		sMap := map[resource.Name]sensors.Service{}
		server, err := newServer(sMap)
		test.That(t, err, test.ShouldBeNil)
		_, err = server.GetSensors(context.Background(), &pb.GetSensorsRequest{})
		test.That(t, err, test.ShouldBeError, errors.New("resource \"rdk:service:sensors/\" not found"))
	})

	t.Run("failed Sensors", func(t *testing.T) {
		injectSensors := &inject.SensorsService{}
		sMap := map[resource.Name]sensors.Service{
			testSvcName1: injectSensors,
		}
		server, err := newServer(sMap)
		test.That(t, err, test.ShouldBeNil)
		passedErr := errors.New("can't get sensors")
		injectSensors.SensorsFunc = func(ctx context.Context, extra map[string]interface{}) ([]resource.Name, error) {
			return nil, passedErr
		}
		_, err = server.GetSensors(context.Background(), &pb.GetSensorsRequest{Name: testSvcName1.ShortName()})
		test.That(t, err, test.ShouldBeError, passedErr)
	})

	t.Run("working Sensors", func(t *testing.T) {
		injectSensors := &inject.SensorsService{}
		sMap := map[resource.Name]sensors.Service{
			testSvcName1: injectSensors,
		}
		server, err := newServer(sMap)
		test.That(t, err, test.ShouldBeNil)

		var extraOptions map[string]interface{}
		names := []resource.Name{movementsensor.Named("gps"), movementsensor.Named("imu")}
		injectSensors.SensorsFunc = func(ctx context.Context, extra map[string]interface{}) ([]resource.Name, error) {
			extraOptions = extra
			return names, nil
		}
		extra := map[string]interface{}{"foo": "Sensors"}
		ext, err := protoutils.StructToStructPb(extra)
		test.That(t, err, test.ShouldBeNil)

		resp, err := server.GetSensors(context.Background(), &pb.GetSensorsRequest{Name: testSvcName1.ShortName(), Extra: ext})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, extraOptions, test.ShouldResemble, extra)

		convertedNames := make([]resource.Name, 0, len(resp.SensorNames))
		for _, rn := range resp.SensorNames {
			convertedNames = append(convertedNames, rprotoutils.ResourceNameFromProto(rn))
		}
		test.That(t, testutils.NewResourceNameSet(convertedNames...), test.ShouldResemble, testutils.NewResourceNameSet(names...))
	})
}

func TestServerGetReadings(t *testing.T) {
	t.Run("no sensors service", func(t *testing.T) {
		sMap := map[resource.Name]sensors.Service{}
		server, err := newServer(sMap)
		test.That(t, err, test.ShouldBeNil)
		_, err = server.GetReadings(context.Background(), &pb.GetReadingsRequest{})
		test.That(t, err, test.ShouldBeError, errors.New("resource \"rdk:service:sensors/\" not found"))
	})

	t.Run("failed Readings", func(t *testing.T) {
		injectSensors := &inject.SensorsService{}
		sMap := map[resource.Name]sensors.Service{
			testSvcName1: injectSensors,
		}
		server, err := newServer(sMap)
		test.That(t, err, test.ShouldBeNil)
		passedErr := errors.New("can't get readings")
		injectSensors.ReadingsFunc = func(
			ctx context.Context, sensors []resource.Name, extra map[string]interface{},
		) ([]sensors.Readings, error) {
			return nil, passedErr
		}
		req := &pb.GetReadingsRequest{
			Name:        testSvcName1.ShortName(),
			SensorNames: []*commonpb.ResourceName{},
		}
		_, err = server.GetReadings(context.Background(), req)
		test.That(t, err, test.ShouldBeError, passedErr)
	})

	t.Run("working Readings", func(t *testing.T) {
		injectSensors := &inject.SensorsService{}
		sMap := map[resource.Name]sensors.Service{
			testSvcName1: injectSensors,
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
		var extraOptions map[string]interface{}
		injectSensors.ReadingsFunc = func(
			ctx context.Context, sensors []resource.Name, extra map[string]interface{},
		) ([]sensors.Readings, error) {
			extraOptions = extra
			return readings, nil
		}
		extra := map[string]interface{}{"foo": "Readings"}
		ext, err := protoutils.StructToStructPb(extra)
		test.That(t, err, test.ShouldBeNil)

		req := &pb.GetReadingsRequest{
			Name:        testSvcName1.ShortName(),
			SensorNames: []*commonpb.ResourceName{},
			Extra:       ext,
		}
		resp, err := server.GetReadings(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(resp.Readings), test.ShouldEqual, 2)
		test.That(t, extraOptions, test.ShouldResemble, extra)

		conv := func(rs map[string]*structpb.Value) map[string]interface{} {
			r := map[string]interface{}{}
			for k, value := range rs {
				r[k] = value.AsInterface()
			}
			return r
		}

		observed := map[resource.Name]interface{}{
			rprotoutils.ResourceNameFromProto(resp.Readings[0].Name): conv(resp.Readings[0].Readings),
			rprotoutils.ResourceNameFromProto(resp.Readings[1].Name): conv(resp.Readings[1].Readings),
		}
		test.That(t, observed, test.ShouldResemble, expected)
	})
}

func TestServerDoCommand(t *testing.T) {
	resourceMap := map[resource.Name]sensors.Service{
		testSvcName1: &inject.SensorsService{
			DoCommandFunc: testutils.EchoFunc,
		},
	}
	server, err := newServer(resourceMap)
	test.That(t, err, test.ShouldBeNil)

	cmd, err := protoutils.StructToStructPb(testutils.TestCommand)
	test.That(t, err, test.ShouldBeNil)
	doCommandRequest := &commonpb.DoCommandRequest{
		Name:    testSvcName1.ShortName(),
		Command: cmd,
	}
	doCommandResponse, err := server.DoCommand(context.Background(), doCommandRequest)
	test.That(t, err, test.ShouldBeNil)

	// Assert that do command response is an echoed request.
	respMap := doCommandResponse.Result.AsMap()
	test.That(t, respMap["command"], test.ShouldResemble, "test")
	test.That(t, respMap["data"], test.ShouldResemble, 500.0)
}
