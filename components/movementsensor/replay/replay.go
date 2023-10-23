// Package replay implements a replay movement sensor that can return motion data.
package replay

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	datapb "go.viam.com/api/app/data/v1"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/internal/cloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils/contextutils"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	timeFormat               = time.RFC3339
	grpcConnectionTimeout    = 10 * time.Second
	dataReceivedLoopWaitTime = time.Second
	maxCacheSize             = 1000
)

type method string

const (
	position           method = "Position"
	linearVelocity     method = "LinearVelocity"
	angularVelocity    method = "AngularVelocity"
	linearAcceleration method = "LinearAcceleration"
	compassHeading     method = "CompassHeading"
	orientation        method = "Orientation"
)

var (
	// model is the model of a replay movement sensor.
	model = resource.DefaultModelFamily.WithModel("replay")

	// initializePropertiesTimeout defines the amount of time we allot to the attempt to initialize Properties.
	initializePropertiesTimeout = 180 * time.Second

	// ErrEndOfDataset represents that the replay sensor has reached the end of the dataset.
	ErrEndOfDataset = errors.New("reached end of dataset")

	// errPropertiesFailedToInitialize represents that the properties failed to initialize.
	errPropertiesFailedToInitialize = errors.New("Properties failed to initialize")

	// errCloudConnectionFailure represents that the attempt to connect to the cloud failed.
	errCloudConnectionFailure = errors.New("failure to connect to the cloud")

	// errSessionClosed represents that the session has ended.
	errSessionClosed = errors.New("session closed")

	// ererMessageNoDataAvailable indicates that no data was available for the given filter.
	errMessageNoDataAvailable = "no data available for given filter"

	// methodList is a list of all the base methods possible for a movement sensor to implement.
	methodList = []method{position, linearVelocity, angularVelocity, linearAcceleration, compassHeading, orientation}
)

func init() {
	resource.RegisterComponent(movementsensor.API, model, resource.Registration[movementsensor.MovementSensor, *Config]{
		Constructor: newReplayMovementSensor,
	})
}

// Validate checks that the config attributes are valid for a replay movement sensor.
func (cfg *Config) Validate(path string) ([]string, error) {
	if cfg.Source == "" {
		return nil, goutils.NewConfigValidationFieldRequiredError(path, "source")
	}

	if cfg.RobotID == "" {
		return nil, goutils.NewConfigValidationFieldRequiredError(path, "robot_id")
	}

	if cfg.LocationID == "" {
		return nil, goutils.NewConfigValidationFieldRequiredError(path, "location_id")
	}

	if cfg.OrganizationID == "" {
		return nil, goutils.NewConfigValidationFieldRequiredError(path, "organization_id")
	}
	if cfg.APIKey == "" {
		return nil, goutils.NewConfigValidationFieldRequiredError(path, "api_key")
	}
	if cfg.APIKeyID == "" {
		return nil, goutils.NewConfigValidationFieldRequiredError(path, "api_key_id")
	}

	var err error
	var startTime time.Time
	if cfg.Interval.Start != "" {
		startTime, err = time.Parse(timeFormat, cfg.Interval.Start)
		if err != nil {
			return nil, errors.New("invalid time format for start time (UTC), use RFC3339")
		}
	}

	var endTime time.Time
	if cfg.Interval.End != "" {
		endTime, err = time.Parse(timeFormat, cfg.Interval.End)
		if err != nil {
			return nil, errors.New("invalid time format for end time (UTC), use RFC3339")
		}
	}

	if cfg.Interval.Start != "" && cfg.Interval.End != "" && startTime.After(endTime) {
		return nil, errors.New("invalid config, end time (UTC) must be after start time (UTC)")
	}

	if cfg.BatchSize != nil && (*cfg.BatchSize > uint64(maxCacheSize) || *cfg.BatchSize == 0) {
		return nil, errors.Errorf("batch_size must be between 1 and %d", maxCacheSize)
	}

	return []string{cloud.InternalServiceName.String()}, nil
}

