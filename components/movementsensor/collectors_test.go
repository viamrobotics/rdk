package movementsensor

import (
	"context"
	"testing"
	"time"

	clk "github.com/benbjohnson/clock"
	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	v1 "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/movementsensor/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	tu "go.viam.com/rdk/testutils"
)

type collectorFunc func(resource interface{}, params data.CollectorParams) (data.Collector, error)

const componentName = "movementsensor"

var vec = r3.Vector{
	X: 1.0,
	Y: 2.0,
	Z: 3.0,
}

func TestMovementSensorCollectors(t *testing.T) {
	tests := []struct {
		name      string
		params    data.CollectorParams
		collector collectorFunc
		expected  map[string]any
	}{
		{
			name: "Movement sensor linear velocity collector should write a velocity response",
			params: data.CollectorParams{
				ComponentName: componentName,
				Interval:      time.Second,
				Logger:        golog.NewTestLogger(t),
			},
			collector: newLinearVelocityCollector,
			expected: toProtoMap(pb.GetLinearVelocityResponse{
				LinearVelocity: r3VectorToV1Vector(vec),
			}),
		},
		{
			name: "Movement sensor position collector should write a position response",
			params: data.CollectorParams{
				ComponentName: componentName,
				Interval:      time.Second,
				Logger:        golog.NewTestLogger(t),
			},
			collector: newPositionCollector,
			expected: toProtoMap(pb.GetPositionResponse{
				Coordinate: &v1.GeoPoint{
					Latitude:  1.0,
					Longitude: 2.0,
				},
				AltitudeM: 3.0,
			}),
		},
		{
			name: "Movement sensor angular velocity collector should write a velocity response",
			params: data.CollectorParams{
				ComponentName: componentName,
				Interval:      time.Second,
				Logger:        golog.NewTestLogger(t),
			},
			collector: newAngularVelocityCollector,
			expected: toProtoMap(pb.GetAngularVelocityResponse{
				AngularVelocity: r3VectorToV1Vector(vec),
			}),
		},
		{
			name: "Movement sensor compass heading collector should write a heading response",
			params: data.CollectorParams{
				ComponentName: componentName,
				Interval:      time.Second,
				Logger:        golog.NewTestLogger(t),
			},
			collector: newCompassHeadingCollector,
			expected: toProtoMap(pb.GetCompassHeadingResponse{
				Value: 1.0,
			}),
		},
		{
			name: "Movement sensor linear acceleration collector should write an acceleration response",
			params: data.CollectorParams{
				ComponentName: componentName,
				Interval:      time.Second,
				Logger:        golog.NewTestLogger(t),
			},
			collector: newLinearAccelerationCollector,
			expected: toProtoMap(pb.GetLinearAccelerationResponse{
				LinearAcceleration: r3VectorToV1Vector(vec),
			}),
		},
		{
			name: "Movement sensor orientation collector should write an orientation response",
			params: data.CollectorParams{
				ComponentName: componentName,
				Interval:      time.Second,
				Logger:        golog.NewTestLogger(t),
			},
			collector: newOrientationCollector,
			expected: toProtoMap(pb.GetOrientationResponse{
				Orientation: getExpectedOrientation(),
			}),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockClock := clk.NewMock()
			buf := tu.MockBuffer{}
			tc.params.Clock = mockClock
			tc.params.Target = &buf

			movSens := newMovementSensor(componentName)
			col, err := tc.collector(movSens, tc.params)
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

type fakeMovementSensor struct {
	MovementSensor
	name resource.Name
}

func newMovementSensor(name string) MovementSensor {
	return &fakeMovementSensor{name: resource.Name{Name: name}}
}

func (i *fakeMovementSensor) Name() resource.Name {
	return i.name
}

func (i *fakeMovementSensor) Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
	return geo.NewPoint(1.0, 2.0), 3.0, nil
}

func (i *fakeMovementSensor) LinearVelocity(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	return vec, nil
}

func (i *fakeMovementSensor) AngularVelocity(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) {
	return spatialmath.AngularVelocity{
		X: 1.0,
		Y: 2.0,
		Z: 3.0,
	}, nil
}

func (i *fakeMovementSensor) LinearAcceleration(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	return vec, nil
}

func (i *fakeMovementSensor) Orientation(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
	return spatialmath.NewZeroOrientation(), nil
}

func (i *fakeMovementSensor) CompassHeading(ctx context.Context, extra map[string]interface{}) (float64, error) {
	return 1.0, nil
}

func r3VectorToV1Vector(vec r3.Vector) *v1.Vector3 {
	return &v1.Vector3{
		X: vec.X,
		Y: vec.Y,
		Z: vec.Z,
	}
}

func getExpectedOrientation() *v1.Orientation {
	convertedAngles := spatialmath.NewZeroOrientation().AxisAngles()
	return &v1.Orientation{
		OX:    convertedAngles.RX,
		OY:    convertedAngles.RY,
		OZ:    convertedAngles.RZ,
		Theta: convertedAngles.Theta,
	}
}

func toProtoMap(data any) map[string]any {
	ret, err := protoutils.StructToStructPbIgnoreOmitEmpty(data)
	if err != nil {
		return nil
	}
	return ret.AsMap()
}
