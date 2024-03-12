package replaypcd

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"
	"google.golang.org/grpc"

	"go.viam.com/rdk/internal/cloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/utils/contextutils"
)

const (
	validSource         = "source"
	validRobotID        = "robot_id"
	validOrganizationID = "organization_id"
	validLocationID     = "location_id"
	validAPIKey         = "a key"
	validAPIKeyID       = "a key id"
)

type fileType string

const (
	pcd   fileType = ".pcd"
	jpeg  fileType = ".jpeg"
	depth fileType = ".dep"
)

type cameraType int64

const (
	unspecifiedCamera cameraType = iota
	lidar
	monoCamera
	rgbdCamera
)

var (
	batchSize0        = uint64(0)
	batchSize1        = uint64(1)
	batchSize2        = uint64(2)
	batchSize3        = uint64(3)
	batchSize4        = uint64(4)
	batchSize7        = uint64(7)
	batchSizeLarge    = uint64(50)
	batchSizeTooLarge = uint64(1000)

	datasetDirectories = map[method]map[fileType]string{
		nextPointCloud: {
			pcd: "slam/mock_lidar/%d.pcd",
		},
		getImages: {
			jpeg:  "slam/mock_rgbd/rgb/%d.jpeg",
			depth: "slam/mock_rgbd/depth/%d.dep",
		},
	}

	currentRGBDFileType = jpeg

	numFilesOriginal = map[method]int{
		nextPointCloud: 15,
		getImages:      26,
	}

	numFiles = map[method]int{
		nextPointCloud: numFilesOriginal[nextPointCloud],
		getImages:      numFilesOriginal[getImages],
	}
)

func TestNewReplayCamera(t *testing.T) {
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
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
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
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
			},
			validCloudConnection: false,
			expectedErr:          errors.New("failure to connect to the cloud: cloud connection error"),
		},
		{
			description: "bad start timestamp",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
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
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
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
			replayCamera, _, serverClose, err := createNewReplayCamera(ctx, t, tt.cfg, tt.validCloudConnection, unspecifiedCamera)
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

func TestReplayCameraNextPointCloud(t *testing.T) {
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
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
			},
			startFileNum: 0,
			endFileNum:   numFiles[nextPointCloud],
		},
		{
			description: "Calling NextPointCloud with bad source",
			cfg: &Config{
				Source:         "bad_source",
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
			},
			startFileNum: -1,
			endFileNum:   -1,
		},
		{
			description: "Calling NextPointCloud with bad robot_id",
			cfg: &Config{
				Source:         validSource,
				RobotID:        "bad_robot_id",
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
			},
			startFileNum: -1,
			endFileNum:   -1,
		},
		{
			description: "Calling NextPointCloud with bad location_id",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     "bad_location_id",
				OrganizationID: validOrganizationID,
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
			},
			startFileNum: -1,
			endFileNum:   -1,
		},
		{
			description: "Calling NextPointCloud with bad organization_id",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: "bad_organization_id",
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
			},
			startFileNum: -1,
			endFileNum:   -1,
		},
		{
			description: "Calling NextPointCloud with filter no data",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
				BatchSize:      &batchSize1,
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
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
				BatchSize:      &batchSize1,
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
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
				BatchSize:      &batchSize1,
				Interval: TimeInterval{
					Start: "2000-01-01T12:00:05Z",
				},
			},
			startFileNum: 5,
			endFileNum:   numFiles[nextPointCloud],
		},
		{
			description: "Calling NextPointCloud with start and end filter",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
				BatchSize:      &batchSize1,
				Interval: TimeInterval{
					Start: "2000-01-01T12:00:05Z",
					End:   "2000-01-01T12:00:10Z",
				},
			},
			startFileNum: 5,
			endFileNum:   10,
		},
		{
			description: "Calling NextPointCloud with non-divisible batch size, last batch size 1",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
				BatchSize:      &batchSize2,
			},
			startFileNum: 0,
			endFileNum:   numFiles[nextPointCloud],
		},
		{
			description: "Calling NextPointCloud with non-divisible batch size, last batch > 1",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				BatchSize:      &batchSize4,
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
			},
			startFileNum: 0,
			endFileNum:   numFiles[nextPointCloud],
		},
		{
			description: "Calling NextPointCloud with divisible batch size",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				BatchSize:      &batchSize3,
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
			},
			startFileNum: 0,
			endFileNum:   numFiles[nextPointCloud],
		},
		{
			description: "Calling NextPointCloud with batching and a start and end filter",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
				BatchSize:      &batchSize2,
				Interval: TimeInterval{
					Start: "2000-01-01T12:00:05Z",
					End:   "2000-01-01T12:00:10Z",
				},
			},
			startFileNum: 5,
			endFileNum:   11,
		},
		{
			description: "Calling NextPointCloud with a large batch size",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				BatchSize:      &batchSizeLarge,
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
			},
			startFileNum: 0,
			endFileNum:   numFiles[nextPointCloud],
		},
	}

	for _, tt := range cases {
		t.Run(tt.description, func(t *testing.T) {
			replayCamera, _, serverClose, err := createNewReplayCamera(ctx, t, tt.cfg, true, lidar)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, replayCamera, test.ShouldNotBeNil)

			// Iterate through all files that meet the provided filter
			if tt.startFileNum != -1 {
				for i := tt.startFileNum; i < tt.endFileNum; i++ {
					pc, err := replayCamera.NextPointCloud(ctx)
					test.That(t, err, test.ShouldBeNil)
					pcExpected, err := getPointCloudFromArtifact(i, lidar)
					if err != nil {
						test.That(t, err.Error, test.ShouldContainSubstring, "artifact not found")
						test.That(t, pc, test.ShouldBeNil)
					} else {
						test.That(t, err, test.ShouldBeNil)
						test.That(t, pc, test.ShouldResemble, pcExpected)
					}
				}
			}

			// Confirm the end of the dataset was reached when expected
			pc, err := replayCamera.NextPointCloud(ctx)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, ErrEndOfDataset.Error())
			test.That(t, pc, test.ShouldBeNil)

			err = replayCamera.Close(ctx)
			test.That(t, err, test.ShouldBeNil)

			test.That(t, serverClose(), test.ShouldBeNil)
		})
	}
}

