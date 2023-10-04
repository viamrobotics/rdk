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
	"go.viam.com/utils/protoutils"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	tu "go.viam.com/rdk/testutils"
)

type collectorFunc func(resource interface{}, params data.CollectorParams) (data.Collector, error)

const componentName = "arm"

var floatList = []float64{1.0, 2.0, 3.0}

func TestCollectors(t *testing.T) {
	tests := []struct {
		name      string
		params    data.CollectorParams
		collector collectorFunc
		expected  map[string]any
	}{
		{
			name: "End position collector should write a pose",
			params: data.CollectorParams{
				ComponentName: componentName,
				Interval:      time.Second,
				Logger:        golog.NewTestLogger(t),
			},
			collector: newEndPositionCollector,
			expected: toProtoMap(pb.GetEndPositionResponse{
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
			name: "Joint positions collector should write a list of positions",
			params: data.CollectorParams{
				ComponentName: componentName,
				Interval:      time.Second,
				Logger:        golog.NewTestLogger(t),
			},
			collector: newJointPositionsCollector,
			expected: toProtoMap(pb.GetJointPositionsResponse{
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
			tc.params.Clock = mockClock
			tc.params.Target = &buf

			arm := newArm(componentName)
			col, err := tc.collector(arm, tc.params)
			test.That(t, err, test.ShouldBeNil)

			defer col.Close()
			col.Collect()
			mockClock.Add(1 * time.Second)

			test.That(t, err, test.ShouldBeNil)
			test.That(t, len(buf.Writes), test.ShouldEqual, 1)
			test.That(t, buf.Writes[0].GetStruct().AsMap(), test.ShouldResemble, tc.expected)
		})
	}
}

type fakeArm struct {
	Arm
	name resource.Name
}

func newArm(name string) Arm {
	return &fakeArm{name: resource.Name{Name: name}}
}

func (a *fakeArm) EndPosition(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
	return spatialmath.NewPoseFromPoint(r3.Vector{X: 1, Y: 2, Z: 3}), nil
}

func (a *fakeArm) JointPositions(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error) {
	return &pb.JointPositions{
		Values: floatList,
	}, nil
}

func toProtoMap(data any) map[string]any {
	ret, err := protoutils.StructToStructPbIgnoreOmitEmpty(data)
	if err != nil {
		return nil
	}
	return ret.AsMap()
}
