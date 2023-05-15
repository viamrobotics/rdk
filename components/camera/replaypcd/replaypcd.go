// Package replaypcd implements a replay camera that can return point cloud data.
package replaypcd

import (
	"context"
	"time"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage/transform"
)

// Model is the model of a replay camera.
var model = resource.DefaultModelFamily.WithModel("replay_pcd")

func init() {
	resource.RegisterComponent(camera.API, model, resource.Registration[camera.Camera, *Config]{
		Constructor: newPCDCamera,
	})
}

// Config describes how to configure the replay camera component.
type Config struct {
	Source   resource.Name `json:"source,omitempty"`
	RobotID  string        `json:"robot_id,omitempty"`
	Interval TimeInterval  `json:"time_interval,omitempty"`
}

// TimeInterval holds the start and end time used to filter data.
type TimeInterval struct {
	Start time.Time `json:"start,omitempty"`
	End   time.Time `json:"end,omitempty"`
}

// Validate checks that the config attributes are valid for a replay camera.
func (c *Config) Validate(path string) ([]string, error) {
	return nil, nil
}

// pcdCamera is a camera model that plays back pre-captured point cloud data.
type pcdCamera struct {
	resource.Named
	logger golog.Logger
}

// newPCDCamera creates a new replay camera based on the inputted config and dependencies.
func newPCDCamera(ctx context.Context, deps resource.Dependencies, cfg resource.Config, logger golog.Logger) (camera.Camera, error) {
	cam := &pcdCamera{
		logger: logger,
	}
	// TODO: Add start protocol for replay camera https://viam.atlassian.net/browse/RSDK-2893
	return cam, nil
}

// NextPointCloud is part of the camera interface but is not implemented for replay.
func (replay *pcdCamera) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	var pc pointcloud.PointCloud

	// If the caller is communicating with the replay camera over gRPC, set the timestamps on
	// the gRPC header.
	theTime := time.Now().Format(time.RFC3339)
	if stream := grpc.ServerTransportStreamFromContext(ctx); stream != nil {
		if err := grpc.SendHeader(ctx, metadata.MD{
			camera.TimeRequestedMetadataKey: []string{theTime},
			camera.TimeReceivedMetadataKey:  []string{theTime},
		}); err != nil {
			return nil, err
		}
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

// Close stops replay camera but is not implemented for replay.
func (replay *pcdCamera) Close(ctx context.Context) error {
	return errors.New("Close is unimplemented")
}

// Reconfigure will bring up a replay camera using the new config but is not implemented for replay.
func (replay *pcdCamera) Reconfigure(ctx context.Context, _ resource.Dependencies, cfg resource.Config) error {
	return errors.New("Reconfigure is unimplemented")
}