func TestReplayCameraImages(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		description  string
		cfg          *Config
		startFileNum int
		endFileNum   int
	}{
		{
			description: "Calling Images no filter",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
			},
			startFileNum: 0,
			endFileNum:   numFiles[getImages],
		},
		{
			description: "Calling Images with bad source",
			cfg: &Config{
				Source:         "bad_source",
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
			},
			startFileNum: -1,
			endFileNum:   -1,
		},
		{
			description: "Calling Images with bad robot_id",
			cfg: &Config{
				Source:         validSource,
				RobotID:        "bad_robot_id",
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
			},
			startFileNum: -1,
			endFileNum:   -1,
		},
		{
			description: "Calling Images with bad location_id",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     "bad_location_id",
				OrganizationID: validOrganizationID,
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
			},
			startFileNum: -1,
			endFileNum:   -1,
		},
		{
			description: "Calling Images with bad organization_id",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: "bad_organization_id",
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
			},
			startFileNum: -1,
			endFileNum:   -1,
		},
		{
			description: "Calling Images with filter no data",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
				BatchSize:      &batchSize1,
				Interval: TimeInterval{
					Start: "2000-01-01T12:00:30Z",
					End:   "2000-01-01T12:00:40Z",
				},
			},
			startFileNum: -1,
			endFileNum:   -1,
		},
		{
			description: "Calling Images with end filter",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
				BatchSize:      &batchSize1,
				Interval: TimeInterval{
					End: "2000-01-01T12:00:10Z",
				},
			},
			startFileNum: 0,
			endFileNum:   10,
		},
		{
			description: "Calling Images with start filter",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
				BatchSize:      &batchSize1,
				Interval: TimeInterval{
					Start: "2000-01-01T12:00:05Z",
				},
			},
			startFileNum: 5,
			endFileNum:   numFiles[getImages],
		},
		{
			description: "Calling Images with start and end filter",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
				BatchSize:      &batchSize1,
				Interval: TimeInterval{
					Start: "2000-01-01T12:00:05Z",
					End:   "2000-01-01T12:00:10Z",
				},
			},
			startFileNum: 5,
			endFileNum:   10,
		},
		{
			description: "Calling Images with non-divisible batch size, last batch size 1",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
				BatchSize:      &batchSize7,
			},
			startFileNum: 0,
			endFileNum:   numFiles[getImages],
		},
		{
			description: "Calling Images with non-divisible batch size, last batch size 2",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				BatchSize:      &batchSize3,
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
			},
			startFileNum: 0,
			endFileNum:   numFiles[getImages],
		},
		{
			description: "Calling Images with divisible batch size",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				BatchSize:      &batchSize3,
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
			},
			startFileNum: 0,
			endFileNum:   numFiles[getImages],
		},
		{
			description: "Calling Images with batching and a start and end filter",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
				BatchSize:      &batchSize2,
				Interval: TimeInterval{
					Start: "2000-01-01T12:00:05Z",
					End:   "2000-01-01T12:00:10Z",
				},
			},
			startFileNum: 5,
			endFileNum:   11,
		},
		{
			description: "Calling Images with a large batch size",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				BatchSize:      &batchSizeLarge,
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
			},
			startFileNum: 0,
			endFileNum:   numFiles[getImages],
		},
	}

	for _, tt := range cases {
		t.Run(tt.description, func(t *testing.T) {
			for _, camType := range []cameraType{monoCamera, rgbdCamera} {
				replayCamera, _, serverClose, err := createNewReplayCamera(ctx, t, tt.cfg, true, camType)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, replayCamera, test.ShouldNotBeNil)
				fmt.Println("")
				fmt.Println("test description: ", tt.description)
				fmt.Println("")

				// Iterate through all files that meet the provided filter
				if tt.startFileNum != -1 {
					for i := tt.startFileNum; i < tt.endFileNum; i++ {
						fmt.Println("")
						fmt.Println("file number: ", i)
						images, _, err := replayCamera.Images(ctx)
						fmt.Println("images error: ", err)
						fmt.Println("")
						test.That(t, err, test.ShouldBeNil)
						_, err = getImagesFromArtifact(i, camType)
						if err != nil {
							test.That(t, err.Error, test.ShouldContainSubstring, "artifact not found")
							test.That(t, images, test.ShouldBeNil)
						} else {
							test.That(t, err, test.ShouldBeNil)
							// test.That(t, images, test.ShouldResemble, imagesExpected)
						}
					}
				}

				// Confirm the end of the dataset was reached when expected
				images, _, err := replayCamera.Images(ctx)
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, ErrEndOfDataset.Error())
				test.That(t, images, test.ShouldBeNil)

				err = replayCamera.Close(ctx)
				test.That(t, err, test.ShouldBeNil)

				test.That(t, serverClose(), test.ShouldBeNil)

			}
		})
	}
}

