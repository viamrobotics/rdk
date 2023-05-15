package replaypcd

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/internal/cloud"
	"go.viam.com/rdk/pointcloud"
)

const datasetDirectory = "slam/mock_lidar/%d.pcd"

var numPCDFiles = 15

// getPointCloudFromArtifact will return a point cloud based on the provided artifact path.
func getPointCloudFromArtifact(t *testing.T, i int) pointcloud.PointCloud {
	path := filepath.Clean(artifact.MustPath(fmt.Sprintf(datasetDirectory, i)))
	pcdFile, err := os.Open(path)
	test.That(t, err, test.ShouldBeNil)
	defer utils.UncheckedErrorFunc(pcdFile.Close)

	pcExpected, err := pointcloud.ReadPCD(pcdFile)
	test.That(t, err, test.ShouldBeNil)

	return pcExpected
}

func TestNewReplayPCD(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		description          string
		cfg                  *Config
		expectedErr          error
		validCloudConnection bool
	}{
		{
			description: "valid config with internal cloud service",
			cfg: &Config{
				Source: "source",
			},
			validCloudConnection: true,
		},
		{
			description: "bad internal cloud service",
			cfg: &Config{
				Source: "source",
			},
			validCloudConnection: false,
			expectedErr:          errors.New("failure to connect to the cloud: cloud connection error"),
		},
		{
			description: "bad start timestamp",
			cfg: &Config{
				Source: "source",
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
				Source: "source",
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
			replayCamera, serverClose, err := createNewReplayPCDCamera(ctx, t, tt.cfg, tt.validCloudConnection)
			if err != nil {
				test.That(t, err, test.ShouldBeError, tt.expectedErr)
				test.That(t, replayCamera, test.ShouldBeNil)
			} else {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, replayCamera, test.ShouldNotBeNil)

				err = replayCamera.Close(ctx)
				test.That(t, err, test.ShouldBeNil)
			}

			if tt.validCloudConnection {
				test.That(t, serverClose(), test.ShouldBeNil)
			}
		})
	}
}

func TestNextPointCloud(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		description  string
		cfg          *Config
		startFileNum int
		endFileNum   int
	}{
		{
			description: "Calling NextPointCloud no filter",
			cfg: &Config{
				Source: "source",
			},
			startFileNum: 0,
			endFileNum:   numPCDFiles,
		},
		{
			description: "Calling NextPointCloud with filter no data",
			cfg: &Config{
				Source: "source",
				Interval: TimeInterval{
					Start: "2000-01-01T12:00:30Z",
					End:   "2000-01-01T12:00:40Z",
				},
			},
			startFileNum: -1,
			endFileNum:   -1,
		},
		{
			description: "Calling NextPointCloud with end filter",
			cfg: &Config{
				Source: "source",
				Interval: TimeInterval{
					End: "2000-01-01T12:00:10Z",
				},
			},
			startFileNum: 0,
			endFileNum:   10,
		},
		{
			description: "Calling NextPointCloud with start filter",
			cfg: &Config{
				Source: "source",
				Interval: TimeInterval{
					Start: "2000-01-01T12:00:05Z",
				},
			},
			startFileNum: 5,
			endFileNum:   numPCDFiles,
		},
		{
			description: "Calling NextPointCloud with start and end filter",
			cfg: &Config{
				Source: "source",
				Interval: TimeInterval{
					Start: "2000-01-01T12:00:05Z",
					End:   "2000-01-01T12:00:10Z",
				},
			},
			startFileNum: 5,
			endFileNum:   10,
		},
		{
			description: "Calling NextPointCloud with bad source",
			cfg: &Config{
				Source: "bad_source",
			},
			startFileNum: -1,
			endFileNum:   -1,
		},
		{
			description: "Calling NextPointCloud with robot_id",
			cfg: &Config{
				Source:  "source",
				RobotID: "robot_id",
			},
			startFileNum: 0,
			endFileNum:   numPCDFiles,
		},
		{
			description: "Calling NextPointCloud with bad robot_id",
			cfg: &Config{
				Source:  "source",
				RobotID: "bad_robot_id",
			},
			startFileNum: -1,
			endFileNum:   -1,
		},
	}

	for _, tt := range cases {
		t.Run(tt.description, func(t *testing.T) {
			replayCamera, serverClose, err := createNewReplayPCDCamera(ctx, t, tt.cfg, true)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, replayCamera, test.ShouldNotBeNil)

			// Iterate through all files that meet the provided filter
			if tt.startFileNum != -1 {
				for i := tt.startFileNum; i < tt.endFileNum; i++ {
					pc, err := replayCamera.NextPointCloud(ctx)
					test.That(t, err, test.ShouldBeNil)
					test.That(t, pc, test.ShouldResemble, getPointCloudFromArtifact(t, i))
				}
			}

			// Confirm the end of the dataset was reached when expected
			pc, err := replayCamera.NextPointCloud(ctx)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, errEndOfDataset.Error())
			test.That(t, pc, test.ShouldBeNil)

			err = replayCamera.Close(ctx)
			test.That(t, err, test.ShouldBeNil)

			test.That(t, serverClose(), test.ShouldBeNil)
		})
	}
}

