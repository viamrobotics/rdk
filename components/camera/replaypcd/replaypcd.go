// Package replaypcd implements a replay camera that can return point cloud data.
package replaypcd

import (
	"bytes"
	"compress/gzip"
	"context"
	"time"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/pkg/errors"
	datapb "go.viam.com/api/app/data/v1"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/internal/cloud"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage/transform"
)

// Model is the model of a replay camera.
var (
	model                 = resource.DefaultModelFamily.WithModel("replay_pcd")
	errEndOfDataset       = errors.New("Reached end of dataset")
	timeFormat            = time.RFC3339
	grpcConnectionTimeout = 10 * time.Second
)

func init() {
	resource.RegisterComponent(camera.API, model, resource.Registration[camera.Camera, *Config]{
		Constructor: newPCDCamera,
	})
}

// Config describes how to configure the replay camera component.
type Config struct {
	Source   string       `json:"source,omitempty"`
	RobotID  string       `json:"robot_id,omitempty"`
	Interval TimeInterval `json:"time_interval,omitempty"`
}

// TimeInterval holds the start and end time used to filter data.
type TimeInterval struct {
	Start string `json:"start,omitempty"`
	End   string `json:"end,omitempty"`
}

// Validate checks that the config attributes are valid for a replay camera.
func (cfg *Config) Validate(path string) ([]string, error) {
	if cfg.Source == "" {
		return nil, goutils.NewConfigValidationFieldRequiredError(path, "source")
	}

	if cfg.Interval.Start != "" && cfg.Interval.Start > time.Now().UTC().Format(timeFormat) {
		return nil, errors.New("invalid config, start time must be in the past")
	}

	if cfg.Interval.End != "" && cfg.Interval.End > time.Now().UTC().Format(timeFormat) {
		return nil, errors.New("invalid config, end time must be in the past")
	}

	if cfg.Interval.Start != "" && cfg.Interval.End != "" && cfg.Interval.Start > cfg.Interval.End {
		return nil, errors.New("invalid config, end time must be after start time")
	}

	return []string{cloud.InternalServiceName.String()}, nil
}

// pcdCamera is a camera model that plays back pre-captured point cloud data.
type pcdCamera struct {
	resource.Named
	logger golog.Logger

	cloudConnSvc cloud.ConnectionService
	cloudConn    rpc.ClientConn
	dataClient   datapb.DataServiceClient

	source  string
	robotID string

	lastData string
	limit    uint64
	filter   *datapb.Filter
}

// newPCDCamera creates a new replay camera based on the inputted config and dependencies.
func newPCDCamera(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger golog.Logger) (camera.Camera, error) {
	cam := &pcdCamera{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
		limit:  1,
	}

	if err := cam.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}

	return cam, nil
}

// NextPointCloud returns a point cloud retrieved the next from cloud storage based on the applied filter.
func (replay *pcdCamera) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	resp, err := replay.dataClient.BinaryDataByFilter(ctx, &datapb.BinaryDataByFilterRequest{
		DataRequest: &datapb.DataRequest{
			Filter:    replay.filter,
			Limit:     replay.limit,
			Last:      replay.lastData,
			SortOrder: datapb.Order_ORDER_ASCENDING,
		},
		CountOnly:     false,
		IncludeBinary: true,
	})
	if err != nil {
		return nil, err
	}

	// If no data is returned, return an error.
	if len(resp.GetData()) == 0 {
		return nil, errEndOfDataset
	}

	replay.lastData = resp.GetLast()
	data := resp.Data[0].GetBinary()

	r, err := gzip.NewReader(bytes.NewBuffer(data))
	if err != nil {
		return nil, errors.Errorf("Failed to initialize gzip reader: %v", err)
	}

	defer func() {
		if err = r.Close(); err != nil {
			replay.logger.Warnw("Failed to close gzip reader", "warn", err)
		}
	}()

	pc, err := pointcloud.ReadPCD(r)
	if err != nil {
		return nil, err
	}
	return pc, nil
}

// Properties is a part of the camera interface but is not implemented for replay.
func (replay *pcdCamera) Properties(ctx context.Context) (camera.Properties, error) {
	var props camera.Properties
	return props, errors.New("Properties is unimplemented")
}

// Projector is a part of the camera interface but is not implemented for replay.
func (replay *pcdCamera) Projector(ctx context.Context) (transform.Projector, error) {
	var proj transform.Projector
	return proj, errors.New("Projector is unimplemented")
}

// Stream is a part of the camera interface but is not implemented for replay.
func (replay *pcdCamera) Stream(ctx context.Context, errHandlers ...gostream.ErrorHandler) (gostream.VideoStream, error) {
	var stream gostream.VideoStream
	return stream, errors.New("Stream is unimplemented")
}

// Close stops replay camera and closes its connections to the cloud.
func (replay *pcdCamera) Close(ctx context.Context) error {
	replay.closeCloudConnection(ctx)
	return nil
}

// Reconfigure will bring up a replay camera using the new config but is not implemented for replay.
func (replay *pcdCamera) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
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
			defer replay.closeCloudConnection(ctx)
			return errors.Wrap(err, "failure to connect to the cloud")
		}
	}

	replay.source = replayCamConfig.Source
	replay.robotID = replayCamConfig.RobotID
	replay.filter = &datapb.Filter{
		ComponentName: replay.source,
		RobotId:       replay.robotID,
		MimeType:      []string{"pointcloud/pcd"},
		Interval:      &datapb.CaptureInterval{},
	}

	if replayCamConfig.Interval.Start != "" {
		startTime, err := time.Parse(timeFormat, replayCamConfig.Interval.Start)
		if err != nil {
			defer replay.closeCloudConnection(ctx)
			return errors.New("invalid time format, use RFC3339")
		}
		replay.filter.Interval.Start = timestamppb.New(startTime)
	}

	if replayCamConfig.Interval.End != "" {
		endTime, err := time.Parse(timeFormat, replayCamConfig.Interval.End)
		if err != nil {
			defer replay.closeCloudConnection(ctx)
			return errors.New("invalid time format, use RFC3339")
		}
		replay.filter.Interval.End = timestamppb.New(endTime)
	}
	return nil
}

// closeCloud closes all parts of the cloud connection used by the replay camera.
func (replay *pcdCamera) closeCloudConnection(ctx context.Context) {
	if replay.cloudConn != nil {
		goutils.UncheckedError(replay.cloudConn.Close())
	}

	if replay.cloudConnSvc != nil {
		goutils.UncheckedError(replay.cloudConnSvc.Close(ctx))
	}
}

// initCloudConnection creates a rpc client connection and data service.
func (replay *pcdCamera) initCloudConnection(ctx context.Context) error {
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
