package builtin

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	clk "github.com/benbjohnson/clock"
	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	v1 "go.viam.com/api/app/datasync/v1"
	"go.viam.com/test"
	"go.viam.com/utils/protoutils"
	"google.golang.org/protobuf/types/known/structpb"

	pbArm "go.viam.com/api/component/arm/v1"
	pbEnc "go.viam.com/api/component/encoder/v1"
	pbGan "go.viam.com/api/component/gantry/v1"
	pbMot "go.viam.com/api/component/motor/v1"
	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/encoder"
	"go.viam.com/rdk/components/gantry"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/components/powersensor"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/components/servo"
	"go.viam.com/rdk/internal/cloud"
	cloudinject "go.viam.com/rdk/internal/testutils/inject"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/services/datamanager/datacapture"
	"go.viam.com/rdk/services/datamanager/internal"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
)

var (
	// Robot config which specifies data manager service.
	enabledTabularCollectorConfigPath           = "services/datamanager/data/fake_robot_with_data_manager.json"
	enabledTabularCollectorEmptyConfigPath      = "services/datamanager/data/fake_robot_with_data_manager_empty.json"
	disabledTabularCollectorConfigPath          = "services/datamanager/data/fake_robot_with_disabled_collector.json"
	enabledBinaryCollectorConfigPath            = "services/datamanager/data/robot_with_cam_capture.json"
	infrequentCaptureTabularCollectorConfigPath = "services/datamanager/data/fake_robot_with_infrequent_capture.json"
	remoteCollectorConfigPath                   = "services/datamanager/data/fake_robot_with_remote_and_data_manager.json"
	emptyFileBytesSize                          = 100 // size of leading metadata message
	captureInterval                             = time.Millisecond * 10
)