// Config describes how to configure the replay movement sensor.
type Config struct {
	Source         string       `json:"source,omitempty"`
	RobotID        string       `json:"robot_id,omitempty"`
	LocationID     string       `json:"location_id,omitempty"`
	OrganizationID string       `json:"organization_id,omitempty"`
	Interval       TimeInterval `json:"time_interval,omitempty"`
	BatchSize      *uint64      `json:"batch_size,omitempty"`
	APIKey         string       `json:"api_key,omitempty"`
	APIKeyID       string       `json:"api_key_id,omitempty"`
}

// TimeInterval holds the start and end time used to filter data.
type TimeInterval struct {
	Start string `json:"start,omitempty"`
	End   string `json:"end,omitempty"`
}

// cacheEntry stores data that was downloaded from a previous operation but has not yet been passed
// to the caller.
type cacheEntry struct {
	data          *structpb.Struct
	timeRequested *timestamppb.Timestamp
	timeReceived  *timestamppb.Timestamp
}

// replayMovementSensor is a movement sensor model that plays back pre-captured movement sensor data.
type replayMovementSensor struct {
	resource.Named
	logger golog.Logger

	APIKey       string
	APIKeyID     string
	cloudConnSvc cloud.ConnectionService
	cloudConn    rpc.ClientConn
	dataClient   datapb.DataServiceClient

	lastData map[method]string
	limit    uint64
	filter   *datapb.Filter

	cache map[method][]*cacheEntry

	mu         sync.RWMutex
	closed     bool
	properties movementsensor.Properties
}

// newReplayMovementSensor creates a new replay movement sensor based on the inputted config and dependencies.
func newReplayMovementSensor(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger golog.Logger) (
	movementsensor.MovementSensor, error,
) {
	replay := &replayMovementSensor{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
	}

	if err := replay.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}

	return replay, nil
}

// Position returns the next position from the cache, in the form of a geo.Point and altitude.
func (replay *replayMovementSensor) Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
	replay.mu.Lock()
	defer replay.mu.Unlock()
	if replay.closed {
		return nil, 0, errSessionClosed
	}

	if !replay.properties.PositionSupported {
		return nil, 0, movementsensor.ErrMethodUnimplementedPosition
	}

	data, err := replay.getDataFromCache(ctx, position)
	if err != nil {
		return nil, 0, err
	}

	if isNewPositionFormat(data) {
		coordStruct := data.GetFields()["coordinate"].GetStructValue()
		return geo.NewPoint(
				coordStruct.GetFields()["latitude"].GetNumberValue(),
				coordStruct.GetFields()["longitude"].GetNumberValue()),
			data.GetFields()["altitude_m"].GetNumberValue(), nil
	}
	return geo.NewPoint(
		data.GetFields()["Latitude"].GetNumberValue(),
		data.GetFields()["Longitude"].GetNumberValue()), data.GetFields()["Altitude"].GetNumberValue(), nil
}

// LinearVelocity returns the next linear velocity from the cache in the form of an r3.Vector.
func (replay *replayMovementSensor) LinearVelocity(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	replay.mu.Lock()
	defer replay.mu.Unlock()
	if replay.closed {
		return r3.Vector{}, errSessionClosed
	}

	if !replay.properties.LinearVelocitySupported {
		return r3.Vector{}, movementsensor.ErrMethodUnimplementedLinearVelocity
	}

	data, err := replay.getDataFromCache(ctx, linearVelocity)
	if err != nil {
		return r3.Vector{}, err
	}

	if isNewLinearVelocityFormat(data) {
		return newFormatStructToVector(data.GetFields()["linear_velocity"].GetStructValue()), nil
	}
	return r3.Vector{
		X: data.GetFields()["X"].GetNumberValue(),
		Y: data.GetFields()["Y"].GetNumberValue(),
		Z: data.GetFields()["Z"].GetNumberValue(),
	}, nil
}

