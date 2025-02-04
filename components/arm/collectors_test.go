package arm_test

import (
	"context"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/golang/geo/r3"
	datasyncpb "go.viam.com/api/app/datasync/v1"
	pb "go.viam.com/api/component/arm/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	tu "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const (
	componentName   = "arm"
	captureInterval = time.Millisecond
)

var floatList = &pb.JointPositions{Values: []float64{1.0, 2.0, 3.0}}

func TestCollectors(t *testing.T) {
	tests := []struct {
		name      string
		collector data.CollectorConstructor
		expected  []*datasyncpb.SensorData
	}{
		{
			name:      "End position collector should write a pose",
			collector: arm.NewEndPositionCollector,
			expected: []*datasyncpb.SensorData{{
				Metadata: &datasyncpb.SensorMetadata{},
				Data: &datasyncpb.SensorData_Struct{Struct: tu.ToStructPBStruct(t, map[string]any{
					"pose": map[string]any{
						"o_x":   0,
						"o_y":   0,
						"o_z":   1,
						"theta": 0,
						"x":     1,
						"y":     2,
						"z":     3,
					},
				})},
			}},
		},
		{
			name:      "Joint positions collector should write a list of positions",
			collector: arm.NewJointPositionsCollector,
			expected: []*datasyncpb.SensorData{{
				Metadata: &datasyncpb.SensorMetadata{},
				Data: &datasyncpb.SensorData_Struct{Struct: tu.ToStructPBStruct(t, map[string]any{
					"positions": map[string]any{
						"values": []any{1.0, 2.0, 3.0},
					},
				})},
			}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			buf := tu.NewMockBuffer(t)
			params := data.CollectorParams{
				DataType:      data.CaptureTypeTabular,
				ComponentName: componentName,
				Interval:      captureInterval,
				Logger:        logging.NewTestLogger(t),
				Clock:         clock.New(),
				Target:        buf,
			}

			arm := newArm()
			col, err := tc.collector(arm, params)
			test.That(t, err, test.ShouldBeNil)

			defer col.Close()
			col.Collect()

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			tu.CheckMockBufferWrites(t, ctx, start, buf.Writes, tc.expected)
			buf.Close()
		})
	}
}

func newArm() arm.Arm {
	a := &inject.Arm{}
	a.EndPositionFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
		return spatialmath.NewPoseFromPoint(r3.Vector{X: 1, Y: 2, Z: 3}), nil
	}
	a.JointPositionsFunc = func(ctx context.Context, extra map[string]interface{}) ([]referenceframe.Input, error) {
		return referenceframe.FloatsToInputs(referenceframe.JointPositionsToRadians(floatList)), nil
	}
	a.ModelFrameFunc = func() referenceframe.Model {
		return nil
	}
	return a
}