func TestDataCaptureEnabled(t *testing.T) {
	tests := []struct {
		name                          string
		initialServiceDisableStatus   bool
		newServiceDisableStatus       bool
		initialCollectorDisableStatus bool
		newCollectorDisableStatus     bool
		remoteCollector               bool
		emptyTabular                  bool
	}{
		{
			name:                          "data capture service disabled, should capture nothing",
			initialServiceDisableStatus:   true,
			newServiceDisableStatus:       true,
			initialCollectorDisableStatus: true,
			newCollectorDisableStatus:     true,
		},
		{
			name:                          "data capture service enabled and a configured collector, should capture data",
			initialServiceDisableStatus:   false,
			newServiceDisableStatus:       false,
			initialCollectorDisableStatus: false,
			newCollectorDisableStatus:     false,
		},
		{
			name:                          "data capture service implicitly enabled and a configured collector, should capture data",
			initialServiceDisableStatus:   false,
			newServiceDisableStatus:       false,
			initialCollectorDisableStatus: false,
			newCollectorDisableStatus:     false,
			emptyTabular:                  true,
		},
		{
			name:                          "disabling data capture service should cause all data capture to stop",
			initialServiceDisableStatus:   false,
			newServiceDisableStatus:       true,
			initialCollectorDisableStatus: false,
			newCollectorDisableStatus:     false,
		},
		{
			name:                          "enabling data capture should cause all enabled collectors to start capturing data",
			initialServiceDisableStatus:   true,
			newServiceDisableStatus:       false,
			initialCollectorDisableStatus: false,
			newCollectorDisableStatus:     false,
		},
		{
			name:                          "enabling a collector should not trigger data capture if the service is disabled",
			initialServiceDisableStatus:   true,
			newServiceDisableStatus:       true,
			initialCollectorDisableStatus: true,
			newCollectorDisableStatus:     false,
		},
		{
			name:                          "disabling an individual collector should stop it",
			initialServiceDisableStatus:   false,
			newServiceDisableStatus:       false,
			initialCollectorDisableStatus: false,
			newCollectorDisableStatus:     true,
		},
		{
			name:                          "enabling an individual collector should start it",
			initialServiceDisableStatus:   false,
			newServiceDisableStatus:       false,
			initialCollectorDisableStatus: true,
			newCollectorDisableStatus:     false,
		},
		{
			name:            "capture should work for remotes too",
			remoteCollector: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set up capture directories.
			initCaptureDir := t.TempDir()
			updatedCaptureDir := t.TempDir()
			mockClock := clk.NewMock()
			// Make mockClock the package level clock used by the dmsvc so that we can simulate time's passage
			clock = mockClock

			// Set up robot config.
			var initConfig *Config
			var deps []string
			switch {
			case tc.remoteCollector:
				initConfig, deps = setupConfig(t, remoteCollectorConfigPath)
			case tc.initialCollectorDisableStatus:
				initConfig, deps = setupConfig(t, disabledTabularCollectorConfigPath)
			case tc.emptyTabular:
				initConfig, deps = setupConfig(t, enabledTabularCollectorEmptyConfigPath)
			default:
				initConfig, deps = setupConfig(t, enabledTabularCollectorConfigPath)
			}

			// further set up service config.
			initConfig.CaptureDisabled = tc.initialServiceDisableStatus
			initConfig.ScheduledSyncDisabled = true
			initConfig.CaptureDir = initCaptureDir

			// Build and start data manager.
			dmsvc, r := newTestDataManager(t)
			defer func() {
				test.That(t, dmsvc.Close(context.Background()), test.ShouldBeNil)
			}()

			resources := resourcesFromDeps(t, r, deps)
			err := dmsvc.Reconfigure(context.Background(), resources, resource.Config{
				ConvertedAttributes: initConfig,
			})
			test.That(t, err, test.ShouldBeNil)
			passTimeCtx1, cancelPassTime1 := context.WithCancel(context.Background())
			donePassingTime1 := passTime(passTimeCtx1, mockClock, captureInterval)

			if !tc.initialServiceDisableStatus && !tc.initialCollectorDisableStatus {
				waitForCaptureFilesToExceedNFiles(initCaptureDir, 0)
				testFilesContainSensorData(t, initCaptureDir)
			} else {
				initialCaptureFiles := getAllFileInfos(initCaptureDir)
				test.That(t, len(initialCaptureFiles), test.ShouldEqual, 0)
			}
			cancelPassTime1()
			<-donePassingTime1

			// Set up updated robot config.
			var updatedConfig *Config
			if tc.newCollectorDisableStatus {
				updatedConfig, deps = setupConfig(t, disabledTabularCollectorConfigPath)
			} else {
				updatedConfig, deps = setupConfig(t, enabledTabularCollectorConfigPath)
			}

			// further set up updated service config.
			updatedConfig.CaptureDisabled = tc.newServiceDisableStatus
			updatedConfig.ScheduledSyncDisabled = true
			updatedConfig.CaptureDir = updatedCaptureDir

			// Update to new config and let it run for a bit.
			resources = resourcesFromDeps(t, r, deps)
			err = dmsvc.Reconfigure(context.Background(), resources, resource.Config{
				ConvertedAttributes: updatedConfig,
			})
			test.That(t, err, test.ShouldBeNil)
			oldCaptureDirFiles := getAllFileInfos(initCaptureDir)
			passTimeCtx2, cancelPassTime2 := context.WithCancel(context.Background())
			donePassingTime2 := passTime(passTimeCtx2, mockClock, captureInterval)

			if !tc.newServiceDisableStatus && !tc.newCollectorDisableStatus {
				waitForCaptureFilesToExceedNFiles(updatedCaptureDir, 0)
				testFilesContainSensorData(t, updatedCaptureDir)
			} else {
				updatedCaptureFiles := getAllFileInfos(updatedCaptureDir)
				test.That(t, len(updatedCaptureFiles), test.ShouldEqual, 0)
				oldCaptureDirFilesAfterWait := getAllFileInfos(initCaptureDir)
				test.That(t, len(oldCaptureDirFilesAfterWait), test.ShouldEqual, len(oldCaptureDirFiles))
				for i := range oldCaptureDirFiles {
					test.That(t, oldCaptureDirFiles[i].Size(), test.ShouldEqual, oldCaptureDirFilesAfterWait[i].Size())
				}
			}
			cancelPassTime2()
			<-donePassingTime2
		})
	}
}