// AngularVelocity returns the next angular velocity from the cache in the form of a spatialmath.AngularVelocity (r3.Vector).
func (replay *replayMovementSensor) AngularVelocity(ctx context.Context, extra map[string]interface{}) (
	spatialmath.AngularVelocity, error,
) {
	replay.mu.Lock()
	defer replay.mu.Unlock()
	if replay.closed {
		return spatialmath.AngularVelocity{}, errSessionClosed
	}

	if !replay.properties.AngularVelocitySupported {
		return spatialmath.AngularVelocity{}, movementsensor.ErrMethodUnimplementedAngularVelocity
	}

	data, err := replay.getDataFromCache(ctx, angularVelocity)
	if err != nil {
		return spatialmath.AngularVelocity{}, err
	}

	if isNewAngularVelocityFormat(data) {
		angularStruct := data.GetFields()["linear_velocity"].GetStructValue()
		return spatialmath.AngularVelocity{
			X: angularStruct.GetFields()["x"].GetNumberValue(),
			Y: angularStruct.GetFields()["y"].GetNumberValue(),
			Z: angularStruct.GetFields()["z"].GetNumberValue(),
		}, nil
	}
	return spatialmath.AngularVelocity{
		X: data.GetFields()["X"].GetNumberValue(),
		Y: data.GetFields()["Y"].GetNumberValue(),
		Z: data.GetFields()["Z"].GetNumberValue(),
	}, nil
}

// LinearAcceleration returns the next linear acceleration from the cache in the form of an r3.Vector.
func (replay *replayMovementSensor) LinearAcceleration(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	replay.mu.Lock()
	defer replay.mu.Unlock()
	if replay.closed {
		return r3.Vector{}, errSessionClosed
	}

	if !replay.properties.LinearAccelerationSupported {
		return r3.Vector{}, movementsensor.ErrMethodUnimplementedLinearAcceleration
	}

	data, err := replay.getDataFromCache(ctx, linearAcceleration)
	if err != nil {
		return r3.Vector{}, err
	}

	if isNewLinearAccelerationFormat(data) {
		return newFormatStructToVector(data.GetFields()["linear_acceleration"].GetStructValue()), nil
	}
	return r3.Vector{
		X: data.GetFields()["X"].GetNumberValue(),
		Y: data.GetFields()["Y"].GetNumberValue(),
		Z: data.GetFields()["Z"].GetNumberValue(),
	}, nil
}

// CompassHeading returns the next compass heading from the cache as a float64.
func (replay *replayMovementSensor) CompassHeading(ctx context.Context, extra map[string]interface{}) (float64, error) {
	replay.mu.Lock()
	defer replay.mu.Unlock()
	if replay.closed {
		return 0., errSessionClosed
	}

	if !replay.properties.CompassHeadingSupported {
		return 0., movementsensor.ErrMethodUnimplementedCompassHeading
	}

	data, err := replay.getDataFromCache(ctx, compassHeading)
	if err != nil {
		return 0., err
	}

	if isNewCompassHeadingFormat(data) {
		return data.GetFields()["value"].GetNumberValue(), nil
	}
	return data.GetFields()["Compass"].GetNumberValue(), nil
}

// Orientation returns the next orientation from the cache as a spatialmath.Orientation created from a spatialmath.OrientationVector.
func (replay *replayMovementSensor) Orientation(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
	replay.mu.Lock()
	defer replay.mu.Unlock()
	if replay.closed {
		return nil, errSessionClosed
	}

	if !replay.properties.OrientationSupported {
		return nil, movementsensor.ErrMethodUnimplementedOrientation
	}

	data, err := replay.getDataFromCache(ctx, orientation)
	if err != nil {
		return nil, err
	}

	if isNewOrientationFormat(data) {
		orientationStruct := data.GetFields()["orientation"].GetStructValue()
		return &spatialmath.OrientationVector{
			OX:    orientationStruct.GetFields()["ox"].GetNumberValue(),
			OY:    orientationStruct.GetFields()["oy"].GetNumberValue(),
			OZ:    orientationStruct.GetFields()["oz"].GetNumberValue(),
			Theta: orientationStruct.GetFields()["theta"].GetNumberValue(),
		}, nil
	}
	return &spatialmath.OrientationVector{
		OX:    data.GetFields()["OX"].GetNumberValue(),
		OY:    data.GetFields()["OY"].GetNumberValue(),
		OZ:    data.GetFields()["OZ"].GetNumberValue(),
		Theta: data.GetFields()["Theta"].GetNumberValue(),
	}, nil
}

