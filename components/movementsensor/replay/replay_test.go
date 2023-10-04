package replay

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"
	"google.golang.org/grpc"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/internal/cloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/utils/contextutils"
)

var (
	validSource         = "source"
	validRobotID        = "robot_id"
	validOrganizationID = "organization_id"
	validLocationID     = "location_id"

	batchSizeZero        = uint64(0)
	batchSizeNonZero     = uint64(5)
	batchSize4           = uint64(4)
	batchSizeOutOfBounds = uint64(50000)

	positionPointData = []*geo.Point{
		geo.NewPoint(0, 0),
		geo.NewPoint(1, 0),
		geo.NewPoint(.5, 0),
		geo.NewPoint(0, .4),
		geo.NewPoint(1, .4),
	}

	positionAltitudeData = []float64{0, 1, 2, 3, 4}

	linearVelocityData = []r3.Vector{
		{X: 0, Y: 0, Z: 0},
		{X: 1, Y: 0, Z: 0},
		{X: 0, Y: 1, Z: 11},
		{X: 1, Y: 4, Z: 0},
		{X: 0, Y: 3, Z: 3},
		{X: 3, Y: 2, Z: 7},
		{X: 0, Y: 3, Z: 3},
		{X: 3, Y: 2, Z: 7},
		{X: 0, Y: 3, Z: 311},
	}

	angularVelocityData = []spatialmath.AngularVelocity{
		{X: 0, Y: 0, Z: 0},
		{X: 1, Y: 0, Z: 2},
		{X: 0, Y: 1, Z: 0},
		{X: 0, Y: 5, Z: 2},
		{X: 2, Y: 3, Z: 3},
		{X: 1, Y: 2, Z: 0},
		{X: 0, Y: 0, Z: 12},
	}

	linearAccelerationData = []r3.Vector{
		{X: 0, Y: 0, Z: 0},
		{X: 1, Y: 0, Z: 0},
		{X: 0, Y: 1, Z: 0},
		{X: 0, Y: 2, Z: 0},
		{X: 0, Y: 3, Z: 3},
	}

	compassHeadingData = []float64{0, 1, 2, 3, 4, 5, 6, 4, 3, 2, 1}

	orientationData = []*spatialmath.OrientationVector{
		{OX: 1, OY: 0, OZ: 1, Theta: 0},
		{OX: 2, OY: 1, OZ: 1, Theta: 0},
	}

	allMethodsMaxDataLength = map[method]int{
		position:           len(positionPointData),
		linearVelocity:     len(linearVelocityData),
		angularVelocity:    len(angularVelocityData),
		linearAcceleration: len(linearAccelerationData),
		compassHeading:     len(compassHeadingData),
		orientation:        len(orientationData),
	}

	allMethodsMinDataLength = map[method]int{
		position:           0,
		linearVelocity:     0,
		angularVelocity:    0,
		linearAcceleration: 0,
		compassHeading:     0,
		orientation:        0,
	}

	defaultReplayMovementSensorFunction = linearAcceleration

	allMethodsSupported = map[method]bool{
		position:           true,
		linearAcceleration: true,
		angularVelocity:    true,
		linearVelocity:     true,
		orientation:        true,
		compassHeading:     true,
	}
)

func TestNewReplayMovementSensor(t *testing.T) {
	ctx := context.Background()

	initializePropertiesTimeout = 2 * time.Second

	cases := []struct {
		description          string
		cfg                  *Config
		expectedErr          error
		validCloudConnection bool
	}{
		{
			description: "valid config with internal cloud service",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
			},
			validCloudConnection: true,
		},
		{
			description: "bad internal cloud service",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
			},
			validCloudConnection: false,
			expectedErr:          errors.Wrap(errTestCloudConnection, errCloudConnectionFailure.Error()),
		},
		{
			description: "bad start timestamp",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				Interval: TimeInterval{
					Start: "bad timestamp",
				},
			},
			validCloudConnection: true,
			expectedErr:          errors.New("invalid time format for start time, missed during config validation"),
		},
		{
			description: "bad end timestamp",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				Interval: TimeInterval{
					End: "bad timestamp",
				},
			},
			validCloudConnection: true,
			expectedErr:          errors.New("invalid time format for end time, missed during config validation"),
		},
	}

	for _, tt := range cases {
		t.Run(tt.description, func(t *testing.T) {
			replay, _, serverClose, err := createNewReplayMovementSensor(ctx, t, tt.cfg, tt.validCloudConnection)
			if err != nil {
				test.That(t, err, test.ShouldBeError, tt.expectedErr)
				test.That(t, replay, test.ShouldBeNil)
			} else {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, replay, test.ShouldNotBeNil)

				err = replay.Close(ctx)
				test.That(t, err, test.ShouldBeNil)
			}

			if tt.validCloudConnection {
				test.That(t, serverClose(), test.ShouldBeNil)
			}
		})
	}
}

