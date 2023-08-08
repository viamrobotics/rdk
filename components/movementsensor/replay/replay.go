// Package replay implements a replay movement sensor that can return motion data.
package replay

import (
	"context"
	"sync"
	"time"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"

	"github.com/edaniels/golog"
	datapb "go.viam.com/api/app/data/v1"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/internal/cloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	timeFormat            = time.RFC3339
	grpcConnectionTimeout = 10 * time.Second
	downloadTimeout       = 30 * time.Second
	maxCacheSize          = 100
)

var (
	// model is the model of a replay movement sensor.
	model = resource.DefaultModelFamily.WithModel("replay")

	// ErrEndOfDataset represents that the replay sensor has reached the end of the dataset.
	ErrEndOfDataset = errors.New("reached end of dataset")

	defaultCacheMap = map[string][]*cacheEntry{
		"position":            nil,
		"orientation":         nil,
		"angular_velocity":    nil,
		"linear_velocity":     nil,
		"compass_heading":     nil,
		"linear_acceleration": nil,
	}
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

type Config struct {
	Source     string          `json:"source,omitempty"`
	RobotID    string          `json:"robot_id,omitempty"`
	Interval   TimeInterval    `json:"time_interval,omitempty"`
	BatchSize  *uint64         `json:"batch_size,omitempty"`
	Properties map[string]bool `json:"properties,omitempty"`
}

// TimeInterval holds the start and end time used to filter data.
type TimeInterval struct {
	Start string `json:"start,omitempty"`
	End   string `json:"end,omitempty"`
}

type positionData struct {
	point    *geo.Point
	altitude float64
}

type movementSensorData struct {
	position           positionData
	linearVelocity     r3.Vector
	angularVelocity    spatialmath.AngularVelocity
	linearAcceleration r3.Vector
	compassHeading     float64
	orientation        spatialmath.Orientation
}

// cacheEntry stores data that was downloaded from a previous operation but has not yet been passed
// to the caller.
type cacheEntry struct {
	id            *datapb.BinaryID
	data          movementSensorData
	timeRequested *timestamppb.Timestamp
	timeReceived  *timestamppb.Timestamp
	err           error
}

// replayMovementSensor is a movement sensor model that plays back pre-captured point cloud data.
type replayMovementSensor struct {
	resource.Named
	logger golog.Logger

	cloudConnSvc cloud.ConnectionService
	cloudConn    rpc.ClientConn
	dataClient   datapb.DataServiceClient

	lastData string
	limit    uint64
	filter   *datapb.Filter

	cache      map[string][]*cacheEntry
	properties movementsensor.Properties

	mu     sync.RWMutex
	closed bool
}

// newReplayMovementSensor creates a new replay movement sensor based on the inputted config and dependencies.
func newReplayMovementSensor(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger golog.Logger) (movementsensor.MovementSensor, error) {
	replay := &replayMovementSensor{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
	}

	if err := replay.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}

	return replay, nil
}

// Position returns the next position from the cache.
func (replay *replayMovementSensor) Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
	replay.mu.Lock()
	defer replay.mu.Unlock()
	if replay.closed {
		return nil, 0, errors.New("session closed")
	}

	if !replay.properties.PositionSupported {
		return nil, 0, errors.New("position is not supported, add position_supported to the properties to enable")
	}

	entry, err := replay.getDataFromCache(ctx, "position", replay.cache["position"])
	if err != nil {
		return nil, 0, err
	}

	return entry.data.position.point, entry.data.position.altitude, nil
}

// LinearVelocity returns the next linear velocity from the cache.
func (replay *replayMovementSensor) LinearVelocity(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	replay.mu.Lock()
	defer replay.mu.Unlock()
	if replay.closed {
		return r3.Vector{}, errors.New("session closed")
	}

	if !replay.properties.LinearVelocitySupported {
		return r3.Vector{}, errors.New("linear velocity is not supported, add linear_velocity_supported to the properties to enable")
	}

	entry, err := replay.getDataFromCache(ctx, "linear_velocity", replay.cache["linear_velocity"])
	if err != nil {
		return r3.Vector{}, err
	}

	return entry.data.linearVelocity, nil
}

// AngularVelocity returns the next angular velocity from the cache.
func (replay *replayMovementSensor) AngularVelocity(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) {
	replay.mu.Lock()
	defer replay.mu.Unlock()
	if replay.closed {
		return spatialmath.AngularVelocity{}, errors.New("session closed")
	}

	if !replay.properties.AngularVelocitySupported {
		return spatialmath.AngularVelocity{}, errors.New("angular velocity is not supported, add angular_velocity_supported to the properties to enable")
	}

	entry, err := replay.getDataFromCache(ctx, "angular_velocity", replay.cache["angular_velocity"])
	if err != nil {
		return spatialmath.AngularVelocity{}, err
	}

	return entry.data.angularVelocity, nil
}

