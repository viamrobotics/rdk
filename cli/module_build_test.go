package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	v1 "go.viam.com/api/app/build/v1"
	apppb "go.viam.com/api/app/v1"
	"go.viam.com/test"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/testutils/inject"
)

func createTestManifest(t *testing.T, path string, overrides map[string]any) string {
	t.Helper()
	if len(path) == 0 {
		path = filepath.Join(t.TempDir(), "meta.json")
	}

	// Default manifest structure
	defaultManifest := map[string]any{
		"module_id":   "test:test",
		"visibility":  "private",
		"url":         "https://github.com/",
		"description": "a",
		"models": []any{
			map[string]any{
				"api":   "a:b:c",
				"model": "a:b:c",
			},
		},
		"build": map[string]any{
			"setup": "./setup.sh",
			"build": "make build",
			"path":  "module",
			"arch":  []any{"linux/amd64"},
		},
		"entrypoint": "bin/module",
	}

	// Apply overrides
	for key, value := range overrides {
		if value == nil {
			// nil means delete the key entirely
			delete(defaultManifest, key)
		} else {
			defaultManifest[key] = value
		}
	}

	// Marshal to JSON
	jsonBytes, err := json.MarshalIndent(defaultManifest, "", "  ")
	test.That(t, err, test.ShouldBeNil)

	// Write to file
	err = os.WriteFile(path, jsonBytes, 0o644)
	test.That(t, err, test.ShouldBeNil)

	return path
}

func TestStartBuild(t *testing.T) {
	manifest := filepath.Join(t.TempDir(), "meta.json")
	createTestManifest(t, manifest, nil)
	cCtx, ac, out, errOut := setup(&inject.AppServiceClient{}, nil, &inject.BuildServiceClient{
		StartBuildFunc: func(ctx context.Context, in *v1.StartBuildRequest, opts ...grpc.CallOption) (*v1.StartBuildResponse, error) {
			return &v1.StartBuildResponse{BuildId: "xyz123"}, nil
		},
	}, map[string]any{moduleFlagPath: manifest, generalFlagVersion: "1.2.3"}, "token")
	path, err := ac.moduleBuildStartAction(cCtx, parseStructFromCtx[moduleBuildStartArgs](cCtx))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, path, test.ShouldEqual, "xyz123")
	test.That(t, out.messages, test.ShouldHaveLength, 1)
	test.That(t, out.messages[0], test.ShouldEqual, "xyz123\n")
	test.That(t, errOut.messages, test.ShouldHaveLength, 1)

	// Modify manifest to set url to empty string
	createTestManifest(t, manifest, map[string]any{"url": ""})
	out.messages = nil
	errOut.messages = nil

	path, err = ac.moduleBuildStartAction(cCtx, parseStructFromCtx[moduleBuildStartArgs](cCtx))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "meta.json must have a url field set in order to start a cloud build")
	test.That(t, path, test.ShouldBeEmpty)
	test.That(t, out.messages, test.ShouldHaveLength, 0)
	test.That(t, errOut.messages, test.ShouldHaveLength, 0)
}

func TestListBuild(t *testing.T) {
	manifest := filepath.Join(t.TempDir(), "meta.json")
	createTestManifest(t, manifest, nil)
	cCtx, ac, out, errOut := setup(&inject.AppServiceClient{}, nil, &inject.BuildServiceClient{
		ListJobsFunc: func(ctx context.Context, in *v1.ListJobsRequest, opts ...grpc.CallOption) (*v1.ListJobsResponse, error) {
			return &v1.ListJobsResponse{Jobs: []*v1.JobInfo{
				{
					BuildId:  "xyz123",
					Platform: "linux/amd64",
					Version:  "1.2.3",
					Status:   v1.JobStatus_JOB_STATUS_DONE,
					EndTime:  timestamppb.New(time.Unix(0, 0)), // Jan 1 1970
				},
			}}, nil
		},
	}, map[string]any{moduleFlagPath: manifest}, "token")
	err := ac.moduleBuildListAction(cCtx, parseStructFromCtx[moduleBuildListArgs](cCtx))
	test.That(t, err, test.ShouldBeNil)
	joinedOutput := strings.Join(out.messages, "")
	test.That(t, joinedOutput, test.ShouldEqual, `ID     PLATFORM    STATUS VERSION TIME
xyz123 linux/amd64 Done   1.2.3   1970-01-01T00:00:00Z
`)
	test.That(t, errOut.messages, test.ShouldHaveLength, 0)
}

func TestBuildError(t *testing.T) {
	err := buildError(map[string]jobStatus{"ok": jobStatusDone})
	test.That(t, err, test.ShouldBeNil)
	err = buildError(map[string]jobStatus{"bad": jobStatusFailed})
	test.That(t, err.Error(), test.ShouldEqual, "some platforms failed to build: bad")
	err = buildError(map[string]jobStatus{"ok": jobStatusDone, "bad": jobStatusFailed})
	test.That(t, err.Error(), test.ShouldEqual, "some platforms failed to build: bad")
}