func TestReplayMovementSensorFunctions(t *testing.T) {
	ctx := context.Background()

	initializePropertiesTimeout = 2 * time.Second

	cases := []struct {
		description           string
		cfg                   *Config
		startFileNum          map[method]int
		endFileNum            map[method]int
		propertiesExpectedErr error
		methodsExpectedErr    map[method]error
		methodSupported       map[method]bool
	}{
		{
			description: "Calling method with valid filter, all methods are supported",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
			},
			startFileNum:    allMethodsMinDataLength,
			endFileNum:      allMethodsMaxDataLength,
			methodSupported: allMethodsSupported,
		},
		{
			description: "Calling method with bad source",
			cfg: &Config{
				Source:         "bad_source",
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
			},
			propertiesExpectedErr: errPropertiesFailedToInitialize,
		},
		{
			description: "Calling method with bad robot_id",
			cfg: &Config{
				Source:         validSource,
				RobotID:        "bad_robot_id",
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
			},
			propertiesExpectedErr: errPropertiesFailedToInitialize,
		},
		{
			description: "Calling method with bad location_id",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     "bad_location_id",
				OrganizationID: validOrganizationID,
			},
			propertiesExpectedErr: errPropertiesFailedToInitialize,
		},
		{
			description: "Calling method with bad organization_id",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: "bad_organization_id",
			},
			propertiesExpectedErr: errPropertiesFailedToInitialize,
		},
		{
			description: "Calling method with filter no data, no methods are supported",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				BatchSize:      &batchSizeNonZero,
				Interval: TimeInterval{
					Start: "2000-01-01T12:00:30Z",
					End:   "2000-01-01T12:00:40Z",
				},
			},
			propertiesExpectedErr: errPropertiesFailedToInitialize,
		},
		{
			description: "Calling methods with end filter, all methods supported",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				BatchSize:      &batchSizeNonZero,
				Interval: TimeInterval{
					End: "2000-01-01T12:00:03Z",
				},
			},
			startFileNum: allMethodsMinDataLength,
			endFileNum: map[method]int{
				linearAcceleration: 3,
				angularVelocity:    3,
				position:           3,
				linearVelocity:     3,
				compassHeading:     3,
				orientation:        allMethodsMaxDataLength[orientation],
			},
			methodSupported: allMethodsSupported,
		},
		{
			description: "Calling methods with start filter starting at 2",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				BatchSize:      &batchSizeNonZero,
				Interval: TimeInterval{
					Start: "2000-01-01T12:00:02Z",
				},
			},
			startFileNum: map[method]int{
				linearAcceleration: 2,
				angularVelocity:    2,
				position:           2,
				linearVelocity:     2,
				compassHeading:     2,
			},
			endFileNum: map[method]int{
				position:           allMethodsMaxDataLength[position],
				linearVelocity:     allMethodsMaxDataLength[linearVelocity],
				angularVelocity:    allMethodsMaxDataLength[angularVelocity],
				linearAcceleration: allMethodsMaxDataLength[linearAcceleration],
				compassHeading:     allMethodsMaxDataLength[compassHeading],
			},
			methodsExpectedErr: map[method]error{orientation: ErrOrientationNotSupported},
			methodSupported: map[method]bool{
				position:           true,
				linearVelocity:     true,
				angularVelocity:    true,
				linearAcceleration: true,
				compassHeading:     true,
				orientation:        false,
			},
		},
		{
			description: "Calling methods with start filter starting at 6",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				BatchSize:      &batchSizeNonZero,
				Interval: TimeInterval{
					Start: "2000-01-01T12:00:06Z",
				},
			},
			startFileNum: map[method]int{
				angularVelocity: 6,
				linearVelocity:  6,
				compassHeading:  6,
			},
			endFileNum: map[method]int{
				angularVelocity: allMethodsMaxDataLength[angularVelocity],
				linearVelocity:  allMethodsMaxDataLength[linearVelocity],
				compassHeading:  allMethodsMaxDataLength[compassHeading],
			},
			methodsExpectedErr: map[method]error{
				linearAcceleration: errLinearAccelerationNotSupported,
				position:           errPositionNotSupported,
				orientation:        errOrientationNotSupported,
			},
			methodSupported: map[method]bool{
				linearAcceleration: false,
				angularVelocity:    true,
				position:           false,
				linearVelocity:     true,
				compassHeading:     true,
				orientation:        false,
			},
		},
		{
			description: "Calling methods with start filter starting at 8",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				BatchSize:      &batchSizeNonZero,
				Interval: TimeInterval{
					Start: "2000-01-01T12:00:08Z",
				},
			},
			startFileNum: map[method]int{
				linearVelocity: 8,
				compassHeading: 8,
			},
			endFileNum: map[method]int{
				linearVelocity: allMethodsMaxDataLength[linearVelocity],
				compassHeading: allMethodsMaxDataLength[compassHeading],
			},
			methodsExpectedErr: map[method]error{
				linearAcceleration: errLinearAccelerationNotSupported,
				angularVelocity:    errAngularVelocityNotSupported,
				position:           errPositionNotSupported,
				orientation:        errOrientationNotSupported,
			},
			methodSupported: map[method]bool{
				linearAcceleration: false,
				angularVelocity:    false,
				position:           false,
				linearVelocity:     true,
				compassHeading:     true,
				orientation:        false,
			},
		},
		{
			description: "Calling methods with start filter starting at 10",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				BatchSize:      &batchSizeNonZero,
				Interval: TimeInterval{
					Start: "2000-01-01T12:00:10Z",
				},
			},
			startFileNum: map[method]int{
				compassHeading: 10,
			},
			endFileNum: map[method]int{
				compassHeading: allMethodsMaxDataLength[compassHeading],
			},
			methodsExpectedErr: map[method]error{
				linearAcceleration: errLinearAccelerationNotSupported,
				angularVelocity:    errAngularVelocityNotSupported,
				position:           errPositionNotSupported,
				linearVelocity:     errLinearVelocityNotSupported,
				orientation:        errOrientationNotSupported,
			},
			methodSupported: map[method]bool{
				linearAcceleration: false,
				angularVelocity:    false,
				position:           false,
				linearVelocity:     false,
				compassHeading:     true,
				orientation:        false,
			},
		},
		{
			description: "Calling methods with start filter starting at 12",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				BatchSize:      &batchSizeNonZero,
				Interval: TimeInterval{
					Start: "2000-01-01T12:00:12Z",
				},
			},
			propertiesExpectedErr: errPropertiesFailedToInitialize,
		},
		{
			description: "Calling methods with start and end filter",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				BatchSize:      &batchSizeNonZero,
				Interval: TimeInterval{
					Start: "2000-01-01T12:00:01Z",
					End:   "2000-01-01T12:00:03Z",
				},
			},
			startFileNum: map[method]int{
				position:           1,
				linearVelocity:     1,
				angularVelocity:    1,
				linearAcceleration: 1,
				compassHeading:     1,
				orientation:        1,
			},
			endFileNum: map[method]int{
				position:           3,
				linearVelocity:     3,
				angularVelocity:    3,
				linearAcceleration: 3,
				compassHeading:     3,
				orientation:        allMethodsMaxDataLength[orientation],
			},
			methodSupported: allMethodsSupported,
		},
	}

	for _, tt := range cases {
		t.Run(tt.description, func(t *testing.T) {
			replay, _, serverClose, err := createNewReplayMovementSensor(ctx, t, tt.cfg, true)
			if tt.propertiesExpectedErr != nil {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tt.propertiesExpectedErr.Error())
				test.That(t, replay, test.ShouldBeNil)
			} else {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, replay, test.ShouldNotBeNil)

				props, err := replay.Properties(ctx, map[string]interface{}{})
				test.That(t, err, test.ShouldBeNil)
				test.That(t, props.PositionSupported, test.ShouldEqual, tt.methodSupported[position])
				test.That(t, props.OrientationSupported, test.ShouldEqual, tt.methodSupported[orientation])
				test.That(t, props.AngularVelocitySupported, test.ShouldEqual, tt.methodSupported[angularVelocity])
				test.That(t, props.LinearAccelerationSupported, test.ShouldEqual, tt.methodSupported[linearAcceleration])
				test.That(t, props.LinearVelocitySupported, test.ShouldEqual, tt.methodSupported[linearVelocity])
				test.That(t, props.CompassHeadingSupported, test.ShouldEqual, tt.methodSupported[compassHeading])

				for _, method := range methodList {
					if tt.methodsExpectedErr[method] != nil {
						testReplayMovementSensorMethodError(ctx, t, replay, method, tt.methodsExpectedErr[method])
					} else {
						// Iterate through all files that meet the provided filter
						if _, ok := tt.startFileNum[method]; ok {
							for i := tt.startFileNum[method]; i < tt.endFileNum[method]; i++ {
								testReplayMovementSensorMethodData(ctx, t, replay, method, i)
							}
						}
						// Confirm the end of the dataset was reached when expected
						testReplayMovementSensorMethodError(ctx, t, replay, method, errEndOfDataset)
					}
				}

				err = replay.Close(ctx)
				test.That(t, err, test.ShouldBeNil)

				test.That(t, serverClose(), test.ShouldBeNil)
			}
		})
	}
}