// LinearAcceleration returns the next linear acceleration from the cache.
func (replay *replayMovementSensor) LinearAcceleration(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	replay.mu.Lock()
	defer replay.mu.Unlock()
	if replay.closed {
		return r3.Vector{}, errors.New("session closed")
	}

	if !replay.properties.LinearAccelerationSupported {
		return r3.Vector{}, errors.New("linear acceleration is not supported, add linear_acceleration_supported to the properties to enable")
	}

	entry, err := replay.getDataFromCache(ctx, "linear_acceleration", replay.cache["linear_acceleration"])
	if err != nil {
		return r3.Vector{}, err
	}

	return entry.data.linearAcceleration, nil
}

// CompassHeading returns the next compass heading from the cache.
func (replay *replayMovementSensor) CompassHeading(ctx context.Context, extra map[string]interface{}) (float64, error) {
	replay.mu.Lock()
	defer replay.mu.Unlock()
	if replay.closed {
		return 0, errors.New("session closed")
	}

	if !replay.properties.CompassHeadingSupported {
		return 0, errors.New("compass heading is not supported, add compass_heading_supported to the properties to enable")
	}

	entry, err := replay.getDataFromCache(ctx, "compass_heading", replay.cache["compass_heading"])
	if err != nil {
		return 0, err
	}

	return entry.data.compassHeading, nil
}

// Orientation returns the next orientation from the cache.
func (replay *replayMovementSensor) Orientation(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
	replay.mu.Lock()
	defer replay.mu.Unlock()
	if replay.closed {
		return nil, errors.New("session closed")
	}

	if !replay.properties.OrientationSupported {
		return nil, errors.New("orientation is not supported, add orientation_supported to the properties to enable")
	}

	entry, err := replay.getDataFromCache(ctx, "orientation", replay.cache["orientation"])
	if err != nil {
		return nil, err
	}

	return entry.data.orientation, nil
}

// Properties returns the available properties for the given replay movement sensor.
func (replay *replayMovementSensor) Properties(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
	return &replay.properties, nil
}

// Accuracy is currently not defined for replay movement sensors.
func (replay *replayMovementSensor) Accuracy(ctx context.Context, extra map[string]interface{}) (map[string]float32, error) {
	return map[string]float32{}, movementsensor.ErrMethodUnimplementedAccuracy
}

// Close stops replay movement sensor, closes the channels and its connections to the cloud.
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
// connection.
func (replay *replayMovementSensor) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	replay.mu.Lock()
	defer replay.mu.Unlock()
	if replay.closed {
		return errors.New("session closed")
	}

	replayCamConfig, err := resource.NativeConfig[*Config](conf)
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
			return errors.Wrap(err, "failure to connect to the cloud")
		}
	}

	if replayCamConfig.BatchSize == nil {
		replay.limit = 1
	} else {
		replay.limit = *replayCamConfig.BatchSize
	}
	replay.cache = defaultCacheMap

	replay.properties = movementsensor.Properties{
		LinearVelocitySupported:     replayCamConfig.Properties["linear_velocity_supported"],
		AngularVelocitySupported:    replayCamConfig.Properties["angular_velocity_supported"],
		OrientationSupported:        replayCamConfig.Properties["orientation_supported"],
		PositionSupported:           replayCamConfig.Properties["position_supported"],
		CompassHeadingSupported:     replayCamConfig.Properties["compass_heading_supported"],
		LinearAccelerationSupported: replayCamConfig.Properties["linear_acceleration_supported"],
	}

	replay.filter = &datapb.Filter{
		ComponentName: replayCamConfig.Source,
		RobotId:       replayCamConfig.RobotID,
		Interval:      &datapb.CaptureInterval{},
	}
	replay.lastData = ""

	if replayCamConfig.Interval.Start != "" {
		startTime, err := time.Parse(timeFormat, replayCamConfig.Interval.Start)
		if err != nil {
			replay.closeCloudConnection(ctx)
			return errors.New("invalid time format for start time, missed during config validation")
		}
		replay.filter.Interval.Start = timestamppb.New(startTime)
	}

	if replayCamConfig.Interval.End != "" {
		endTime, err := time.Parse(timeFormat, replayCamConfig.Interval.End)
		if err != nil {
			replay.closeCloudConnection(ctx)
			return errors.New("invalid time format for end time, missed during config validation")
		}
		replay.filter.Interval.End = timestamppb.New(endTime)
	}

	return nil
}

// TODO UNIMPLEMENTED
func (replay *replayMovementSensor) downloadMoreData(ctx context.Context, method string) error {
	return nil
}

// getDataFromCache will return the next valid cache entry and advance the cache as needed. If there is not more data in the cache, a new batch of data will be downloaded.
func (replay *replayMovementSensor) getDataFromCache(ctx context.Context, method string, cache []*cacheEntry) (cacheEntry, error) {
	if len(replay.cache) == 0 {
		if err := replay.downloadMoreData(ctx, method); err != nil {
			return cacheEntry{}, err
		}
	}

	return *cache[0], nil
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
