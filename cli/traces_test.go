package cli

import (
	"context"
	"maps"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	apppb "go.viam.com/api/app/v1"
	"go.viam.com/test"
	"google.golang.org/grpc"

	"go.viam.com/rdk/logging"
	shelltestutils "go.viam.com/rdk/services/shell/testutils"
	"go.viam.com/rdk/testutils/inject"
)

func TestTraceGetRemote(t *testing.T) {
	logger := logging.NewTestLogger(t)

	listOrganizationsFunc := func(ctx context.Context, in *apppb.ListOrganizationsRequest,
		opts ...grpc.CallOption,
	) (*apppb.ListOrganizationsResponse, error) {
		orgs := []*apppb.Organization{{Name: "jedi", Id: uuid.NewString(), PublicNamespace: "anakin"}, {Name: "mandalorians"}}
		return &apppb.ListOrganizationsResponse{Organizations: orgs}, nil
	}
	listLocationsFunc := func(ctx context.Context, in *apppb.ListLocationsRequest,
		opts ...grpc.CallOption,
	) (*apppb.ListLocationsResponse, error) {
		locs := []*apppb.Location{{Name: "naboo"}}
		return &apppb.ListLocationsResponse{Locations: locs}, nil
	}
	listRobotsFunc := func(ctx context.Context, in *apppb.ListRobotsRequest,
		opts ...grpc.CallOption,
	) (*apppb.ListRobotsResponse, error) {
		robots := []*apppb.Robot{{Name: "r2d2"}}
		return &apppb.ListRobotsResponse{Robots: robots}, nil
	}

	partFqdn := uuid.NewString()
	partID := uuid.NewString()
	getRobotPartsFunc := func(ctx context.Context, in *apppb.GetRobotPartsRequest,
		opts ...grpc.CallOption,
	) (*apppb.GetRobotPartsResponse, error) {
		parts := []*apppb.RobotPart{{Name: "main", Fqdn: partFqdn, Id: partID}}
		return &apppb.GetRobotPartsResponse{Parts: parts}, nil
	}

	asc := &inject.AppServiceClient{
		ListOrganizationsFunc: listOrganizationsFunc,
		ListLocationsFunc:     listLocationsFunc,
		ListRobotsFunc:        listRobotsFunc,
		GetRobotPartsFunc:     getRobotPartsFunc,
	}

	basePartFlags := map[string]any{
		"organization": "jedi",
		"location":     "naboo",
		"robot":        "r2d2",
		"part":         "main",
	}

	tfs := shelltestutils.SetupTestFileSystem(t)

	t.Run("trace data does not exist", func(t *testing.T) {
		tempDir := t.TempDir()

		output := t.TempDir()
		testPartFlags := maps.Collect(maps.All(basePartFlags))
		testPartFlags["destination"] = output

		originalTracesPath := tracesPath
		tracesPath = filepath.Join(tfs.Root, "FAKEDIR")
		t.Cleanup(func() {
			tracesPath = originalTracesPath
		})

		cCtx, viamClient, _, _ := setupWithRunningPart(
			t, asc, nil, nil, testPartFlags, "token", partFqdn)
		test.That(t,
			viamClient.tracesGetRemoteAction(cCtx, parseStructFromCtx[traceGetRemoteArgs](cCtx), true, logger),
			test.ShouldNotBeNil)

		entries, err := os.ReadDir(tempDir)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, entries, test.ShouldHaveLength, 0)
	})

	t.Run("trace data exists", func(t *testing.T) {
		tmpPartTracePath := filepath.Join(tfs.Root, partID)
		err := os.Mkdir(tmpPartTracePath, 0o750)
		test.That(t, err, test.ShouldBeNil)
		t.Cleanup(func() {
			err = os.RemoveAll(tmpPartTracePath)
			test.That(t, err, test.ShouldBeNil)
		})
		err = os.WriteFile(filepath.Join(tmpPartTracePath, "traces"), nil, 0o640)
		test.That(t, err, test.ShouldBeNil)
		originalTracePath := tracesPath
		tracesPath = tfs.Root
		t.Cleanup(func() {
			tracesPath = originalTracePath
		})

		testDownload := func(t *testing.T, targetPath string) {
			testFlags := maps.Collect(maps.All(basePartFlags))
			testFlags["destination"] = targetPath

			cCtx, viamClient, _, _ := setupWithRunningPart(
				t, asc, nil, nil, testFlags, "token", partFqdn)
			test.That(t,
				viamClient.tracesGetRemoteAction(cCtx, parseStructFromCtx[traceGetRemoteArgs](cCtx), true, logger),
				test.ShouldBeNil)

			entries, err := os.ReadDir(targetPath)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, entries, test.ShouldHaveLength, 1)
			traceFile := entries[0]
			test.That(t, traceFile.Name(), test.ShouldEqual, "traces")
			test.That(t, traceFile.IsDir(), test.ShouldBeFalse)
		}

		t.Run("download to cwd", func(t *testing.T) {
			tempDir := t.TempDir()
			t.Chdir(tempDir)
			testDownload(t, ".")
		})
		t.Run("download to specified path", func(t *testing.T) {
			testDownload(t, t.TempDir())
		})
	})
}
