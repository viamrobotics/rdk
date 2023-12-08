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

func createTestManifest(t *testing.T, path string) {
	t.Helper()
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
    "setup": "",
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
}
}
func TestStartBuild(t *testing.T) {
	manifest := filepath.Join(t.TempDir(), "meta.json")
	createTestManifest(t, manifest)
	cCtx, ac, out, errOut := setup(&inject.AppServiceClient{}, nil, &inject.BuildServiceClient{
		StartBuildFunc: func(ctx context.Context, in *v1.StartBuildRequest, opts ...grpc.CallOption) (*v1.StartBuildResponse, error) {
			return &v1.StartBuildResponse{BuildId: "xyz123"}, nil
		},
	}, &map[string]string{moduleBuildFlagPath: manifest, moduleBuildFlagVersion: "1.2.3"}, "token")
	err := ac.moduleBuildStartAction(cCtx)
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
					BuildId:   "xyz123",
					Platform:  "linux/amd64",
					Version:   "1.2.3",
					Status:    v1.JobStatus_JOB_STATUS_DONE,

					EndTime:   timestamppb.New(time.Unix(0, 0)), // Jan 1 1970
				},
			}}, nil
		},
	}, &map[string]string{moduleBuildFlagPath: manifest}, "token")
	err := ac.moduleBuildListAction(cCtx)
	test.That(t, err, test.ShouldBeNil)
	joinedOutput := strings.Join(out.messages, "")
	test.That(t, joinedOutput, test.ShouldEqual, `ID     PLATFORM    STATUS VERSION TIME
xyz123 linux/amd64 Done   1.2.3   1970-01-01T00:00:00Z
`)
	test.That(t, errOut.messages, test.ShouldHaveLength, 0)
}