func TestModuleBuildWait(t *testing.T) {
	// this creates a race condiition if there are multiple tests testing the moduleBuildPollingInterval
	originalPollingInterval := moduleBuildPollingInterval
	moduleBuildPollingInterval = 200 * time.Millisecond
	defer func() { moduleBuildPollingInterval = originalPollingInterval }()
	startTime := time.Now()
	//nolint:dogsled
	_, ac, _, _ := setup(&inject.AppServiceClient{}, nil, &inject.BuildServiceClient{
		ListJobsFunc: func(ctx context.Context, in *v1.ListJobsRequest, opts ...grpc.CallOption) (*v1.ListJobsResponse, error) {
			// this will only report DONE after 2.5 polling intervals
			status := v1.JobStatus_JOB_STATUS_DONE
			if time.Since(startTime).Seconds() < moduleBuildPollingInterval.Seconds()*2.5 {
				status = v1.JobStatus_JOB_STATUS_IN_PROGRESS
			}
			return &v1.ListJobsResponse{Jobs: []*v1.JobInfo{
				{
					BuildId:  "xyz123",
					Platform: "linux/amd64",
					Version:  "1.2.3",
					Status:   status,
				},
			}}, nil
		},
	}, map[string]any{}, "token")
	startWaitTime := time.Now()
	statuses, err := ac.waitForBuildToFinish("xyz123", "", nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, statuses, test.ShouldResemble, map[string]jobStatus{"linux/amd64": "Done"})
	// ensure that we had to wait for at least 2, but no more than 5 polling intervals
	test.That(t,
		time.Since(startWaitTime).Seconds(),
		test.ShouldBeBetween,
		2*moduleBuildPollingInterval.Seconds(),
		5*moduleBuildPollingInterval.Seconds())
}

func TestModuleGetPlatformsForModule(t *testing.T) {
	//nolint:dogsled
	_, ac, _, _ := setup(&inject.AppServiceClient{}, nil, &inject.BuildServiceClient{
		ListJobsFunc: func(ctx context.Context, in *v1.ListJobsRequest, opts ...grpc.CallOption) (*v1.ListJobsResponse, error) {
			return &v1.ListJobsResponse{Jobs: []*v1.JobInfo{
				{
					BuildId:  "xyz123",
					Platform: "linux/amd64",
					Version:  "1.2.3",
					Status:   v1.JobStatus_JOB_STATUS_DONE,
				},

				{
					BuildId:  "xyz123",
					Platform: "linux/arm64",
					Version:  "1.2.3",
					Status:   v1.JobStatus_JOB_STATUS_DONE,
				},
			}}, nil
		},
	}, map[string]any{}, "token")
	platforms, err := ac.getPlatformsForModuleBuild("xyz123")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, platforms, test.ShouldResemble, []string{"linux/amd64", "linux/arm64"})
}

// testChdir is os.Chdir scoped to a test.
// Necessary because Getwd() fails if run on a deleted path.
func testChdir(t *testing.T, dest string) {
	t.Helper()
	orig, err := os.Getwd()
	test.That(t, err, test.ShouldBeNil)
	os.Chdir(dest)
	t.Cleanup(func() { os.Chdir(orig) })
}

func TestLocalBuild(t *testing.T) {
	testDir := t.TempDir()
	testChdir(t, testDir)

	// write manifest and setup.sh
	// the manifest contains a:
	// "setup": "./setup.sh"
	// and a "build": "make build"
	manifestPath := createTestManifest(t, "", nil)
	err := os.WriteFile(
		filepath.Join(testDir, "setup.sh"),
		[]byte("echo setup step msg"),
		0o700,
	)
	test.That(t, err, test.ShouldBeNil)

	err = os.WriteFile(
		filepath.Join(testDir, "Makefile"),
		[]byte("make build:\n\techo build step msg"),
		0o700,
	)
	test.That(t, err, test.ShouldBeNil)

	// run the build local action
	cCtx, _, out, errOut := setup(&inject.AppServiceClient{}, nil, &inject.BuildServiceClient{},
		map[string]any{moduleFlagPath: manifestPath, generalFlagVersion: "1.2.3"}, "token")
	manifest, err := loadManifest(manifestPath)
	test.That(t, err, test.ShouldBeNil)
	err = moduleBuildLocalAction(cCtx, &manifest, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, errOut.messages, test.ShouldHaveLength, 0)

	outMsg := strings.Join(out.messages, "")
	test.That(t, outMsg, test.ShouldContainSubstring, "setup step msg")
	test.That(t, outMsg, test.ShouldContainSubstring, "build step msg")
}

func TestIsReloadVersion(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		expected bool
	}{
		{
			name:     "reload version with part ID",
			version:  "reload-abc123",
			expected: true,
		},
		{
			name:     "reload version simple",
			version:  "reload",
			expected: true,
		},
		{
			name:     "reload-source version",
			version:  "reload-source-abc123",
			expected: true,
		},
		{
			name:     "normal semver version",
			version:  "1.2.3",
			expected: false,
		},
		{
			name:     "latest version",
			version:  "latest",
			expected: false,
		},
		{
			name:     "empty version",
			version:  "",
			expected: false,
		},
		{
			name:     "version containing reload but not prefix",
			version:  "v1.0.0-reload",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsReloadVersion(tt.version)
			test.That(t, result, test.ShouldEqual, tt.expected)
		})
	}
}