// Properties returns the available properties for the given replay movement sensor.
func (replay *replayMovementSensor) Properties(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
	replay.mu.Lock()
	defer replay.mu.Unlock()
	return &replay.properties, nil
}

// Accuracy is currently not defined for replay movement sensors.
func (replay *replayMovementSensor) Accuracy(ctx context.Context, extra map[string]interface{}) (map[string]float32, error) {
	return map[string]float32{}, movementsensor.ErrMethodUnimplementedAccuracy
}

// Close stops the replay movement sensor, closes its channels and its connections to the cloud.
func (replay *replayMovementSensor) Close(ctx context.Context) error {
	replay.mu.Lock()
	defer replay.mu.Unlock()
	replay.closed = true
	replay.closeCloudConnection(ctx)

	return nil
}

// Readings returns all available data from the next entry stored in the cache.
func (replay *replayMovementSensor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	return movementsensor.Readings(ctx, replay, extra)
}

// Reconfigure finishes the bring up of the replay movement sensor by evaluating given arguments and setting up the required cloud
// connection as well as updates all required parameters upon a reconfiguration attempt, restarting the cloud connection in the process.
func (replay *replayMovementSensor) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	replay.mu.Lock()
	defer replay.mu.Unlock()
	if replay.closed {
		return errSessionClosed
	}

	replayMovementSensorConfig, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	replay.APIKey = replayMovementSensorConfig.APIKey
	replay.APIKeyID = replayMovementSensorConfig.APIKeyID

	cloudConnSvc, err := resource.FromDependencies[cloud.ConnectionService](deps, cloud.InternalServiceName)
	if err != nil {
		return err
	}

	// Update cloud connection if needed
	if replay.cloudConnSvc != cloudConnSvc {
		replay.closeCloudConnection(ctx)
		replay.cloudConnSvc = cloudConnSvc

		if err := replay.initCloudConnection(ctx); err != nil {
			replay.closeCloudConnection(ctx)
			return errors.Wrap(err, errCloudConnectionFailure.Error())
		}
	}

	if replayMovementSensorConfig.BatchSize == nil {
		replay.limit = 1
	} else {
		replay.limit = *replayMovementSensorConfig.BatchSize
	}

	replay.cache = map[method][]*cacheEntry{}
	for _, k := range methodList {
		replay.cache[k] = nil
	}

	replay.lastData = map[method]string{}
	for _, k := range methodList {
		replay.lastData[k] = ""
	}

	replay.filter = &datapb.Filter{
		ComponentName:   replayMovementSensorConfig.Source,
		RobotId:         replayMovementSensorConfig.RobotID,
		LocationIds:     []string{replayMovementSensorConfig.LocationID},
		OrganizationIds: []string{replayMovementSensorConfig.OrganizationID},
		Interval:        &datapb.CaptureInterval{},
	}

	if replayMovementSensorConfig.Interval.Start != "" {
		startTime, err := time.Parse(timeFormat, replayMovementSensorConfig.Interval.Start)
		if err != nil {
			replay.closeCloudConnection(ctx)
			return errors.New("invalid time format for start time, missed during config validation")
		}
		replay.filter.Interval.Start = timestamppb.New(startTime)
	}

	if replayMovementSensorConfig.Interval.End != "" {
		endTime, err := time.Parse(timeFormat, replayMovementSensorConfig.Interval.End)
		if err != nil {
			replay.closeCloudConnection(ctx)
			return errors.New("invalid time format for end time, missed during config validation")
		}
		replay.filter.Interval.End = timestamppb.New(endTime)
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, initializePropertiesTimeout)
	defer cancel()
	if err := replay.initializeProperties(ctxWithTimeout); err != nil {
		err = errors.Wrap(err, errPropertiesFailedToInitialize.Error())
		if errors.Is(err, context.DeadlineExceeded) {
			err = errors.Wrap(err, errMessageNoDataAvailable)
		}
		return err
	}

	return nil
}