func TestReplayMovementSensorConfigValidation(t *testing.T) {
	cases := []struct {
		description  string
		cfg          *Config
		expectedDeps []string
		expectedErr  error
	}{
		{
			description: "Valid config and no timestamp",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				Interval:       TimeInterval{},
			},
			expectedDeps: []string{cloud.InternalServiceName.String()},
		},
		{
			description: "Valid config with start timestamp",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				Interval: TimeInterval{
					Start: "2000-01-01T12:00:00Z",
				},
			},
			expectedDeps: []string{cloud.InternalServiceName.String()},
		},
		{
			description: "Valid config with end timestamp",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				Interval: TimeInterval{
					End: "2000-01-01T12:00:00Z",
				},
			},
			expectedDeps: []string{cloud.InternalServiceName.String()},
		},
		{
			description: "Valid config with start and end timestamps",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				Interval: TimeInterval{
					Start: "2000-01-01T12:00:00Z",
					End:   "2000-01-01T12:00:01Z",
				},
			},
			expectedDeps: []string{cloud.InternalServiceName.String()},
		},
		{
			description: "Invalid config no source",
			cfg: &Config{
				Interval: TimeInterval{},
			},
			expectedErr: utils.NewConfigValidationFieldRequiredError("", validSource),
		},
		{
			description: "Invalid config no robot_id",
			cfg: &Config{
				Source:         validSource,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				Interval:       TimeInterval{},
			},
			expectedErr: utils.NewConfigValidationFieldRequiredError("", validRobotID),
		},
		{
			description: "Invalid config no location_id",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				OrganizationID: validOrganizationID,
				Interval:       TimeInterval{},
			},
			expectedErr: utils.NewConfigValidationFieldRequiredError("", validLocationID),
		},
		{
			description: "Invalid config no organization_id",
			cfg: &Config{
				Source:     validSource,
				RobotID:    validRobotID,
				LocationID: validLocationID,
				Interval:   TimeInterval{},
			},
			expectedErr: utils.NewConfigValidationFieldRequiredError("", validOrganizationID),
		},
		{
			description: "Invalid config with bad start timestamp format",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				Interval: TimeInterval{
					Start: "gibberish",
				},
			},
			expectedErr: errors.New("invalid time format for start time (UTC), use RFC3339"),
		},
		{
			description: "Invalid config with bad end timestamp format",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				Interval: TimeInterval{
					End: "gibberish",
				},
			},
			expectedErr: errors.New("invalid time format for end time (UTC), use RFC3339"),
		},
		{
			description: "Invalid config with bad start timestamp",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				Interval: TimeInterval{
					Start: "3000-01-01T12:00:00Z",
				},
			},
			expectedErr: errors.New("invalid config, start time (UTC) must be in the past"),
		},
		{
			description: "Invalid config with bad end timestamp",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				Interval: TimeInterval{
					End: "3000-01-01T12:00:00Z",
				},
			},
			expectedErr: errors.New("invalid config, end time (UTC) must be in the past"),
		},
		{
			description: "Invalid config with start after end timestamps",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				Interval: TimeInterval{
					Start: "2000-01-01T12:00:01Z",
					End:   "2000-01-01T12:00:00Z",
				},
			},
			expectedErr: errors.New("invalid config, end time (UTC) must be after start time (UTC)"),
		},
		{
			description: "Invalid config with batch size of 0",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				Interval: TimeInterval{
					Start: "2000-01-01T12:00:00Z",
					End:   "2000-01-01T12:00:01Z",
				},
				BatchSize: &batchSizeZero,
			},
			expectedErr: errors.New("batch_size must be between 1 and 1000"),
		},
		{
			description: "Invalid config with batch size above max",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				Interval: TimeInterval{
					Start: "2000-01-01T12:00:00Z",
					End:   "2000-01-01T12:00:01Z",
				},
				BatchSize: &batchSizeOutOfBounds,
			},
			expectedErr: errors.New("batch_size must be between 1 and 1000"),
		},
	}

	for _, tt := range cases {
		t.Run(tt.description, func(t *testing.T) {
			deps, err := tt.cfg.Validate("")
			if tt.expectedErr != nil {
				test.That(t, err, test.ShouldBeError, tt.expectedErr)
			} else {
				test.That(t, err, test.ShouldBeNil)
			}
			test.That(t, deps, test.ShouldResemble, tt.expectedDeps)
		})
	}
}

