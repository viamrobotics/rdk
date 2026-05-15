package data

import (
	"context"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/golang/geo/r3"
	datasyncpb "go.viam.com/api/app/datasync/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	tu "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

// GetWorldPoseTestConfig holds configuration for GetWorldPose collector tests.
type GetWorldPoseTestConfig struct {
	ComponentName   string
	CaptureInterval time.Duration
	Collector       data.CollectorConstructor
	ResourceFactory func() interface{}
}

// TestGetWorldPoseCollector runs tests for GetWorldPose collectors.
func TestGetWorldPoseCollector(t *testing.T, config GetWorldPoseTestConfig) {
	t.Helper()

	t.Run("GetWorldPose collector should write the component's world-space pose", func(t *testing.T) {
		start := time.Now()
		buf := tu.NewMockBuffer(t)

		fs := inject.NewFrameSystemService("test-fs")
		fs.GetPoseFunc = func(
			ctx context.Context,
			componentName, destinationFrame string,
			supplementalTransforms []*referenceframe.LinkInFrame,
			extra map[string]interface{},
		) (*referenceframe.PoseInFrame, error) {
			return referenceframe.NewPoseInFrame(
				referenceframe.World,
				spatialmath.NewPoseFromPoint(r3.Vector{X: 1, Y: 2, Z: 3}),
			), nil
		}

		params := data.CollectorParams{
			DataType:      data.CaptureTypeTabular,
			ComponentName: config.ComponentName,
			Interval:      config.CaptureInterval,
			Logger:        logging.NewTestLogger(t),
			Clock:         clock.New(),
			Target:        buf,
			FrameSystem:   fs,
		}

		col, err := config.Collector(config.ResourceFactory(), params)
		test.That(t, err, test.ShouldBeNil)

		defer col.Close()
		col.Collect()

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		tu.CheckMockBufferWrites(t, ctx, start, buf.Writes, []*datasyncpb.SensorData{{
			Metadata: &datasyncpb.SensorMetadata{},
			Data: &datasyncpb.SensorData_Struct{Struct: tu.ToStructPBStruct(t, map[string]any{
				"pose": map[string]any{
					"x": 1.0, "y": 2.0, "z": 3.0,
					"o_x": 0.0, "o_y": 0.0, "o_z": 1.0, "theta": 0.0,
				},
			})},
		}})
		buf.Close()
	})

	t.Run("GetWorldPose collector should error when frame system is nil", func(t *testing.T) {
		params := data.CollectorParams{
			DataType:      data.CaptureTypeTabular,
			ComponentName: config.ComponentName,
			Interval:      config.CaptureInterval,
			Logger:        logging.NewTestLogger(t),
			Clock:         clock.New(),
			Target:        tu.NewMockBuffer(t),
		}

		_, err := config.Collector(config.ResourceFactory(), params)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "frame system is required")
	})
}