// TestReplayCameraLiveNextPointCloud checks the replay pcd camera's ability to handle new data being
// added to the database the pool during a session, proving that NextPointCloud can return new data
// even after returning errEndOfDataset.
func TestReplayCameraLiveNextPointCloud(t *testing.T) {
	ctx := context.Background()

	numFiles[nextPointCloud] = 10
	defer func() { numFiles[nextPointCloud] = numFilesOriginal[nextPointCloud] }()

	cfg := &Config{
		Source:         validSource,
		RobotID:        validRobotID,
		LocationID:     validLocationID,
		OrganizationID: validOrganizationID,
		APIKey:         validAPIKey,
		APIKeyID:       validAPIKeyID,
	}

	replayCamera, _, serverClose, err := createNewReplayCamera(ctx, t, cfg, true, lidar)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, replayCamera, test.ShouldNotBeNil)

	// Iterate through all files that meet the provided filter
	i := 0
	for {
		pc, err := replayCamera.NextPointCloud(ctx)
		if i == numFiles[nextPointCloud] {
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, ErrEndOfDataset.Error())
			test.That(t, pc, test.ShouldBeNil)

			// Add new files for future processing
			numFiles[nextPointCloud] += rand.Intn(3)

			if numFiles[nextPointCloud] >= numFilesOriginal[nextPointCloud] {
				break
			}
		} else {
			pcExpected, err := getPointCloudFromArtifact(i, lidar)
			if err != nil {
				test.That(t, err.Error, test.ShouldContainSubstring, "artifact not found")
				test.That(t, pc, test.ShouldBeNil)
			} else {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, pc, test.ShouldResemble, pcExpected)
			}
			i++
		}
	}

	err = replayCamera.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, serverClose(), test.ShouldBeNil)
}