func TestUnimplementedFunctionAccuracy(t *testing.T) {
	ctx := context.Background()

	initializePropertiesTimeout = 2 * time.Second

	cfg := &Config{
		Source:         validSource,
		RobotID:        validRobotID,
		LocationID:     validLocationID,
		OrganizationID: validOrganizationID,
	}
	replay, _, serverClose, err := createNewReplayMovementSensor(ctx, t, cfg, true)
	test.That(t, err, test.ShouldBeNil)

	acc, err := replay.Accuracy(ctx, map[string]interface{}{})
	test.That(t, err, test.ShouldResemble, movementsensor.ErrMethodUnimplementedAccuracy)
	test.That(t, acc, test.ShouldResemble, map[string]float32{})

	err = replay.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, serverClose(), test.ShouldBeNil)
}

func TestReplayMovementSensorReadings(t *testing.T) {
	ctx := context.Background()

	initializePropertiesTimeout = 2 * time.Second

	cfg := &Config{
		Source:         validSource,
		RobotID:        validRobotID,
		LocationID:     validLocationID,
		OrganizationID: validOrganizationID,
	}
	replay, _, serverClose, err := createNewReplayMovementSensor(ctx, t, cfg, true)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, replay, test.ShouldNotBeNil)

	// For loop depends on the data length of orientation as it has the fewest points of data
	for i := 0; i < allMethodsMaxDataLength[orientation]; i++ {
		readings, err := replay.Readings(ctx, map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, readings["position"], test.ShouldResemble, positionPointData[i])
		test.That(t, readings["altitude"], test.ShouldResemble, positionAltitudeData[i])
		test.That(t, readings["linear_velocity"], test.ShouldResemble, linearVelocityData[i])
		test.That(t, readings["angular_velocity"], test.ShouldResemble, angularVelocityData[i])
		test.That(t, readings["linear_acceleration"], test.ShouldResemble, linearAccelerationData[i])
		test.That(t, readings["compass"], test.ShouldResemble, compassHeadingData[i])
		test.That(t, readings["orientation"], test.ShouldResemble, orientationData[i])
	}

	readings, err := replay.Readings(ctx, map[string]interface{}{})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, errEndOfDataset.Error())
	test.That(t, readings, test.ShouldBeNil)

	err = replay.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, serverClose(), test.ShouldBeNil)
}

