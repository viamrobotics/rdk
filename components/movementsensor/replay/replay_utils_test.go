package replay

import (
	"context"
	"fmt"
	"math"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	datapb "go.viam.com/api/app/data/v1"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/components/movementsensor"
	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/internal/cloud"
	cloudinject "go.viam.com/rdk/internal/testutils/inject"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
)

const (
	testTime = "2000-01-01T12:00:%02dZ"
)

var ErrCloudConnection = errors.New("cloud connection error")

// mockDataServiceServer is a struct that includes unimplemented versions of all the Data Service endpoints. These
// can be overwritten to allow developers to trigger desired behaviors during testing.
type mockDataServiceServer struct {
	datapb.UnimplementedDataServiceServer
}

// TabularDataByFilter is a mocked version of the Data Service function of a similar name. It returns a response with
// data corresponding to the stored data associated with that function and index.
func (mDServer *mockDataServiceServer) TabularDataByFilter(ctx context.Context, req *datapb.TabularDataByFilterRequest,
) (*datapb.TabularDataByFilterResponse, error) {
	filter := req.DataRequest.GetFilter()
	last := req.DataRequest.GetLast()
	limit := req.DataRequest.GetLimit()

	var dataset []*datapb.TabularData
	var dataIndex int
	var err error
	for i := 0; i < int(limit); i++ {
		dataIndex, err = getNextDataAfterFilter(filter, last)
		if err != nil {
			if i == 0 {
				return nil, err
			}
			continue
		}

		// Call desired function
		data := createDataByMovementSensorMethod(method(filter.Method), dataIndex)

		timeReq, timeRec, err := timestampsFromIndex(dataIndex)
		if err != nil {
			return nil, err
		}

		last = fmt.Sprint(dataIndex)

		tabularData := &datapb.TabularData{
			Data:          data,
			TimeRequested: timeReq,
			TimeReceived:  timeRec,
		}
		dataset = append(dataset, tabularData)
	}

	// Construct response
	resp := &datapb.TabularDataByFilterResponse{
		Data: dataset,
		Last: last,
	}

	return resp, nil
}

// timestampsFromIndex uses the index of the data to generate a timeReceived and timeRequested for testing.
func timestampsFromIndex(index int) (*timestamppb.Timestamp, *timestamppb.Timestamp, error) {
	timeReq, err := time.Parse(time.RFC3339, fmt.Sprintf(testTime, index))
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed parsing time")
	}
	timeRec := timeReq.Add(time.Second)
	return timestamppb.New(timeReq), timestamppb.New(timeRec), nil
}

// getNextDataAfterFilter returns the index of the next data based on the provided.
func getNextDataAfterFilter(filter *datapb.Filter, last string) (int, error) {
	// Basic component part (source) filter
	if filter.ComponentName != "" && filter.ComponentName != validSource {
		return 0, ErrEndOfDataset
	}

	// Basic robot_id filter
	if filter.RobotId != "" && filter.RobotId != validRobotID {
		return 0, ErrEndOfDataset
	}

	// Basic location_id filter
	if len(filter.LocationIds) == 0 {
		return 0, errors.New("issue occurred with transmitting LocationIds to the cloud")
	}
	if filter.LocationIds[0] != "" && filter.LocationIds[0] != validLocationID {
		return 0, ErrEndOfDataset
	}

	// Basic organization_id filter
	if len(filter.OrganizationIds) == 0 {
		return 0, errors.New("issue occurred with transmitting OrganizationIds to the cloud")
	}
	if filter.OrganizationIds[0] != "" && filter.OrganizationIds[0] != validOrganizationID {
		return 0, ErrEndOfDataset
	}

	// Apply the time-based filter based on the seconds value in the start and end fields. Because our mock data
	// does not have timestamps associated with them but are ordered we can approximate the filtering
	// by sorting for the data in the list whose index is after the start second count and before the end second count.
	// For example, if there are 15 entries the start time is 2000-01-01T12:00:10Z and the end time is 2000-01-01T12:00:14Z,
	// we will return data from indices 10 to 14.
	startIntervalIndex := 0
	endIntervalIndex := math.MaxInt
	availableDataNum := allMethodsMaxDataLength[method(filter.Method)]

	if filter.Interval.Start != nil {
		startIntervalIndex = filter.Interval.Start.AsTime().Second()
	}
	if filter.Interval.End != nil {
		endIntervalIndex = filter.Interval.End.AsTime().Second()
	}
	if last == "" {
		return checkDataEndCondition(startIntervalIndex, endIntervalIndex, availableDataNum)
	}
	lastFileNum, err := strconv.Atoi(last)
	if err != nil {
		return 0, err
	}

	return checkDataEndCondition(lastFileNum+1, endIntervalIndex, availableDataNum)
}

