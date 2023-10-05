package arm

import (
	"context"
	"testing"
	"time"

	clk "github.com/benbjohnson/clock"
	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	v1 "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/arm/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/spatialmath"
	tu "go.viam.com/rdk/testutils"
)

const (
	componentName   = "arm"
	captureInterval = time.Second
)

var floatList = []float64{1.0, 2.0, 3.0}

func TestCollectors(t *testing.T) {
	tests := []struct {
		name      string
		collector data.CollectorConstructor
		expected  map[string]any
	}{
		{
			name:      "End position collector should write a pose",
			collector: newEndPositionCollector,
			expected: tu.ToProtoMapIgnoreOmitEmpty(pb.GetEndPositionResponse{
				Pose: &v1.Pose{
					OX:    0,
					OY:    0,
					OZ:    1,
					Theta: 0,
					X:     1,
					Y:     2,
					Z:     3,
				},
			}),
		},
		{
			name:      "Joint positions collector should write a list of positions",
			collector: newJointPositionsCollector,
			expected: tu.ToProtoMapIgnoreOmitEmpty(pb.GetJointPositionsResponse{
				Positions: &pb.JointPositions{
					Values: floatList,
				},
			}),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockClock := clk.NewMock()
			buf := tu.MockBuffer{}
			params := data.CollectorParams{
				ComponentName: componentName,
				Interval:      captureInterval,
				Logger:        golog.NewTestLogger(t),
				Clock:         mockClock,
				Target:        &buf,
			}

			arm := newArm()
			col, err := tc.collector(arm, params)
			test.That(t, err, test.ShouldBeNil)

			defer col.Close()
			col.Collect()
			mockClock.Add(captureInterval)

			test.That(t, buf.Length(), test.ShouldEqual, 1)
			test.That(t, buf.Writes[0].GetStruct().AsMap(), test.ShouldResemble, tc.expected)
		})
	}
}

type fakeArm struct {
	Arm
}

func newArm() Arm {
	return &fakeArm{}
}

func (a *fakeArm) EndPosition(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
	return spatialmath.NewPoseFromPoint(r3.Vector{X: 1, Y: 2, Z: 3}), nil
}

func (a *fakeArm) JointPositions(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error) {
	return &pb.JointPositions{
		Values: floatList,
	}, nil
}