func TestReplayMovementSensorTimestampsMetadata(t *testing.T) {
	initializePropertiesTimeout = 2 * time.Second

	// Construct replay movement sensor.
	ctx := context.Background()
	cfg := &Config{
		Source:         validSource,
		RobotID:        validRobotID,
		LocationID:     validLocationID,
		OrganizationID: validOrganizationID,
		BatchSize:      &batchSizeNonZero,
	}
	replay, _, serverClose, err := createNewReplayMovementSensor(ctx, t, cfg, true)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, replay, test.ShouldNotBeNil)

	// Repeatedly call the default method, checking for timestamps in the gRPC header.
	for i := 0; i < allMethodsMaxDataLength[defaultReplayMovementSensorFunction]; i++ {
		serverStream := testutils.NewServerTransportStream()
		ctx = grpc.NewContextWithServerTransportStream(ctx, serverStream)

		testReplayMovementSensorMethodData(ctx, t, replay, defaultReplayMovementSensorFunction, i)

		expectedTimeReq := fmt.Sprintf(testTime, i)
		expectedTimeRec := fmt.Sprintf(testTime, i+1)

		actualTimeReq := serverStream.Value(contextutils.TimeRequestedMetadataKey)[0]
		actualTimeRec := serverStream.Value(contextutils.TimeReceivedMetadataKey)[0]

		test.That(t, expectedTimeReq, test.ShouldEqual, actualTimeReq)
		test.That(t, expectedTimeRec, test.ShouldEqual, actualTimeRec)
	}

	// Confirm the end of the dataset was reached when expected
	testReplayMovementSensorMethodError(ctx, t, replay, defaultReplayMovementSensorFunction, errEndOfDataset)

	err = replay.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, serverClose(), test.ShouldBeNil)
}

