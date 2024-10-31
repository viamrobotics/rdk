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
	"google.golang.org/protobuf/types/known/structpb"

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
		expected  *datasyncpb.SensorData
	}{
		{
			name:      "Movement sensor linear velocity collector should write a velocity response",
			collector: movementsensor.NewLinearVelocityCollector,
			expected: &datasyncpb.SensorData{
				Metadata: &datasyncpb.SensorMetadata{},
				Data: &datasyncpb.SensorData_Struct{Struct: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"linear_velocity": structpb.NewStructValue(&structpb.Struct{
							Fields: map[string]*structpb.Value{
								"x": structpb.NewNumberValue(1.0),
								"y": structpb.NewNumberValue(2.0),
								"z": structpb.NewNumberValue(3.0),
							},
						}),
					},
				}},
			},
		},
		{
			name:      "Movement sensor position collector should write a position response",
			collector: movementsensor.NewPositionCollector,
			expected: &datasyncpb.SensorData{
				Metadata: &datasyncpb.SensorMetadata{},
				Data: &datasyncpb.SensorData_Struct{Struct: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"coordinate": structpb.NewStructValue(&structpb.Struct{
							Fields: map[string]*structpb.Value{
								"latitude":  structpb.NewNumberValue(1.0),
								"longitude": structpb.NewNumberValue(2.0),
							},
						}),
						"altitude_m": structpb.NewNumberValue(3.0),
					},
				}},
			},
		},
		{
			name:      "Movement sensor angular velocity collector should write a velocity response",
			collector: movementsensor.NewAngularVelocityCollector,
			expected: &datasyncpb.SensorData{
				Metadata: &datasyncpb.SensorMetadata{},
				Data: &datasyncpb.SensorData_Struct{Struct: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"angular_velocity": structpb.NewStructValue(&structpb.Struct{
							Fields: map[string]*structpb.Value{
								"x": structpb.NewNumberValue(1.0),
								"y": structpb.NewNumberValue(2.0),
								"z": structpb.NewNumberValue(3.0),
							},
						}),
					},
				}},
			},
		},
		{
			name:      "Movement sensor compass heading collector should write a heading response",
			collector: movementsensor.NewCompassHeadingCollector,
			expected: &datasyncpb.SensorData{
				Metadata: &datasyncpb.SensorMetadata{},
				Data: &datasyncpb.SensorData_Struct{Struct: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"value": structpb.NewNumberValue(1.0),
					},
				}},
			},
		},
		{
			name:      "Movement sensor linear acceleration collector should write an acceleration response",
			collector: movementsensor.NewLinearAccelerationCollector,
			expected: &datasyncpb.SensorData{
				Metadata: &datasyncpb.SensorMetadata{},
				Data: &datasyncpb.SensorData_Struct{Struct: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"linear_acceleration": structpb.NewStructValue(&structpb.Struct{
							Fields: map[string]*structpb.Value{
								"x": structpb.NewNumberValue(1.0),
								"y": structpb.NewNumberValue(2.0),
								"z": structpb.NewNumberValue(3.0),
							},
						}),
					},
				}},
			},
		},
		{
			name:      "Movement sensor orientation collector should write an orientation response",
			collector: movementsensor.NewOrientationCollector,
			expected: &datasyncpb.SensorData{
				Metadata: &datasyncpb.SensorMetadata{},
				Data: &datasyncpb.SensorData_Struct{Struct: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"orientation": structpb.NewStructValue(&structpb.Struct{
							Fields: map[string]*structpb.Value{
								"o_x":   structpb.NewNumberValue(0),
								"o_y":   structpb.NewNumberValue(0),
								"o_z":   structpb.NewNumberValue(1),
								"theta": structpb.NewNumberValue(0),
							},
						}),
					},
				}},
			},
		},
		{
			name:      "Movement sensor readings collector should write a readings response",
			collector: movementsensor.NewReadingsCollector,
			expected: &datasyncpb.SensorData{
				Metadata: &datasyncpb.SensorMetadata{},
				Data: &datasyncpb.SensorData_Struct{Struct: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"readings": structpb.NewStructValue(&structpb.Struct{
							Fields: map[string]*structpb.Value{
								"reading1": structpb.NewBoolValue(false),
								"reading2": structpb.NewStringValue("test"),
							},
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

			movSens := newMovementSensor()
			col, err := tc.collector(movSens, params)
			test.That(t, err, test.ShouldBeNil)

			defer col.Close()
			col.Collect()

			tu.CheckMockBufferWrites(t, ctx, start, buf.TabularWrites, tc.expected)
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