// updateCache will update the cache with an additional batch of data downloaded from the cloud
// via TabularDataByFilter based on the given filter, and the last data accessed.
func (replay *replayMovementSensor) updateCache(ctx context.Context, method method) error {
	filter := replay.filter
	filter.Method = string(method)

	// Retrieve data from the cloud
	resp, err := replay.dataClient.TabularDataByFilter(ctx, &datapb.TabularDataByFilterRequest{
		DataRequest: &datapb.DataRequest{
			Filter:    filter,
			Limit:     replay.limit,
			Last:      replay.lastData[method],
			SortOrder: datapb.Order_ORDER_ASCENDING,
		},
		CountOnly: false,
	})
	if err != nil {
		return err
	}

	// Check if data exists
	if len(resp.GetData()) == 0 {
		return ErrEndOfDataset
	}
	replay.lastData[method] = resp.GetLast()

	// Add data to associated cache
	for _, dataResponse := range resp.Data {
		entry := &cacheEntry{
			data:          dataResponse.Data,
			timeRequested: dataResponse.GetTimeRequested(),
			timeReceived:  dataResponse.GetTimeReceived(),
		}
		replay.cache[method] = append(replay.cache[method], entry)
	}

	return nil
}

// addGRPCMetadata adds timestamps from the data response to the gRPC response header if one is found in the context.
func addGRPCMetadata(ctx context.Context, timeRequested, timeReceived *timestamppb.Timestamp) error {
	if stream := grpc.ServerTransportStreamFromContext(ctx); stream != nil {
		var grpcMetadata metadata.MD = make(map[string][]string)
		if timeRequested != nil {
			grpcMetadata.Set(contextutils.TimeRequestedMetadataKey, timeRequested.AsTime().Format(time.RFC3339Nano))
		}
		if timeReceived != nil {
			grpcMetadata.Set(contextutils.TimeReceivedMetadataKey, timeReceived.AsTime().Format(time.RFC3339Nano))
		}
		if err := grpc.SetHeader(ctx, grpcMetadata); err != nil {
			return err
		}
	}

	return nil
}

func (replay *replayMovementSensor) setProperty(method method, supported bool) error {
	switch method {
	case position:
		replay.properties.PositionSupported = supported
	case linearVelocity:
		replay.properties.LinearVelocitySupported = supported
	case angularVelocity:
		replay.properties.AngularVelocitySupported = supported
	case linearAcceleration:
		replay.properties.LinearAccelerationSupported = supported
	case compassHeading:
		replay.properties.CompassHeadingSupported = supported
	case orientation:
		replay.properties.OrientationSupported = supported
	default:
		return errors.New("can't set property, invalid method: " + string(method))
	}
	return nil
}

// attemptToGetData will try to update the cache for the provided method. Returns a bool that
// indicates whether or not the endpoint has data.
func (replay *replayMovementSensor) attemptToGetData(ctx context.Context, method method) (bool, error) {
	if replay.closed {
		return false, errSessionClosed
	}
	if err := replay.updateCache(ctx, method); err != nil && !strings.Contains(err.Error(), ErrEndOfDataset.Error()) {
		return false, errors.Wrap(err, "could not update the cache")
	}
	return len(replay.cache[method]) != 0, nil
}