func TestSwitchResource(t *testing.T) {
	captureDir := t.TempDir()
	mockClock := clk.NewMock()
	// Make mockClock the package level clock used by the dmsvc so that we can simulate time's passage
	clock = mockClock

	// Set up robot config.
	config, deps := setupConfig(t, enabledTabularCollectorConfigPath)
	config.CaptureDisabled = false
	config.ScheduledSyncDisabled = true
	config.CaptureDir = captureDir

	// Build and start data manager.
	dmsvc, r := newTestDataManager(t)
	defer func() {
		test.That(t, dmsvc.Close(context.Background()), test.ShouldBeNil)
	}()

	resources := resourcesFromDeps(t, r, deps)
	err := dmsvc.Reconfigure(context.Background(), resources, resource.Config{
		ConvertedAttributes: config,
	})
	test.That(t, err, test.ShouldBeNil)
	passTimeCtx1, cancelPassTime1 := context.WithCancel(context.Background())
	donePassingTime1 := passTime(passTimeCtx1, mockClock, captureInterval)

	waitForCaptureFilesToExceedNFiles(captureDir, 0)
	testFilesContainSensorData(t, captureDir)

	cancelPassTime1()
	<-donePassingTime1

	// Change the resource named arm1 to show that the data manager recognizes that the collector has changed with no other config changes.
	for resource := range resources {
		if resource.Name == "arm1" {
			newResource := inject.NewArm(resource.Name)
			newResource.EndPositionFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
				// Return a different value from the initial arm1 resource.
				return spatialmath.NewPoseFromPoint(r3.Vector{X: 888, Y: 888, Z: 888}), nil
			}
			resources[resource] = newResource
		}
	}

	err = dmsvc.Reconfigure(context.Background(), resources, resource.Config{
		ConvertedAttributes: config,
	})
	test.That(t, err, test.ShouldBeNil)

	dataBeforeSwitch, err := getSensorData(captureDir)
	test.That(t, err, test.ShouldBeNil)

	passTimeCtx2, cancelPassTime2 := context.WithCancel(context.Background())
	donePassingTime2 := passTime(passTimeCtx2, mockClock, captureInterval)

	// Test that sensor data is captured from the new collector.
	waitForCaptureFilesToExceedNFiles(captureDir, len(getAllFileInfos(captureDir)))
	testFilesContainSensorData(t, captureDir)

	filePaths := getAllFilePaths(captureDir)
	test.That(t, len(filePaths), test.ShouldEqual, 2)

	initialData, err := datacapture.SensorDataFromFilePath(filePaths[0])
	test.That(t, err, test.ShouldBeNil)
	for _, d := range initialData {
		// Each resource's mocked capture method outputs a different value.
		// Assert that we see the expected data captured by the initial arm1 resource.
		test.That(
			t,
			d.GetStruct().GetFields()["pose"].GetStructValue().GetFields()["o_z"].GetNumberValue(), test.ShouldEqual,
			float64(1),
		)
	}
	// Assert that the initial arm1 resource isn't capturing any more data.
	test.That(t, len(initialData), test.ShouldEqual, len(dataBeforeSwitch))

	newData, err := datacapture.SensorDataFromFilePath(filePaths[1])
	test.That(t, err, test.ShouldBeNil)
	for _, d := range newData {
		// Assert that we see the expected data captured by the updated arm1 resource.
		test.That(
			t,
			d.GetStruct().GetFields()["pose"].GetStructValue().GetFields()["x"].GetNumberValue(),
			test.ShouldEqual,
			float64(888),
		)
	}
	// Assert that the updated arm1 resource is capturing data.
	test.That(t, len(newData), test.ShouldBeGreaterThan, 0)

	cancelPassTime2()
	<-donePassingTime2
}