func TestGetOrgIDForPart(t *testing.T) {
	t.Run("returns primary org ID", func(t *testing.T) {
		expectedOrgID := "primary-org-123"
		secondaryOrgID := "secondary-org-456"
		robotID := "robot-abc"
		locationID := "location-xyz"

		mockClient := &inject.AppServiceClient{
			GetRobotFunc: func(ctx context.Context, req *apppb.GetRobotRequest,
				opts ...grpc.CallOption,
			) (*apppb.GetRobotResponse, error) {
				return &apppb.GetRobotResponse{
					Robot: &apppb.Robot{
						Id:       robotID,
						Location: locationID,
					},
				}, nil
			},
			GetLocationFunc: func(ctx context.Context, req *apppb.GetLocationRequest,
				opts ...grpc.CallOption,
			) (*apppb.GetLocationResponse, error) {
				test.That(t, req.LocationId, test.ShouldEqual, locationID)
				return &apppb.GetLocationResponse{
					Location: &apppb.Location{
						Id: locationID,
						Organizations: []*apppb.LocationOrganization{
							{OrganizationId: secondaryOrgID, Primary: false},
							{OrganizationId: expectedOrgID, Primary: true},
						},
						PrimaryOrgIdentity: &apppb.OrganizationIdentity{
							Id: expectedOrgID,
						},
					},
				}, nil
			},
		}

		_, vc, _, _ := setup(mockClient, nil, &inject.BuildServiceClient{}, map[string]any{}, "token")

		part := &apppb.RobotPart{
			Robot: robotID,
		}
		orgID, err := vc.getOrgIDForPart(part)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, orgID, test.ShouldEqual, expectedOrgID)
	})

	t.Run("falls back to first org when no primary", func(t *testing.T) {
		firstOrgID := "first-org-123"
		secondOrgID := "second-org-456"
		robotID := "robot-abc"
		locationID := "location-xyz"

		mockClient := &inject.AppServiceClient{
			GetRobotFunc: func(ctx context.Context, req *apppb.GetRobotRequest,
				opts ...grpc.CallOption,
			) (*apppb.GetRobotResponse, error) {
				return &apppb.GetRobotResponse{
					Robot: &apppb.Robot{
						Id:       robotID,
						Location: locationID,
					},
				}, nil
			},
			GetLocationFunc: func(ctx context.Context, req *apppb.GetLocationRequest,
				opts ...grpc.CallOption,
			) (*apppb.GetLocationResponse, error) {
				return &apppb.GetLocationResponse{
					Location: &apppb.Location{
						Id: locationID,
						Organizations: []*apppb.LocationOrganization{
							{OrganizationId: firstOrgID, Primary: false},
							{OrganizationId: secondOrgID, Primary: false},
						},
						PrimaryOrgIdentity: nil,
					},
				}, nil
			},
		}

		_, vc, _, _ := setup(mockClient, nil, &inject.BuildServiceClient{}, map[string]any{}, "token")

		part := &apppb.RobotPart{
			Robot: robotID,
		}
		orgID, err := vc.getOrgIDForPart(part)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, orgID, test.ShouldEqual, firstOrgID)
	})

	t.Run("returns error when GetRobot fails", func(t *testing.T) {
		mockClient := &inject.AppServiceClient{
			GetRobotFunc: func(ctx context.Context, req *apppb.GetRobotRequest,
				opts ...grpc.CallOption,
			) (*apppb.GetRobotResponse, error) {
				return nil, errors.New("robot not found")
			},
		}

		_, vc, _, _ := setup(mockClient, nil, &inject.BuildServiceClient{}, map[string]any{}, "token")

		part := &apppb.RobotPart{
			Robot: "robot-abc",
		}
		_, err := vc.getOrgIDForPart(part)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "robot not found")
	})

	t.Run("returns error when GetLocation fails", func(t *testing.T) {
		robotID := "robot-abc"
		locationID := "location-xyz"

		mockClient := &inject.AppServiceClient{
			GetRobotFunc: func(ctx context.Context, req *apppb.GetRobotRequest,
				opts ...grpc.CallOption,
			) (*apppb.GetRobotResponse, error) {
				return &apppb.GetRobotResponse{
					Robot: &apppb.Robot{
						Id:       robotID,
						Location: locationID,
					},
				}, nil
			},
			GetLocationFunc: func(ctx context.Context, req *apppb.GetLocationRequest,
				opts ...grpc.CallOption,
			) (*apppb.GetLocationResponse, error) {
				return nil, errors.New("location not found")
			},
		}

		_, vc, _, _ := setup(mockClient, nil, &inject.BuildServiceClient{}, map[string]any{}, "token")

		part := &apppb.RobotPart{
			Robot: robotID,
		}
		_, err := vc.getOrgIDForPart(part)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "location not found")
	})
}
