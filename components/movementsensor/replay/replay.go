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
	// model is the model of a replay camera.
	model = resource.DefaultModelFamily.WithModel("replay_pcd")

	// ErrEndOfDataset represents that the replay sensor has reached the end of the dataset.
	ErrEndOfDataset = errors.New("reached end of dataset")
)

func init() {
	resource.RegisterComponent(movementsensor.API, model, resource.Registration[movementsensor.MovementSensor, *Config]{
		Constructor: newReplayMovementSensor,
	})
}

// Validate checks that the config attributes are valid for a replay camera.
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

// cacheEntry stores data that was downloaded from a previous operation but has not yet been passed
// to the caller.
type cacheEntry struct {
	id            *datapb.BinaryID
	data          map[string]interface{}
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

	cache []*cacheEntry

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

func (replay *replayMovementSensor) Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
	return nil, 0, nil
}

func (replay *replayMovementSensor) LinearVelocity(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	return r3.Vector{}, nil
}

func (replay *replayMovementSensor) AngularVelocity(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) {
	return spatialmath.AngularVelocity{}, nil
}

func (replay *replayMovementSensor) LinearAcceleration(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	return r3.Vector{}, nil
}

func (replay *replayMovementSensor) CompassHeading(ctx context.Context, extra map[string]interface{}) (float64, error) {
	return 0, nil
}

func (replay *replayMovementSensor) Orientation(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
	return nil, nil
}

func (replay *replayMovementSensor) Properties(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
	return &movementsensor.Properties{}, nil
}

func (replay *replayMovementSensor) Accuracy(ctx context.Context, extra map[string]interface{}) (map[string]float32, error) {
	return nil, nil
}

func (replay *replayMovementSensor) Close(ctx context.Context) error {
	return nil
}

func (replay *replayMovementSensor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	return nil, nil
}

func (replay *replayMovementSensor) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	return nil
}