// passTime repeatedly increments mc by interval until the context is canceled.
func passTime(ctx context.Context, mc *clk.Mock, interval time.Duration) chan struct{} {
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-ctx.Done():
				close(done)
				return
			default:
				time.Sleep(10 * time.Millisecond)
				mc.Add(interval)
			}
		}
	}()
	return done
}

func getSensorData(dir string) ([]*v1.SensorData, error) {
	var sd []*v1.SensorData
	filePaths := getAllFilePaths(dir)
	for _, path := range filePaths {
		d, err := datacapture.SensorDataFromFilePath(path)
		// It's possible a file was closed (and so its extension changed) in between the points where we gathered
		// file names and here. So if the file does not exist, check if the extension has just been changed.
		if errors.Is(err, os.ErrNotExist) {
			path = strings.TrimSuffix(path, filepath.Ext(path)) + datacapture.FileExt
			d, err = datacapture.SensorDataFromFilePath(path)
			if err != nil {
				return nil, err
			}
		} else if err != nil {
			return nil, err
		}

		sd = append(sd, d...)
	}
	return sd, nil
}

func TestCollector(t *testing.T) {
	const basePath = "services/datamanager/data/collectortests/"
	tests := []struct {
		name                        string
		config                      string
		expectedCollectedDataFormat []string
	}{
		{
			name:                        "arm end position capture should match format",
			config:                      basePath + "arm_endposition_collector.json",
			expectedCollectedDataFormat: getKeys(toProto(pbArm.GetEndPositionResponse{}).AsMap()),
		},
		{
			name:                        "arm joint position capture should match format",
			config:                      basePath + "arm_jointposition_collector.json",
			expectedCollectedDataFormat: getKeys(toProto(pbArm.GetJointPositionsResponse{}).AsMap()),
		},
		{
			name:                        "encoder tick count capture should match format",
			config:                      basePath + "encoder_tickcount_collector.json",
			expectedCollectedDataFormat: getKeys(toProto(pbEnc.GetPositionResponse{}).AsMap()),
		},
		{
			name:                        "gantry position capture should match format",
			config:                      basePath + "gantry_position_collector.json",
			expectedCollectedDataFormat: getKeys(toProto(pbGan.GetPositionResponse{}).AsMap()),
		},
		{
			name:                        "gantry lengths capture should match format",
			config:                      basePath + "gantry_lengths_collector.json",
			expectedCollectedDataFormat: getKeys(toProto(pbGan.GetLengthsResponse{}).AsMap()),
		},
		{
			name:                        "motor isPowered capture should match format",
			config:                      basePath + "motor_ispowered_collector.json",
			expectedCollectedDataFormat: getKeys(toProto(pbMot.IsPoweredResponse{}).AsMap()),
		},
		{
			name:                        "motor position capture should match format",
			config:                      basePath + "motor_position_collector.json",
			expectedCollectedDataFormat: getKeys(toProto(pbMot.GetPositionResponse{}).AsMap()),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			initCaptureDir := t.TempDir()
			mockClock := clk.NewMock()
			// Make mockClock the package level clock used by the dmsvc so that we can simulate time's passage
			clock = mockClock

			// Set up robot config.
			var initConfig *Config
			var deps []string
			initConfig, deps = setupConfig(t, tc.config)

			// further set up service config.
			initConfig.ScheduledSyncDisabled = true
			initConfig.CaptureDir = initCaptureDir

			// Build and start data manager.
			dmsvc, r := newTestDataManagerAllCollectors(t)
			defer func() {
				test.That(t, dmsvc.Close(context.Background()), test.ShouldBeNil)
			}()

			resources := resourcesFromDeps(t, r, deps)
			err := dmsvc.Reconfigure(context.Background(), resources, resource.Config{
				ConvertedAttributes: initConfig,
			})
			test.That(t, err, test.ShouldBeNil)
			passTimeCtx1, cancelPassTime1 := context.WithCancel(context.Background())
			donePassingTime1 := passTime(passTimeCtx1, mockClock, captureInterval)

			waitForCaptureFilesToExceedNFiles(initCaptureDir, 10)
			testFilesContainMatchingSensorData(t, initCaptureDir, tc.expectedCollectedDataFormat)
			cancelPassTime1()
			<-donePassingTime1
		})
	}

}