// checkDataEndCondition will return the index of the data to be returned after checking the amount of data available and the end
// internal condition.
func checkDataEndCondition(i, endIntervalIndex, availableDataNum int) (int, error) {
	if i < endIntervalIndex && i < availableDataNum {
		return i, nil
	}
	return 0, ErrEndOfDataset
}

// createMockCloudDependencies creates a mockDataServiceServer and rpc client connection to it which is then
// stored in a mockCloudConnectionService.
func createMockCloudDependencies(ctx context.Context, t *testing.T, logger golog.Logger, validCloudConnection bool,
) (resource.Dependencies, func() error) {
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	test.That(t, rpcServer.RegisterServiceServer(
		ctx,
		&datapb.DataService_ServiceDesc,
		&mockDataServiceServer{},
		datapb.RegisterDataServiceHandlerFromEndpoint,
	), test.ShouldBeNil)

	go rpcServer.Serve(listener)

	conn, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)

	mockCloudConnectionService := &cloudinject.CloudConnectionService{
		Named: cloud.InternalServiceName.AsNamed(),
		Conn:  conn,
	}
	if !validCloudConnection {
		mockCloudConnectionService.AcquireConnectionErr = errTestCloudConnection
	}

	r := &inject.Robot{}
	rs := map[resource.Name]resource.Resource{}
	rs[cloud.InternalServiceName] = mockCloudConnectionService
	r.MockResourcesFromMap(rs)

	return resourcesFromDeps(t, r, []string{cloud.InternalServiceName.String()}), rpcServer.Stop
}

// createNewReplayMovementSensor will create a new replay movement sensor based on the provided config with either
// a valid or invalid data client.
func createNewReplayMovementSensor(ctx context.Context, t *testing.T, replayMovementSensorCfg *Config, validCloudConnection bool,
) (movementsensor.MovementSensor, resource.Dependencies, func() error, error) {
	logger := golog.NewTestLogger(t)

	resources, closeRPCFunc := createMockCloudDependencies(ctx, t, logger, validCloudConnection)

	cfg := resource.Config{ConvertedAttributes: replayMovementSensorCfg}
	replay, err := newReplayMovementSensor(ctx, resources, cfg, logger)

	return replay, resources, closeRPCFunc, err
}

// resourcesFromDeps returns a list of dependencies from the provided robot.
func resourcesFromDeps(t *testing.T, r robot.Robot, deps []string) resource.Dependencies {
	t.Helper()
	resources := resource.Dependencies{}
	for _, dep := range deps {
		resName, err := resource.NewFromString(dep)
		test.That(t, err, test.ShouldBeNil)
		res, err := r.ResourceByName(resName)
		if err == nil {
			// some resources are weakly linked
			resources[resName] = res
		}
	}
	return resources
}