func TestReplayCameraConfigValidation(t *testing.T) {
	cases := []struct {
		description  string
		cfg          *Config
		expectedDeps []string
		expectedErr  error
	}{
		{
			description: "Valid config with source and no timestamp",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				Interval:       TimeInterval{},
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
			},
			expectedDeps: []string{cloud.InternalServiceName.String()},
		},
		{
			description: "Valid config with no source",
			cfg: &Config{
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				Interval:       TimeInterval{},
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
			},
			expectedErr: resource.NewConfigValidationFieldRequiredError("", validSource),
		},
		{
			description: "Valid config with no robot_id",
			cfg: &Config{
				Source:         validSource,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				Interval:       TimeInterval{},
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
			},
			expectedErr: resource.NewConfigValidationFieldRequiredError("", validRobotID),
		},
		{
			description: "Valid config with no location_id",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				OrganizationID: validOrganizationID,
				Interval:       TimeInterval{},
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
			},
			expectedErr: resource.NewConfigValidationFieldRequiredError("", validLocationID),
		},
		{
			description: "Valid config with no organization_id",
			cfg: &Config{
				Source:     validSource,
				RobotID:    validRobotID,
				LocationID: validLocationID,
				Interval:   TimeInterval{},
				APIKey:     validAPIKey,
				APIKeyID:   validAPIKeyID,
			},
			expectedErr: resource.NewConfigValidationFieldRequiredError("", validOrganizationID),
		},
		{
			description: "Valid config with start timestamp",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
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
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
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
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
				Interval: TimeInterval{
					Start: "2000-01-01T12:00:00Z",
					End:   "2000-01-01T12:00:01Z",
				},
			},
			expectedDeps: []string{cloud.InternalServiceName.String()},
		},
		{
			description: "Invalid config with bad start timestamp format",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
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
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
				Interval: TimeInterval{
					End: "gibberish",
				},
			},
			expectedErr: errors.New("invalid time format for end time (UTC), use RFC3339"),
		},
		{
			description: "Invalid config with start after end timestamps",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
				Interval: TimeInterval{
					Start: "2000-01-01T12:00:01Z",
					End:   "2000-01-01T12:00:00Z",
				},
			},
			expectedErr: errors.New("invalid config, end time (UTC) must be after start time (UTC)"),
		},
		{
			description: "Invalid config with batch size above max",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
				Interval: TimeInterval{
					Start: "2000-01-01T12:00:00Z",
					End:   "2000-01-01T12:00:01Z",
				},
				BatchSize: &batchSizeTooLarge,
			},
			expectedErr: errors.New("batch_size must be between 1 and 100"),
		},
		{
			description: "Invalid config with batch size 0",
			cfg: &Config{
				Source:         validSource,
				RobotID:        validRobotID,
				LocationID:     validLocationID,
				OrganizationID: validOrganizationID,
				APIKey:         validAPIKey,
				APIKeyID:       validAPIKeyID,
				Interval: TimeInterval{
					Start: "2000-01-01T12:00:00Z",
					End:   "2000-01-01T12:00:01Z",
				},
				BatchSize: &batchSize0,
			},
			expectedErr: errors.New("batch_size must be between 1 and 100"),
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

func TestReplayCameraUnimplementedFunctions(t *testing.T) {
	ctx := context.Background()

	replayCamCfg := &Config{
		Source:         validSource,
		RobotID:        validRobotID,
		LocationID:     validLocationID,
		OrganizationID: validOrganizationID,
	}
	replayCamera, _, serverClose, err := createNewReplayCamera(ctx, t, replayCamCfg, true, lidar)
	test.That(t, err, test.ShouldBeNil)

	t.Run("Stream", func(t *testing.T) {
		_, err := replayCamera.Stream(ctx, nil)
		test.That(t, err.Error(), test.ShouldEqual, "Stream is unimplemented")
	})

	t.Run("Projector", func(t *testing.T) {
		_, err := replayCamera.Projector(ctx)
		test.That(t, err.Error(), test.ShouldEqual, "Projector is unimplemented")
	})

	err = replayCamera.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, serverClose(), test.ShouldBeNil)
}

