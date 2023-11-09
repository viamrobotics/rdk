package movementsensor_test

import (
	"context"
	"testing"
	"time"

	clk "github.com/benbjohnson/clock"
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	v1 "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/movementsensor/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/spatialmath"
	tu "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const (
	componentName   = "movementsensor"
	captureInterval = time.Second
)

var vec = r3.Vector{
	X: 1.0,
	Y: 2.0,
	Z: 3.0,
}

func TestMovementSensorCollectors(t *testing.T) {
	tests := []struct {
		name      string
		collector data.CollectorConstructor
		expected  map[string]any
	}{
		{
			name:      "Movement sensor linear velocity collector should write a velocity response",
			collector: movementsensor.NewLinearVelocityCollector,
			expected: tu.ToProtoMapIgnoreOmitEmpty(pb.GetLinearVelocityResponse{
				LinearVelocity: r3VectorToV1Vector(vec),
			}),
		},
		{
			name:      "Movement sensor position collector should write a position response",
			collector: movementsensor.NewPositionCollector,
			expected: tu.ToProtoMapIgnoreOmitEmpty(pb.GetPositionResponse{
				Coordinate: &v1.GeoPoint{
					Latitude:  1.0,
					Longitude: 2.0,
				},
				AltitudeM: 3.0,
			}),
		},
		{
			name:      "Movement sensor angular velocity collector should write a velocity response",
			collector: movementsensor.NewAngularVelocityCollector,
			expected: tu.ToProtoMapIgnoreOmitEmpty(pb.GetAngularVelocityResponse{
				AngularVelocity: r3VectorToV1Vector(vec),
			}),
		},
		{
			name:      "Movement sensor compass heading collector should write a heading response",
			collector: movementsensor.NewCompassHeadingCollector,
			expected: tu.ToProtoMapIgnoreOmitEmpty(pb.GetCompassHeadingResponse{
				Value: 1.0,
			}),
		},
		{
			name:      "Movement sensor linear acceleration collector should write an acceleration response",
			collector: movementsensor.NewLinearAccelerationCollector,
			expected: tu.ToProtoMapIgnoreOmitEmpty(pb.GetLinearAccelerationResponse{
				LinearAcceleration: r3VectorToV1Vector(vec),
			}),
		},
		{
			name:      "Movement sensor orientation collector should write an orientation response",
			collector: movementsensor.NewOrientationCollector,
			expected: tu.ToProtoMapIgnoreOmitEmpty(pb.GetOrientationResponse{
				Orientation: getExpectedOrientation(),
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
				Logger:        logging.NewTestLogger(t),
				Clock:         mockClock,
				Target:        &buf,
			}

			movSens := newMovementSensor()
			col, err := tc.collector(movSens, params)
			test.That(t, err, test.ShouldBeNil)

			defer col.Close()
			col.Collect()
			mockClock.Add(captureInterval)

			test.That(t, buf.Length(), test.ShouldEqual, 1)
			test.That(t, buf.Writes[0].GetStruct().AsMap(), test.ShouldResemble, tc.expected)
		})
	}
}

func newMovementSensor() movementsensor.MovementSensor {
	m := &inject.MovementSensor{}
	m.LinearVelocityFunc = func(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
		return vec, nil
	}
	m.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
		return geo.NewPoint(1.0, 2.0), 3.0, nil
	}
	m.AngularVelocityFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) {
		return spatialmath.AngularVelocity{
			X: 1.0,
			Y: 2.0,
			Z: 3.0,
		}, nil
	}
	m.CompassHeadingFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
		return 1.0, nil
	}
	m.LinearAccelerationFunc = func(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
		return vec, nil
	}
	m.OrientationFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
		return spatialmath.NewZeroOrientation(), nil
	}
	return m
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