// createDataByMovementSensorMethod will create the mocked structpb.Struct containing the next data returned by calls in tabular data.
func createDataByMovementSensorMethod(method method, index int) *structpb.Struct {
	var data structpb.Struct
	switch method {
	case position:
		data.Fields = map[string]*structpb.Value{
			"Latitude":  structpb.NewNumberValue(positionPointData[index].Lat()),
			"Longitude": structpb.NewNumberValue(positionPointData[index].Lng()),
			"Altitude":  structpb.NewNumberValue(positionAltitudeData[index]),
		}
	case linearVelocity:
		data.Fields = map[string]*structpb.Value{
			"X": structpb.NewNumberValue(linearVelocityData[index].X),
			"Y": structpb.NewNumberValue(linearVelocityData[index].Y),
			"Z": structpb.NewNumberValue(linearVelocityData[index].Z),
		}
	case angularVelocity:
		data.Fields = map[string]*structpb.Value{
			"X": structpb.NewNumberValue(angularVelocityData[index].X),
			"Y": structpb.NewNumberValue(angularVelocityData[index].Y),
			"Z": structpb.NewNumberValue(angularVelocityData[index].Z),
		}
	case linearAcceleration:
		data.Fields = map[string]*structpb.Value{
			"X": structpb.NewNumberValue(linearAccelerationData[index].X),
			"Y": structpb.NewNumberValue(linearAccelerationData[index].Y),
			"Z": structpb.NewNumberValue(linearAccelerationData[index].Z),
		}
	case compassHeading:
		data.Fields = map[string]*structpb.Value{
			"Compass": structpb.NewNumberValue(compassHeadingData[index]),
		}
	case orientation:
		data.Fields = map[string]*structpb.Value{
			"OX":    structpb.NewNumberValue(orientationData[index].OX),
			"OY":    structpb.NewNumberValue(orientationData[index].OY),
			"OZ":    structpb.NewNumberValue(orientationData[index].OZ),
			"Theta": structpb.NewNumberValue(orientationData[index].Theta),
		}
	}
	return &data
}

func testReplayMovementSensorMethodData(ctx context.Context, t *testing.T, replay movementsensor.MovementSensor, method method,
	index int,
) {
	var extra map[string]interface{}
	switch method {
	case position:
		point, altitude, err := replay.Position(ctx, extra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, point, test.ShouldResemble, positionPointData[index])
		test.That(t, altitude, test.ShouldResemble, positionAltitudeData[index])
	case linearVelocity:
		data, err := replay.LinearVelocity(ctx, extra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, data, test.ShouldResemble, linearVelocityData[index])
	case angularVelocity:
		data, err := replay.AngularVelocity(ctx, extra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, data, test.ShouldResemble, angularVelocityData[index])
	case linearAcceleration:
		data, err := replay.LinearAcceleration(ctx, extra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, data, test.ShouldResemble, linearAccelerationData[index])
	case compassHeading:
		data, err := replay.CompassHeading(ctx, extra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, data, test.ShouldEqual, compassHeadingData[index])
	case orientation:
		data, err := replay.Orientation(ctx, extra)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, data, test.ShouldResemble, orientationData[index])
	}
}

func testReplayMovementSensorMethodError(ctx context.Context, t *testing.T, replay movementsensor.MovementSensor, method method,
	expectedErr error,
) {
	var extra map[string]interface{}
	switch method {
	case position:
		point, altitude, err := replay.Position(ctx, extra)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, expectedErr.Error())
		test.That(t, point, test.ShouldBeNil)
		test.That(t, altitude, test.ShouldEqual, 0)
	case linearVelocity:
		data, err := replay.LinearVelocity(ctx, extra)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, expectedErr.Error())
		test.That(t, data, test.ShouldResemble, r3.Vector{})
	case angularVelocity:
		data, err := replay.AngularVelocity(ctx, extra)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, expectedErr.Error())
		test.That(t, data, test.ShouldResemble, spatialmath.AngularVelocity{})
	case linearAcceleration:
		data, err := replay.LinearAcceleration(ctx, extra)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, expectedErr.Error())
		test.That(t, data, test.ShouldResemble, r3.Vector{})
	case compassHeading:
		data, err := replay.CompassHeading(ctx, extra)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, expectedErr.Error())
		test.That(t, data, test.ShouldEqual, 0)
	case orientation:
		data, err := replay.Orientation(ctx, extra)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, expectedErr.Error())
		test.That(t, data, test.ShouldBeNil)
	}
}
