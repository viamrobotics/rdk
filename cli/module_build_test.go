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
	"go.viam.com/test"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/logging"
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
		defaultManifest[key] = value
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

func TestRetryableCopyToPart(t *testing.T) {
	t.Run("SuccessOnFirstAttempt", func(t *testing.T) {
		cCtx, vc, _, errOut := setup(&inject.AppServiceClient{}, nil, &inject.BuildServiceClient{},
			map[string]any{}, "token")

		mockCopyFunc := func(fqdn string, debug, allowRecursion, preserve bool,
			paths []string, destination string, logger logging.Logger, noProgress bool,
		) error {
			return nil // Success immediately
		}

		allSteps := []*Step{
			{ID: "upload", Message: "Uploading package...", CompletedMsg: "Package uploaded", IndentLevel: 0},
		}
		pm := NewProgressManager(allSteps, WithProgressOutput(false))
		defer pm.Stop()

		err := pm.Start("upload")
		test.That(t, err, test.ShouldBeNil)

		logger := logging.NewTestLogger(t)
		err = vc.retryableCopyToPart(
			cCtx,
			"test-fqdn",
			false,
			[]string{"/path/to/file"},
			"/dest/path",
			logger,
			"test-part-123",
			pm,
			mockCopyFunc,
		)

		test.That(t, err, test.ShouldBeNil)
		test.That(t, errOut.messages, test.ShouldHaveLength, 0)

		// Verify no retry steps were created
		retryStepFound := false
		for _, step := range pm.steps {
			if strings.Contains(step.ID, "upload-attempt-") {
				retryStepFound = true
				break
			}
		}
		test.That(t, retryStepFound, test.ShouldBeFalse)
	})

	t.Run("SuccessAfter2Retries", func(t *testing.T) {
		cCtx, vc, _, errOut := setup(&inject.AppServiceClient{}, nil, &inject.BuildServiceClient{},
			map[string]any{}, "token")

		attemptCount := 0
		mockCopyFunc := func(fqdn string, debug, allowRecursion, preserve bool,
			paths []string, destination string, logger logging.Logger, noProgress bool,
		) error {
			attemptCount++
			if attemptCount <= 2 {
				return errors.New("copy failed")
			}
			return nil // Success on 3rd attempt
		}

		allSteps := []*Step{
			{ID: "upload", Message: "Uploading package...", CompletedMsg: "Package uploaded", IndentLevel: 0},
		}
		pm := NewProgressManager(allSteps, WithProgressOutput(false))
		defer pm.Stop()

		err := pm.Start("upload")
		test.That(t, err, test.ShouldBeNil)

		logger := logging.NewTestLogger(t)
		err = vc.retryableCopyToPart(
			cCtx,
			"test-fqdn",
			false,
			[]string{"/path/to/file"},
			"/dest/path",
			logger,
			"test-part-123",
			pm,
			mockCopyFunc,
		)

		test.That(t, err, test.ShouldBeNil)
		test.That(t, attemptCount, test.ShouldEqual, 3)

		// Verify retry steps were created (attempt 1, 2, and 3)
		retryStepCount := 0
		for _, step := range pm.steps {
			if strings.Contains(step.ID, "upload-attempt-") {
				retryStepCount++
				// Verify IndentLevel is 2 for deeper nesting
				test.That(t, step.IndentLevel, test.ShouldEqual, 2)
			}
		}
		test.That(t, retryStepCount, test.ShouldEqual, 3) // attempt-1, attempt-2, and attempt-3

		// Verify no duplicate warning messages in errOut (only permission denied warnings should appear)
		errMsg := strings.Join(errOut.messages, "")
		test.That(t, errMsg, test.ShouldNotContainSubstring, "Upload attempt 1/6 failed:")
		test.That(t, errMsg, test.ShouldNotContainSubstring, "Upload attempt 2/6 failed:")
	})

	t.Run("SuccessAfter5Retries", func(t *testing.T) {
		cCtx, vc, _, errOut := setup(&inject.AppServiceClient{}, nil, &inject.BuildServiceClient{},
			map[string]any{}, "token")

		attemptCount := 0
		mockCopyFunc := func(fqdn string, debug, allowRecursion, preserve bool,
			paths []string, destination string, logger logging.Logger, noProgress bool,
		) error {
			attemptCount++
			if attemptCount <= 5 {
				return errors.New("copy failed")
			}
			return nil // Success on 6th attempt
		}

		allSteps := []*Step{
			{ID: "upload", Message: "Uploading package...", CompletedMsg: "Package uploaded", IndentLevel: 0},
		}
		pm := NewProgressManager(allSteps, WithProgressOutput(false))
		defer pm.Stop()

		err := pm.Start("upload")
		test.That(t, err, test.ShouldBeNil)

		logger := logging.NewTestLogger(t)
		err = vc.retryableCopyToPart(
			cCtx,
			"test-fqdn",
			false,
			[]string{"/path/to/file"},
			"/dest/path",
			logger,
			"test-part-123",
			pm,
			mockCopyFunc,
		)

		test.That(t, err, test.ShouldBeNil)
		test.That(t, attemptCount, test.ShouldEqual, 6)

		// Verify all retry steps were created (attempt 1 through 6)
		retryStepCount := 0
		for _, step := range pm.steps {
			if strings.Contains(step.ID, "upload-attempt-") {
				retryStepCount++
				test.That(t, step.IndentLevel, test.ShouldEqual, 2)
			}
		}
		test.That(t, retryStepCount, test.ShouldEqual, 6) // attempt-1 through attempt-6

		// No duplicate warning messages should appear (only permission denied warnings)
		errMsg := strings.Join(errOut.messages, "")
		test.That(t, errMsg, test.ShouldNotContainSubstring, "Upload attempt")
	})

	t.Run("AllAttemptsFail", func(t *testing.T) {
		cCtx, vc, _, _ := setup(&inject.AppServiceClient{}, nil, &inject.BuildServiceClient{},
			map[string]any{}, "token")

		attemptCount := 0
		mockCopyFunc := func(fqdn string, debug, allowRecursion, preserve bool,
			paths []string, destination string, logger logging.Logger, noProgress bool,
		) error {
			attemptCount++
			return errors.New("persistent copy failure")
		}

		allSteps := []*Step{
			{ID: "upload", Message: "Uploading package...", CompletedMsg: "Package uploaded", IndentLevel: 0},
		}
		pm := NewProgressManager(allSteps, WithProgressOutput(false))
		defer pm.Stop()

		err := pm.Start("upload")
		test.That(t, err, test.ShouldBeNil)

		logger := logging.NewTestLogger(t)
		err = vc.retryableCopyToPart(
			cCtx,
			"test-fqdn",
			false,
			[]string{"/path/to/file"},
			"/dest/path",
			logger,
			"test-part-123",
			pm,
			mockCopyFunc,
		)

		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "all 6 upload attempts failed")
		test.That(t, err.Error(), test.ShouldContainSubstring, "viam module reload --no-build --part-id test-part-123")
		test.That(t, attemptCount, test.ShouldEqual, 6)
	})

	t.Run("PermissionDeniedError", func(t *testing.T) {
		cCtx, vc, _, errOut := setup(&inject.AppServiceClient{}, nil, &inject.BuildServiceClient{},
			map[string]any{}, "token")

		mockCopyFunc := func(fqdn string, debug, allowRecursion, preserve bool,
			paths []string, destination string, logger logging.Logger, noProgress bool,
		) error {
			return status.Error(codes.PermissionDenied, "permission denied")
		}

		allSteps := []*Step{
			{ID: "upload", Message: "Uploading package...", CompletedMsg: "Package uploaded", IndentLevel: 0},
		}
		pm := NewProgressManager(allSteps, WithProgressOutput(false))
		defer pm.Stop()

		err := pm.Start("upload")
		test.That(t, err, test.ShouldBeNil)

		logger := logging.NewTestLogger(t)
		err = vc.retryableCopyToPart(
			cCtx,
			"test-fqdn",
			false,
			[]string{"/path/to/file"},
			"/dest/path",
			logger,
			"test-part-123",
			pm,
			mockCopyFunc,
		)

		test.That(t, err, test.ShouldNotBeNil)

		// Verify permission denied specific warning appears
		errMsg := strings.Join(errOut.messages, "")
		test.That(t, errMsg, test.ShouldContainSubstring, "RDK couldn't write to the default file copy destination")
		test.That(t, errMsg, test.ShouldContainSubstring, "--home")
		test.That(t, errMsg, test.ShouldContainSubstring, "run the RDK as root")
	})
}