func TestLiveNextPointCloud(t *testing.T) {
	ctx := context.Background()

	numPCDFilesOriginal := numPCDFiles
	numPCDFiles = 10
	defer func() { numPCDFiles = numPCDFilesOriginal }()

	cfg := &Config{
		Source: "source",
	}

	replayCamera, serverClose, err := createNewReplayPCDCamera(ctx, t, cfg, true)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, replayCamera, test.ShouldNotBeNil)

	// Iterate through all files that meet the provided filter
	i := 0
	for {
		pc, err := replayCamera.NextPointCloud(ctx)
		if i >= numPCDFiles {
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, errEndOfDataset.Error())
			test.That(t, pc, test.ShouldBeNil)

			// Add new files for future processing
			numPCDFiles += rand.Intn(3)

			if numPCDFiles >= numPCDFilesOriginal {
				break
			}
		} else {
			test.That(t, err, test.ShouldBeNil)
			test.That(t, pc, test.ShouldResemble, getPointCloudFromArtifact(t, i))
			i++
		}
	}

	err = replayCamera.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, serverClose(), test.ShouldBeNil)
}

func TestConfigValidation(t *testing.T) {
	cases := []struct {
		description  string
		cfg          *Config
		expectedDeps []string
		expectedErr  error
	}{
		{
			description: "Valid config with source and no timestamp",
			cfg: &Config{
				Source:   "source",
				Interval: TimeInterval{},
			},
			expectedDeps: []string{cloud.InternalServiceName.String()},
		},
		{
			description: "Valid config with source and any robot id",
			cfg: &Config{
				Source:  "source",
				RobotID: "source",
			},
			expectedDeps: []string{cloud.InternalServiceName.String()},
		},
		{
			description: "Valid config with start timestamp",
			cfg: &Config{
				Source: "source",
				Interval: TimeInterval{
					Start: "2000-01-01T12:00:00Z",
				},
			},
			expectedDeps: []string{cloud.InternalServiceName.String()},
		},
		{
			description: "Valid config with end timestamp",
			cfg: &Config{
				Source: "source",
				Interval: TimeInterval{
					End: "2000-01-01T12:00:00Z",
				},
			},
			expectedDeps: []string{cloud.InternalServiceName.String()},
		},
		{
			description: "Valid config with start and end timestamps",
			cfg: &Config{
				Source: "source",
				Interval: TimeInterval{
					Start: "2000-01-01T12:00:00Z",
					End:   "2000-01-01T12:00:01Z",
				},
			},
			expectedDeps: []string{cloud.InternalServiceName.String()},
		},
		{
			description: "Invalid config no source and no timestamp",
			cfg: &Config{
				Source:   "",
				Interval: TimeInterval{},
			},
			expectedErr: utils.NewConfigValidationFieldRequiredError("", "source"),
		},
		{
			description: "Invalid config with bad start timestamp format",
			cfg: &Config{
				Source: "source",
				Interval: TimeInterval{
					Start: "gibberish",
				},
			},
			expectedErr: errors.New("invalid time format for start time, use RFC3339"),
		},
		{
			description: "Invalid config with bad end timestamp format",
			cfg: &Config{
				Source: "source",
				Interval: TimeInterval{
					End: "gibberish",
				},
			},
			expectedErr: errors.New("invalid time format for end time, use RFC3339"),
		},
		{
			description: "Invalid config with bad start timestamp",
			cfg: &Config{
				Source: "source",
				Interval: TimeInterval{
					Start: "3000-01-01T12:00:00Z",
				},
			},
			expectedErr: errors.New("invalid config, start time must be in the past"),
		},
		{
			description: "Invalid config with bad end timestamp",
			cfg: &Config{
				Source: "source",
				Interval: TimeInterval{
					End: "3000-01-01T12:00:00Z",
				},
			},
			expectedErr: errors.New("invalid config, end time must be in the past"),
		},
		{
			description: "Invalid config with start after end timestamps",
			cfg: &Config{
				Source: "source",
				Interval: TimeInterval{
					Start: "2000-01-01T12:00:01Z",
					End:   "2000-01-01T12:00:00Z",
				},
			},
			expectedErr: errors.New("invalid config, end time must be after start time"),
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

func TestUnimplementedFunctions(t *testing.T) {
	ctx := context.Background()

	replayCamCfg := &Config{Source: "source"}
	replayCamera, serverClose, err := createNewReplayPCDCamera(ctx, t, replayCamCfg, true)
	test.That(t, err, test.ShouldBeNil)

	t.Run("Stream", func(t *testing.T) {
		_, err := replayCamera.Stream(ctx, nil)
		test.That(t, err.Error(), test.ShouldEqual, "Stream is unimplemented")
	})

	t.Run("Properties", func(t *testing.T) {
		_, err := replayCamera.Properties(ctx)
		test.That(t, err.Error(), test.ShouldEqual, "Properties is unimplemented")
	})

	t.Run("Projector", func(t *testing.T) {
		_, err := replayCamera.Projector(ctx)
		test.That(t, err.Error(), test.ShouldEqual, "Projector is unimplemented")
	})

	err = replayCamera.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, serverClose(), test.ShouldBeNil)
}