func testFilesContainMatchingSensorData(t *testing.T, dir string, expectedFormat []string) {
	t.Helper()
	sort.Strings(expectedFormat)
	sd, err := getSensorData(dir)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(sd), test.ShouldBeGreaterThan, 0)
	for _, d := range sd {
		test.That(t, d.GetStruct(), test.ShouldNotBeNil)
		test.That(t, d.GetMetadata(), test.ShouldNotBeNil)
		dataKeys := getKeys(d.GetStruct().AsMap())
		sort.Strings(dataKeys)
		test.That(t, dataKeys, test.ShouldResemble, expectedFormat)
	}
}

// testFilesContainSensorData verifies that the files in `dir` contain sensor data.
func testFilesContainSensorData(t *testing.T, dir string) {
	t.Helper()

	sd, err := getSensorData(dir)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(sd), test.ShouldBeGreaterThan, 0)
	for _, d := range sd {
		test.That(t, d.GetStruct(), test.ShouldNotBeNil)
		test.That(t, d.GetMetadata(), test.ShouldNotBeNil)
	}
}

// waitForCaptureFilesToExceedNFiles returns once `captureDir` contains more than `n` files.
func waitForCaptureFilesToExceedNFiles(captureDir string, n int) {
	totalWait := time.Second * 2
	waitPerCheck := time.Millisecond * 10
	iterations := int(totalWait / waitPerCheck)
	files := getAllFileInfos(captureDir)
	for i := 0; i < iterations; i++ {
		if len(files) > n && files[n].Size() > int64(emptyFileBytesSize) {
			return
		}
		time.Sleep(waitPerCheck)
		files = getAllFileInfos(captureDir)
	}
}

func resourcesFromDeps(t *testing.T, r robot.Robot, deps []string) resource.Dependencies {
	t.Helper()
	resources := resource.Dependencies{}
	for _, dep := range deps {
		resName, err := resource.NewFromString(dep)
		test.That(t, err, test.ShouldBeNil)
		res, err := r.ResourceByName(resName)
		if err == nil {
			// some resources are weakly linked
			resources[resName] = res
		}
	}
	return resources
}