// initializeProperties will set the properties by repeatedly polling the cloud for data from
// the available methods until at least one returns data. The properties are set to
// `true` for the endpoints that returned data.
func (replay *replayMovementSensor) initializeProperties(ctx context.Context) error {
	dataReceived := make(map[method]bool)
	var err error
	// Repeatedly attempt to poll data from the movement sensor for each method until at least
	// one of the methods receives data.
	for {
		if !goutils.SelectContextOrWait(ctx, dataReceivedLoopWaitTime) {
			return ctx.Err()
		}
		for _, method := range methodList {
			if dataReceived[method], err = replay.attemptToGetData(ctx, method); err != nil {
				return err
			}
		}
		// If at least one method successfully managed to return data, we know
		// that we can finish initializing the properties.
		if slices.Contains(maps.Values(dataReceived), true) {
			break
		}
	}
	// Loop once more through all methods to ensure we didn't miss out on catching that they're supported
	for _, method := range methodList {
		if dataReceived[method], err = replay.attemptToGetData(ctx, method); err != nil {
			return err
		}
	}

	for method, supported := range dataReceived {
		if err := replay.setProperty(method, supported); err != nil {
			return err
		}
	}
	return nil
}

// getDataFromCache retrieves the next cached data and removes it from the cache. It assumes the write lock is being held.
func (replay *replayMovementSensor) getDataFromCache(ctx context.Context, method method) (*structpb.Struct, error) {
	// If no data remains in the cache, download a new batch of data
	if len(replay.cache[method]) == 0 {
		if err := replay.updateCache(ctx, method); err != nil {
			return nil, errors.Wrapf(err, "could not update the cache")
		}
	}

	// Grab the next cached data and update the associated cache
	methodCache := replay.cache[method]
	entry := methodCache[0]
	replay.cache[method] = methodCache[1:]

	if err := addGRPCMetadata(ctx, entry.timeRequested, entry.timeReceived); err != nil {
		return nil, errors.Wrapf(err, "adding GRPC metadata failed")
	}

	return entry.data, nil
}

// closeCloudConnection closes all parts of the cloud connection used by the replay movement sensor.
func (replay *replayMovementSensor) closeCloudConnection(ctx context.Context) {
	if replay.cloudConn != nil {
		goutils.UncheckedError(replay.cloudConn.Close())
	}

	if replay.cloudConnSvc != nil {
		goutils.UncheckedError(replay.cloudConnSvc.Close(ctx))
	}
}

// initCloudConnection creates a rpc client connection and data service.
func (replay *replayMovementSensor) initCloudConnection(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, grpcConnectionTimeout)
	defer cancel()

	_, conn, err := replay.cloudConnSvc.AcquireConnectionAPIKey(ctx, replay.APIKey, replay.APIKeyID)
	if err != nil {
		return err
	}
	dataServiceClient := datapb.NewDataServiceClient(conn)

	replay.cloudConn = conn
	replay.dataClient = dataServiceClient
	return nil
}

func isNewPositionFormat(data *structpb.Struct) bool {
	// if coordinate key exists in map, assume it is new format
	_, ok := data.GetFields()["coordinate"]
	return ok
}

func isNewLinearVelocityFormat(data *structpb.Struct) bool {
	_, ok := data.GetFields()["linear_velocity"]
	return ok
}

func isNewAngularVelocityFormat(data *structpb.Struct) bool {
	_, ok := data.GetFields()["angular_velocity"]
	return ok
}

func isNewLinearAccelerationFormat(data *structpb.Struct) bool {
	_, ok := data.GetFields()["linear_acceleration"]
	return ok
}

func isNewCompassHeadingFormat(data *structpb.Struct) bool {
	_, ok := data.GetFields()["value"]
	return ok
}

func isNewOrientationFormat(data *structpb.Struct) bool {
	_, ok := data.GetFields()["orientation"]
	return ok
}

func newFormatStructToVector(data *structpb.Struct) r3.Vector {
	return r3.Vector{
		X: data.GetFields()["x"].GetNumberValue(),
		Y: data.GetFields()["y"].GetNumberValue(),
		Z: data.GetFields()["z"].GetNumberValue(),
	}
}
