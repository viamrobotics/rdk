package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	v1 "go.viam.com/api/app/build/v1"
	"go.viam.com/test"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/testutils/inject"
)

func createTestManifest(t *testing.T, path string) string {
	t.Helper()
	if len(path) == 0 {
		path = filepath.Join(t.TempDir(), "meta.json")
	}
	fi, err := os.Create(path)
	test.That(t, err, test.ShouldBeNil)
	_, err = fi.WriteString(`{
  "module_id": "test:test",
  "visibility": "private",
  "url": "https://github.com/",
  "description": "a",
  "models": [
    {
      "api": "a:b:c",
      "model": "a:b:c"
    }
  ],
  "build": {
    "setup": "./setup.sh",
    "build": "make build",
    "path": "module",
    "arch": ["linux/amd64"]
  },
  "entrypoint": "bin/module"
}
`)
	test.That(t, err, test.ShouldBeNil)
	err = fi.Close()
	test.That(t, err, test.ShouldBeNil)
	return path
}

func TestStartBuild(t *testing.T) {
	manifest := filepath.Join(t.TempDir(), "meta.json")
	createTestManifest(t, manifest)
	cCtx, ac, out, errOut := setup(&inject.AppServiceClient{}, nil, &inject.BuildServiceClient{
		StartBuildFunc: func(ctx context.Context, in *v1.StartBuildRequest, opts ...grpc.CallOption) (*v1.StartBuildResponse, error) {
			return &v1.StartBuildResponse{BuildId: "xyz123"}, nil
		},
	}, map[string]any{moduleFlagPath: manifest, generalFlagVersion: "1.2.3"}, "token")
	err := ac.moduleBuildStartAction(cCtx, parseStructFromCtx[moduleBuildStartArgs](cCtx))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, out.messages, test.ShouldHaveLength, 1)
	test.That(t, out.messages[0], test.ShouldEqual, "xyz123\n")
	test.That(t, errOut.messages, test.ShouldHaveLength, 1)
}

func TestListBuild(t *testing.T) {
	manifest := filepath.Join(t.TempDir(), "meta.json")
	createTestManifest(t, manifest)
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
	statuses, err := ac.waitForBuildToFinish("xyz123", "")
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
	manifestPath := createTestManifest(t, "")
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
	err = moduleBuildLocalAction(cCtx, &manifest)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, errOut.messages, test.ShouldHaveLength, 0)

	outMsg := strings.Join(out.messages, "")
	test.That(t, outMsg, test.ShouldContainSubstring, "setup step msg")
	test.That(t, outMsg, test.ShouldContainSubstring, "build step msg")
}