func TestReplayCameraTimestamps(t *testing.T) {
	testCameraWithCfg := func(cfg *Config) {
		// Construct replay camera.
		ctx := context.Background()
		camType := lidar
		replayCamera, _, serverClose, err := createNewReplayCamera(ctx, t, cfg, true, camType)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, replayCamera, test.ShouldNotBeNil)

		// Repeatedly call NextPointCloud, checking for timestamps in the gRPC header.
		for i := 0; i < numFiles[nextPointCloud]; i++ {
			serverStream := testutils.NewServerTransportStream()
			ctx = grpc.NewContextWithServerTransportStream(ctx, serverStream)
			pc, err := replayCamera.NextPointCloud(ctx)
			test.That(t, err, test.ShouldBeNil)
			pcExpected, err := getPointCloudFromArtifact(i, camType)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, pc, test.ShouldResemble, pcExpected)

			expectedTimeReq := fmt.Sprintf(testTime, i)
			expectedTimeRec := fmt.Sprintf(testTime, i+1)

			actualTimeReq := serverStream.Value(contextutils.TimeRequestedMetadataKey)[0]
			actualTimeRec := serverStream.Value(contextutils.TimeReceivedMetadataKey)[0]

			test.That(t, expectedTimeReq, test.ShouldEqual, actualTimeReq)
			test.That(t, expectedTimeRec, test.ShouldEqual, actualTimeRec)
		}

		// Confirm the end of the dataset was reached when expected
		pc, err := replayCamera.NextPointCloud(ctx)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, ErrEndOfDataset.Error())
		test.That(t, pc, test.ShouldBeNil)

		err = replayCamera.Close(ctx)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, serverClose(), test.ShouldBeNil)
	}

	t.Run("no batching", func(t *testing.T) {
		cfg := &Config{
			Source:         validSource,
			RobotID:        validRobotID,
			LocationID:     validLocationID,
			OrganizationID: validOrganizationID,
		}
		testCameraWithCfg(cfg)
	})
	t.Run("with batching", func(t *testing.T) {
		cfg := &Config{
			Source:         validSource,
			RobotID:        validRobotID,
			LocationID:     validLocationID,
			OrganizationID: validOrganizationID,
			BatchSize:      &batchSize2,
		}
		testCameraWithCfg(cfg)
	})
}

func TestReplayCameraProperties(t *testing.T) {
	// Construct replay camera.
	ctx := context.Background()
	cfg := &Config{
		Source:         validSource,
		RobotID:        validRobotID,
		LocationID:     validLocationID,
		OrganizationID: validOrganizationID,
		BatchSize:      &batchSize1,
	}
	camType := lidar
	replayCamera, _, serverClose, err := createNewReplayCamera(ctx, t, cfg, true, camType)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, replayCamera, test.ShouldNotBeNil)

	props, err := replayCamera.Properties(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, props.SupportsPCD, test.ShouldBeTrue)

	err = replayCamera.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, serverClose(), test.ShouldBeNil)
}

func TestReplayCameraReconfigure(t *testing.T) {
	// Construct replay camera
	cfg := &Config{
		Source:         validSource,
		RobotID:        validRobotID,
		LocationID:     validLocationID,
		OrganizationID: validOrganizationID,
	}
	camType := lidar
	ctx := context.Background()
	replayCamera, deps, serverClose, err := createNewReplayCamera(ctx, t, cfg, true, camType)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, replayCamera, test.ShouldNotBeNil)

	// Call NextPointCloud to iterate through a few files
	for i := 0; i < 3; i++ {
		pc, err := replayCamera.NextPointCloud(ctx)
		test.That(t, err, test.ShouldBeNil)
		pcExpected, err := getPointCloudFromArtifact(i, camType)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pc, test.ShouldResemble, pcExpected)
	}

	// Reconfigure with a new batch size
	cfg = &Config{Source: validSource, BatchSize: &batchSize4}
	replayCamera.Reconfigure(ctx, deps, resource.Config{ConvertedAttributes: cfg})

	// Call NextPointCloud a couple more times, ensuring that we start over from the beginning
	// of the dataset after calling Reconfigure
	for i := 0; i < 5; i++ {
		pc, err := replayCamera.NextPointCloud(ctx)
		test.That(t, err, test.ShouldBeNil)
		pcExpected, err := getPointCloudFromArtifact(i, camType)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pc, test.ShouldResemble, pcExpected)
	}

	// Reconfigure again, batch size 1
	cfg = &Config{Source: validSource, BatchSize: &batchSize1}
	replayCamera.Reconfigure(ctx, deps, resource.Config{ConvertedAttributes: cfg})

	// Again verify dataset starts from beginning
	for i := 0; i < numFiles[nextPointCloud]; i++ {
		pc, err := replayCamera.NextPointCloud(ctx)
		test.That(t, err, test.ShouldBeNil)
		pcExpected, err := getPointCloudFromArtifact(i, camType)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pc, test.ShouldResemble, pcExpected)
	}

	// Confirm the end of the dataset was reached when expected
	pc, err := replayCamera.NextPointCloud(ctx)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, ErrEndOfDataset.Error())
	test.That(t, pc, test.ShouldBeNil)

	err = replayCamera.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, serverClose(), test.ShouldBeNil)
}
