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
	"google.golang.org/protobuf/types/known/structpb"

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
	l, err := structpb.NewList([]any{1.0, 2.0, 3.0})
	test.That(t, err, test.ShouldBeNil)

	tests := []struct {
		name      string
		collector data.CollectorConstructor
		expected  *datasyncpb.SensorData
	}{
		{
			name:      "End position collector should write a pose",
			collector: arm.NewEndPositionCollector,
			expected: &datasyncpb.SensorData{
				Metadata: &datasyncpb.SensorMetadata{},
				Data: &datasyncpb.SensorData_Struct{Struct: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"pose": structpb.NewStructValue(&structpb.Struct{
							Fields: map[string]*structpb.Value{
								"o_x":   structpb.NewNumberValue(0),
								"o_y":   structpb.NewNumberValue(0),
								"o_z":   structpb.NewNumberValue(1),
								"theta": structpb.NewNumberValue(0),
								"x":     structpb.NewNumberValue(1),
								"y":     structpb.NewNumberValue(2),
								"z":     structpb.NewNumberValue(3),
							},
						}),
					},
				}},
			},
		},
		{
			name:      "Joint positions collector should write a list of positions",
			collector: arm.NewJointPositionsCollector,
			expected: &datasyncpb.SensorData{
				Metadata: &datasyncpb.SensorMetadata{},
				Data: &datasyncpb.SensorData_Struct{Struct: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"positions": structpb.NewStructValue(&structpb.Struct{
							Fields: map[string]*structpb.Value{"values": structpb.NewListValue(l)},
						}),
					},
				}},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			buf := tu.NewMockBuffer(ctx)
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

			tu.CheckMockBufferWrites(t, ctx, start, buf.TabularWrites, tc.expected)
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
