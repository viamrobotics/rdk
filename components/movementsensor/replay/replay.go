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
	goutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/internal/cloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils/contextutils"
)

const (
	timeFormat                       = time.RFC3339
	grpcConnectionTimeout            = 10 * time.Second
	initializePropertiesTimeout      = 10 * time.Second
	initializePropertiesLoopWaitTime = time.Second
	maxCacheSize                     = 1000
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

	// errEndOfDataset represents that the replay sensor has reached the end of the dataset.
	errEndOfDataset = errors.New("reached end of dataset")

	// errCloudConnectionFailure represents that the attempt to connect to the cloud failed.
	errCloudConnectionFailure = errors.New("failure to connect to the cloud")
	// ErrPropertiesNotInitializedYet represents that the properties are not initialized yet.
	ErrPropertiesNotInitializedYet = errors.New("Properties are not initialized yet")

	// errPropertiesFailedToInitialize represents that the properties failed to initialize.
	errPropertiesFailedToInitialize = errors.New("Properties failed to initialize")

	// errLinearVelocityNotSupported represents that the LinearVelocity method does not provide any data.
	errLinearVelocityNotSupported = errors.New("LinearVelocity is not supported")

	// errAngularVelocityNotSupported represents that the AngularVelocity endpoint does not provide any data.
	errAngularVelocityNotSupported = errors.New("AngularVelocity is not supported")

	// errOrientationNotSupported represents that the Orientation endpoint does not provide any data.
	errOrientationNotSupported = errors.New("Orientation is not supported")

	// errPositionNotSupported represents that the Position endpoint does not provide any data.
	errPositionNotSupported = errors.New("Position is not supported")

	// errCompassHeadingNotSupported represents that the CompassHeading endpoint does not provide any data.
	errCompassHeadingNotSupported = errors.New("CompassHeading is not supported")

	// errLinearAccelerationNotSupported represents that the LinearAcceleration endpoint does not provide any data.
	errLinearAccelerationNotSupported = errors.New("LinearAcceleration is not supported")

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

	var err error
	var startTime time.Time
	if cfg.Interval.Start != "" {
		startTime, err = time.Parse(timeFormat, cfg.Interval.Start)
		if err != nil {
			return nil, errors.New("invalid time format for start time (UTC), use RFC3339")
		}
		if startTime.After(time.Now()) {
			return nil, errors.New("invalid config, start time (UTC) must be in the past")
		}
	}

	var endTime time.Time
	if cfg.Interval.End != "" {
		endTime, err = time.Parse(timeFormat, cfg.Interval.End)
		if err != nil {
			return nil, errors.New("invalid time format for end time (UTC), use RFC3339")
		}
		if endTime.After(time.Now()) {
			return nil, errors.New("invalid config, end time (UTC) must be in the past")
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

type propertiesStatus int

const (
	notInitialized propertiesStatus = iota
	failedToInitialize
	successfullyInitialized
)

// replayMovementSensor is a movement sensor model that plays back pre-captured movement sensor data.
type replayMovementSensor struct {
	resource.Named
	logger golog.Logger

	cloudConnSvc cloud.ConnectionService
	cloudConn    rpc.ClientConn
	dataClient   datapb.DataServiceClient

	lastData map[method]string
	limit    uint64
	filter   *datapb.Filter

	cache map[method][]*cacheEntry

	mu                      sync.RWMutex
	closed                  bool
	activeBackgroundWorkers sync.WaitGroup
	propertiesStatus        propertiesStatus
	properties              movementsensor.Properties
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
		return nil, 0, errors.New("session closed")
	}

	if err := replay.ensurePropertiesAreInitialized(); err != nil {
		return nil, 0, err
	}

	if !replay.properties.PositionSupported {
		return nil, 0, errPositionNotSupported
	}

	data, err := replay.getDataFromCache(ctx, position)
	if err != nil {
		return nil, 0, err
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
		return r3.Vector{}, errors.New("session closed")
	}

	if err := replay.ensurePropertiesAreInitialized(); err != nil {
		return r3.Vector{}, err
	}

	if !replay.properties.LinearVelocitySupported {
		return r3.Vector{}, errLinearVelocityNotSupported
	}

	data, err := replay.getDataFromCache(ctx, linearVelocity)
	if err != nil {
		return r3.Vector{}, err
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
		return spatialmath.AngularVelocity{}, errors.New("session closed")
	}

	if err := replay.ensurePropertiesAreInitialized(); err != nil {
		return spatialmath.AngularVelocity{}, err
	}

	if !replay.properties.AngularVelocitySupported {
		return spatialmath.AngularVelocity{}, errAngularVelocityNotSupported
	}

	data, err := replay.getDataFromCache(ctx, angularVelocity)
	if err != nil {
		return spatialmath.AngularVelocity{}, err
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
		return r3.Vector{}, errors.New("session closed")
	}

	if err := replay.ensurePropertiesAreInitialized(); err != nil {
		return r3.Vector{}, err
	}

	if !replay.properties.LinearAccelerationSupported {
		return r3.Vector{}, errLinearAccelerationNotSupported
	}

	data, err := replay.getDataFromCache(ctx, linearAcceleration)
	if err != nil {
		return r3.Vector{}, err
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
		return 0., errors.New("session closed")
	}

	if err := replay.ensurePropertiesAreInitialized(); err != nil {
		return 0., err
	}

	if !replay.properties.CompassHeadingSupported {
		return 0., errCompassHeadingNotSupported
	}

	data, err := replay.getDataFromCache(ctx, compassHeading)
	if err != nil {
		return 0., err
	}

	return data.GetFields()["Compass"].GetNumberValue(), nil
}

// Orientation returns the next orientation from the cache as a spatialmath.Orientation created from a spatialmath.OrientationVector.
func (replay *replayMovementSensor) Orientation(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
	replay.mu.Lock()
	defer replay.mu.Unlock()
	if replay.closed {
		return nil, errors.New("session closed")
	}

	if err := replay.ensurePropertiesAreInitialized(); err != nil {
		return nil, err
	}

	if !replay.properties.OrientationSupported {
		return nil, errOrientationNotSupported
	}

	data, err := replay.getDataFromCache(ctx, orientation)
	if err != nil {
		return nil, err
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
	return &replay.properties, replay.ensurePropertiesAreInitialized()
}

// Accuracy is currently not defined for replay movement sensors.
func (replay *replayMovementSensor) Accuracy(ctx context.Context, extra map[string]interface{}) (map[string]float32, error) {
	return map[string]float32{}, movementsensor.ErrMethodUnimplementedAccuracy
}

// Close stops the replay movement sensor, closes its channels and its connections to the cloud.
func (replay *replayMovementSensor) Close(ctx context.Context) error {
	replay.mu.Lock()
	replay.closed = true
	replay.closeCloudConnection(ctx)
	replay.mu.Unlock()

	replay.activeBackgroundWorkers.Wait()
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
		return errors.New("session closed")
	}

	replayMovementSensorConfig, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

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

	replay.activeBackgroundWorkers.Add(1)
	go func() {
		func(ctx context.Context) {
			ctx, cancel := context.WithTimeout(ctx, initializePropertiesTimeout)
			defer cancel()
			if err := replay.initializeProperties(ctx); err != nil {
				replay.logger.Warn(err)
			}
		}(ctx)
		if replay.propertiesStatus == notInitialized {
			replay.propertiesStatus = failedToInitialize
		}
		replay.activeBackgroundWorkers.Done()
	}()

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
		return errEndOfDataset
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
	case linearVelocity:
		replay.properties.LinearVelocitySupported = supported
	case angularVelocity:
		replay.properties.AngularVelocitySupported = supported
	case orientation:
		replay.properties.OrientationSupported = supported
	case position:
		replay.properties.PositionSupported = supported
	case compassHeading:
		replay.properties.CompassHeadingSupported = supported
	case linearAcceleration:
		replay.properties.LinearAccelerationSupported = supported
	default:
		return errors.New("can't set property, invalid method: " + string(method))
	}
	return nil
}

// attemptToInitializeProperty will try to update the cache for the provided method.
func (replay *replayMovementSensor) attemptToInitializeProperty(ctx context.Context, method method) (bool, error) {
	replay.mu.Lock()
	defer replay.mu.Unlock()

	var initializedProperty bool
	if replay.closed {
		return initializedProperty, errors.New("session closed")
	}
	if err := replay.updateCache(ctx, method); err != nil && !strings.Contains(err.Error(), errEndOfDataset.Error()) {
		return initializedProperty, errors.Wrap(err, "could not update the cache")
	}
	if len(replay.cache[method]) != 0 {
		initializedProperty = true
		if err := replay.setProperty(method, true); err != nil {
			return initializedProperty, errors.Wrap(err, "could not set property")
		}
	}
	return initializedProperty, nil
}

// initializeProperties will set the properties by repeatedly attempting to poll data from all the methods
// until at least one of them returns data.
func (replay *replayMovementSensor) initializeProperties(ctx context.Context) error {
	var initializedAtLeastOneProperty, initializedProperty bool
	var err error
	// Repeatedly attempt to poll data from the movement sensor for each method until at least
	// one of the methods receives data.
	for {
		if !goutils.SelectContextOrWait(ctx, initializePropertiesLoopWaitTime) {
			return ctx.Err()
		}
		for _, method := range methodList {
			if initializedProperty, err = replay.attemptToInitializeProperty(ctx, method); err != nil {
				return err
			}
			if !initializedAtLeastOneProperty {
				initializedAtLeastOneProperty = initializedProperty
			}
		}
		// If at least one method successfully managed to initialize, we know that data reached the cloud and
		// that we can finish initializing the properties.
		if initializedAtLeastOneProperty {
			// Loop once more through all methods to ensure we didn't miss out on catching that they're supported
			for _, method := range methodList {
				if _, err = replay.attemptToInitializeProperty(ctx, method); err != nil {
					return err
				}
			}
			replay.mu.Lock()
			replay.propertiesStatus = successfullyInitialized
			replay.mu.Unlock()
			return nil
		}
	}
}

func (replay *replayMovementSensor) ensurePropertiesAreInitialized() error {
	switch replay.propertiesStatus {
	case notInitialized:
		return errPropertiesNotInitializedYet
	case failedToInitialize:
		return errPropertiesFailedToInitialize
	case successfullyInitialized:
		return nil
	default:
		return errors.New("this case should never happen")
	}
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

	_, conn, err := replay.cloudConnSvc.AcquireConnection(ctx)
	if err != nil {
		return err
	}
	dataServiceClient := datapb.NewDataServiceClient(conn)

	replay.cloudConn = conn
	replay.dataClient = dataServiceClient
	return nil
}