func TestReplayMovementSensorReconfigure(t *testing.T) {
	initializePropertiesTimeout = 2 * time.Second

	// Construct replay movement sensor
	cfg := &Config{
		Source:         validSource,
		RobotID:        validRobotID,
		LocationID:     validLocationID,
		OrganizationID: validOrganizationID,
	}
	ctx := context.Background()
	replay, deps, serverClose, err := createNewReplayMovementSensor(ctx, t, cfg, true)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, replay, test.ShouldNotBeNil)

	// Call default movement sensor function to iterate through a few files
	for i := 0; i < 3; i++ {
		testReplayMovementSensorMethodData(ctx, t, replay, defaultReplayMovementSensorFunction, i)
	}

	// Reconfigure with a new batch size
	cfg = &Config{Source: validSource, BatchSize: &batchSize4}
	replay.Reconfigure(ctx, deps, resource.Config{ConvertedAttributes: cfg})

	// Call the default movement sensor function a couple more times, ensuring that we start over from
	// the beginning of the dataset after calling Reconfigure
	for i := 0; i < 5; i++ {
		testReplayMovementSensorMethodData(ctx, t, replay, defaultReplayMovementSensorFunction, i)
	}

	// Reconfigure again, batch size 1
	cfg = &Config{Source: validSource, BatchSize: &batchSizeNonZero}
	replay.Reconfigure(ctx, deps, resource.Config{ConvertedAttributes: cfg})

	// Again verify dataset starts from beginning
	for i := 0; i < allMethodsMaxDataLength[defaultReplayMovementSensorFunction]; i++ {
		testReplayMovementSensorMethod(ctx, t, replay, defaultReplayMovementSensorFunction, i, nil)
	}

	// Confirm the end of the dataset was reached when expected
	testReplayMovementSensorMethodError(ctx, t, replay, defaultReplayMovementSensorFunction, errEndOfDataset)

	err = replay.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, serverClose(), test.ShouldBeNil)
}
