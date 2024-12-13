package movementsensor_test

import (
	"context"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	datasyncpb "go.viam.com/api/app/datasync/v1"
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
	captureInterval = time.Millisecond
)

var vec = r3.Vector{
	X: 1.0,
	Y: 2.0,
	Z: 3.0,
}

var readingMap = map[string]any{"reading1": false, "reading2": "test"}

func TestCollectors(t *testing.T) {
	tests := []struct {
		name      string
		collector data.CollectorConstructor
		expected  []*datasyncpb.SensorData
	}{
		{
			name:      "Movement sensor linear velocity collector should write a velocity response",
			collector: movementsensor.NewLinearVelocityCollector,
			expected: []*datasyncpb.SensorData{{
				Metadata: &datasyncpb.SensorMetadata{},
				Data: &datasyncpb.SensorData_Struct{Struct: tu.ToStructPBStruct(t, map[string]any{
					"linear_velocity": map[string]any{
						"x": 1.0,
						"y": 2.0,
						"z": 3.0,
					},
				})},
			}},
		},
		{
			name:      "Movement sensor position collector should write a position response",
			collector: movementsensor.NewPositionCollector,
			expected: []*datasyncpb.SensorData{
				{
					Metadata: &datasyncpb.SensorMetadata{},
					Data: &datasyncpb.SensorData_Struct{Struct: tu.ToStructPBStruct(t, map[string]any{
						"coordinate": map[string]any{
							"latitude":  1.0,
							"longitude": 2.0,
						},
						"altitude_m": 3.0,
					})},
				},
			},
		},
		{
			name:      "Movement sensor angular velocity collector should write a velocity response",
			collector: movementsensor.NewAngularVelocityCollector,
			expected: []*datasyncpb.SensorData{{
				Metadata: &datasyncpb.SensorMetadata{},
				Data: &datasyncpb.SensorData_Struct{Struct: tu.ToStructPBStruct(t, map[string]any{
					"angular_velocity": map[string]any{
						"x": 1.0,
						"y": 2.0,
						"z": 3.0,
					},
				})},
			}},
		},
		{
			name:      "Movement sensor compass heading collector should write a heading response",
			collector: movementsensor.NewCompassHeadingCollector,
			expected: []*datasyncpb.SensorData{
				{
					Metadata: &datasyncpb.SensorMetadata{},
					Data: &datasyncpb.SensorData_Struct{Struct: tu.ToStructPBStruct(t, map[string]any{
						"value": 1.0,
					})},
				},
			},
		},
		{
			name:      "Movement sensor linear acceleration collector should write an acceleration response",
			collector: movementsensor.NewLinearAccelerationCollector,
			expected: []*datasyncpb.SensorData{
				{
					Metadata: &datasyncpb.SensorMetadata{},
					Data: &datasyncpb.SensorData_Struct{Struct: tu.ToStructPBStruct(t, map[string]any{
						"linear_acceleration": map[string]any{
							"x": 1.0,
							"y": 2.0,
							"z": 3.0,
						},
					})},
				},
			},
		},
		{
			name:      "Movement sensor orientation collector should write an orientation response",
			collector: movementsensor.NewOrientationCollector,
			expected: []*datasyncpb.SensorData{
				{
					Metadata: &datasyncpb.SensorMetadata{},
					Data: &datasyncpb.SensorData_Struct{Struct: tu.ToStructPBStruct(t, map[string]any{
						"orientation": map[string]any{
							"o_x":   0,
							"o_y":   0,
							"o_z":   1,
							"theta": 0,
						},
					})},
				},
			},
		},
		{
			name:      "Movement sensor readings collector should write a readings response",
			collector: movementsensor.NewReadingsCollector,
			expected: []*datasyncpb.SensorData{
				{
					Metadata: &datasyncpb.SensorMetadata{},
					Data: &datasyncpb.SensorData_Struct{Struct: tu.ToStructPBStruct(t, map[string]any{
						"readings": map[string]any{
							"reading1": false,
							"reading2": "test",
						},
					})},
				},
			},
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

			movSens := newMovementSensor()
			col, err := tc.collector(movSens, params)
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
	m.ReadingsFunc = func(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
		return readingMap, nil
	}
	return m
}