func getInjectedAllCollectedComponentsRobot() *inject.Robot {
	r := &inject.Robot{}
	rs := map[resource.Name]resource.Resource{}

	rs[cloud.InternalServiceName] = &cloudinject.CloudConnectionService{
		Named: cloud.InternalServiceName.AsNamed(),
	}
	floatList := []float64{1.0, 2.0, 3.0}
	vec := r3.Vector{
		X: 1.0,
		Y: 2.0,
		Z: 3.0,
	}

	injectedArm := &inject.Arm{}
	injectedArm.EndPositionFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error) {
		return spatialmath.NewPoseFromPoint(r3.Vector{X: 1, Y: 2, Z: 3}), nil
	}
	injectedArm.JointPositionsFunc = func(ctx context.Context, extra map[string]interface{}) (*pbArm.JointPositions, error) {
		return &pbArm.JointPositions{
			Values: floatList,
		}, nil
	}
	rs[arm.Named("arm1")] = injectedArm

	injectedBoard := &inject.Board{}
	rs[board.Named("board")] = injectedBoard

	injectedEncoder := &inject.Encoder{}
	injectedEncoder.PositionFunc = func(ctx context.Context, positionType encoder.PositionType, extra map[string]interface{}) (float64, encoder.PositionType, error) {
		return 1.0, encoder.PositionTypeTicks, nil
	}
	rs[encoder.Named("fakeenc")] = injectedEncoder

	injectedGantry := &inject.Gantry{}
	injectedGantry.PositionFunc = func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
		return floatList, nil
	}
	injectedGantry.LengthsFunc = func(ctx context.Context, extra map[string]interface{}) ([]float64, error) {
		return floatList, nil
	}
	rs[gantry.Named("gan")] = injectedGantry

	injectedMotor := &inject.Motor{}
	injectedMotor.IsPoweredFunc = func(ctx context.Context, extra map[string]interface{}) (bool, float64, error) {
		return true, .5, nil
	}
	injectedMotor.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
		return 1.0, nil
	}
	rs[motor.Named("mot")] = injectedMotor

	injectedMovementSensor := &inject.MovementSensor{}
	injectedMovementSensor.LinearVelocityFunc = func(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
		return vec, nil
	}
	injectedMovementSensor.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
		return geo.NewPoint(1.0, 2.0), 1.0, nil
	}
	injectedMovementSensor.AngularVelocityFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) {
		return spatialmath.AngularVelocity{
			X: 0.0,
			Y: 2.0,
			Z: 3.0,
		}, nil
	}
	injectedMovementSensor.CompassHeadingFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
		return 1.0, nil
	}
	injectedMovementSensor.LinearAccelerationFunc = func(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
		return vec, nil
	}
	injectedMovementSensor.OrientationFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
		return spatialmath.NewZeroOrientation(), nil
	}
	rs[movementsensor.Named("mov")] = injectedMovementSensor

	injectedPowerSensor := &inject.PowerSensor{}
	injectedPowerSensor.VoltageFunc = func(ctx context.Context, extra map[string]interface{}) (float64, bool, error) {
		return 1.0, false, nil
	}
	injectedPowerSensor.CurrentFunc = func(ctx context.Context, extra map[string]interface{}) (float64, bool, error) {
		return 1.0, false, nil
	}
	injectedPowerSensor.PowerFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
		return 1.0, nil
	}
	rs[powersensor.Named("pwr")] = injectedPowerSensor

	injectedSensor := &inject.Sensor{}
	injectedSensor.ReadingsFunc = func(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
		return map[string]any{"Reading 1": 2.0}, nil
	}
	rs[sensor.Named("sens")] = injectedSensor

	injectedServo := &inject.Servo{}
	injectedServo.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (uint32, error) {
		return 1, nil
	}

	rs[servo.Named("servo")] = injectedServo

	r.MockResourcesFromMap(rs)
	return r
}

func newTestDataManagerAllCollectors(t *testing.T) (internal.DMService, robot.Robot) {
	t.Helper()
	dmCfg := &Config{}
	cfgService := resource.Config{
		API:                 datamanager.API,
		ConvertedAttributes: dmCfg,
	}
	logger := golog.NewTestLogger(t)

	// Create local robot with injected arm and remote.
	r := getInjectedAllCollectedComponentsRobot()
	remoteRobot := getInjectedAllCollectedComponentsRobot()
	r.RemoteByNameFunc = func(name string) (robot.Robot, bool) {
		return remoteRobot, true
	}

	resources := resourcesFromDeps(t, r, []string{cloud.InternalServiceName.String()})
	svc, err := NewBuiltIn(context.Background(), resources, cfgService, logger)
	if err != nil {
		t.Log(err)
		t.FailNow()
	}
	return svc.(internal.DMService), r
}

func toProto(r interface{}) *structpb.Struct {
	msg, err := protoutils.StructToStructPbIgnoreOmitEmpty(r)
	if err != nil {
		return nil
	}
	return msg
}

func getKeys(data map[string]any) []string {
	keys := make([]string, len(data))
	i := 0
	for k := range data {
		keys[i] = k
		i++
	}
	return keys
}
