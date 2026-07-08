package cli

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	v1 "go.viam.com/api/app/build/v1"
	packagespb "go.viam.com/api/app/packages/v1"
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
		"url":         "https://github.com/test-org/test-repo",
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

func TestValidateRefExists(t *testing.T) {
	origGitHub := githubRefExists
	t.Cleanup(func() { githubRefExists = origGitHub })

	viamClient := &viamClient{}
	cmd := newTestContext(t, map[string]any{})

	// stub: ref exists on github
	refFound := func(ctx context.Context, owner, repo, ref, token string) (bool, error) {
		return true, nil
	}
	// stub: ref does not exist on github
	refNotFound := func(ctx context.Context, owner, repo, ref, token string) (bool, error) {
		return false, nil
	}
	// stub: should have failed to parse URL
	shouldNotBeCalled := func(ctx context.Context, owner, repo, ref, token string) (bool, error) {
		t.Fatal("githubRefExists should not have been called")
		return false, nil
	}

	cases := []struct {
		name          string
		stub          func(context.Context, string, string, string, string) (bool, error)
		url           string
		ref           string
		wantErrSubstr string
	}{
		{"github ref found", refFound, "https://github.com/allisonschiang/filtered-audio-fix", "reftest", ""},
		{"github ref not found", refNotFound, "https://github.com/allisonschiang/filtered-audio-fix", "typo", "not found"},
		{"malformed github url", shouldNotBeCalled, "https://github.com/viamrobotics", "main", "missing the repo path"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			githubRefExists = tc.stub
			err := viamClient.validateRefExists(context.Background(), cmd, tc.url, tc.ref, "")
			if tc.wantErrSubstr == "" {
				test.That(t, err, test.ShouldBeNil)
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.wantErrSubstr)
			}
		})
	}
}

func TestValidateWindowsCloudBuild(t *testing.T) {
	origFetch := githubFetchManifest
	t.Cleanup(func() { githubFetchManifest = origFetch })

	viamClient := &viamClient{}
	cmd := newTestContext(t, map[string]any{})

	// remote manifest stubs
	withModels := func(ctx context.Context, owner, repo, ref, manifestPath, token string) (ModuleManifest, error) {
		return ModuleManifest{Models: []ModuleComponent{{API: "a:b:c", Model: "a:b:c"}}}, nil
	}
	noModels := func(ctx context.Context, owner, repo, ref, manifestPath, token string) (ModuleManifest, error) {
		return ModuleManifest{Models: nil}, nil
	}
	fetchFailed := func(ctx context.Context, owner, repo, ref, manifestPath, token string) (ModuleManifest, error) {
		return ModuleManifest{}, errors.New("network down")
	}
	fetchUncalled := func(ctx context.Context, owner, repo, ref, manifestPath, token string) (ModuleManifest, error) {
		t.Fatal("githubFetchManifest should not have been called")
		return ModuleManifest{}, nil
	}

	// language is read from the local manifest entrypoint: "bin/" is Go, "dist/" is Python
	goEntrypoint := "bin/module"
	pythonEntrypoint := "dist/main"
	win := []string{"windows/amd64"}
	mixed := []string{"windows/amd64", "linux/amd64"}
	repo := "https://github.com/test-org/test-repo"
	gitlab := "https://gitlab.com/test-org/test-repo"
	cases := []struct {
		name          string
		entrypoint    string
		fetch         func(context.Context, string, string, string, string, string) (ModuleManifest, error)
		url           string
		platforms     []string
		wantErrSubstr string
	}{
		{"windows go module with models", goEntrypoint, withModels, repo, win, ""},
		{"windows go module, empty models", goEntrypoint, noModels, repo, win, "models must be populated"},
		{"windows go module, manifest fetch fails -> proceed", goEntrypoint, fetchFailed, repo, win, ""},
		{"windows python module fails fast", pythonEntrypoint, fetchUncalled, repo, win, "not supported for Windows Python"},
		{"mixed go, empty models -> warn and proceed", goEntrypoint, noModels, repo, mixed, ""},
		{"mixed python still hard fails", pythonEntrypoint, fetchUncalled, repo, mixed, "not supported for Windows Python"},
		{"non-windows build skips check", goEntrypoint, fetchUncalled, repo, []string{"linux/amd64"}, ""},
		{"non-github host, go module proceeds", goEntrypoint, fetchUncalled, gitlab, win, ""},
		{"non-github host, python still blocks", pythonEntrypoint, fetchUncalled, gitlab, win, "not supported for Windows Python"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			githubFetchManifest = tc.fetch
			manifest := &ModuleManifest{}
			manifest.Entrypoint = tc.entrypoint
			err := viamClient.validateWindowsCloudBuild(context.Background(), cmd, manifest, tc.url, "main", "", "", tc.platforms)
			if tc.wantErrSubstr == "" {
				test.That(t, err, test.ShouldBeNil)
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.wantErrSubstr)
			}
		})
	}
}

func TestStartBuild(t *testing.T) {
	// stub the ref validator so the test doesn't hit the network against
	// the placeholder url in the test manifest
	origGitHub := githubRefExists
	githubRefExists = func(ctx context.Context, owner, repo, ref, token string) (bool, error) {
		return true, nil
	}
	t.Cleanup(func() { githubRefExists = origGitHub })

	manifest := filepath.Join(t.TempDir(), "meta.json")
	createTestManifest(t, manifest, nil)
	cCtx, ac, out, errOut := setup(&inject.AppServiceClient{}, nil, &inject.BuildServiceClient{
		StartBuildFunc: func(ctx context.Context, in *v1.StartBuildRequest, opts ...grpc.CallOption) (*v1.StartBuildResponse, error) {
			return &v1.StartBuildResponse{BuildId: "xyz123"}, nil
		},
	}, map[string]any{moduleFlagPath: manifest, generalFlagVersion: "1.2.3"}, "token")
	path, err := ac.moduleBuildStartAction(context.Background(), cCtx, parseStructFromCtx[moduleBuildStartArgs](cCtx))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, path, test.ShouldEqual, "xyz123")
	test.That(t, out.messages, test.ShouldHaveLength, 1)
	test.That(t, out.messages[0], test.ShouldEqual, "xyz123\n")
	test.That(t, errOut.messages, test.ShouldHaveLength, 2)
	test.That(t, errOut.messages[0], test.ShouldEqual, "Build started, follow the logs with:\n")
	test.That(t, errOut.messages[1], test.ShouldEqual, "\tviam module build logs --id xyz123\n")

	// Modify manifest to set url to empty string
	createTestManifest(t, manifest, map[string]any{"url": ""})
	out.messages = nil
	errOut.messages = nil

	path, err = ac.moduleBuildStartAction(context.Background(), cCtx, parseStructFromCtx[moduleBuildStartArgs](cCtx))
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "meta.json must have a url field set in order to start a cloud build")
	test.That(t, path, test.ShouldBeEmpty)
	test.That(t, out.messages, test.ShouldHaveLength, 0)
	test.That(t, errOut.messages, test.ShouldHaveLength, 0)
}

// fakeSourceUploadBuildStream is an in-memory test double for the
// buildpb.BuildService_StartSourceUploadBuildClient streaming RPC. It records
// every Send call so tests can assert on the request shape, and returns a
// configurable CloseAndRecv response.
type fakeSourceUploadBuildStream struct {
	v1.BuildService_StartSourceUploadBuildClient // embedded for unused methods
	sends                                        []*v1.StartSourceUploadBuildRequest
	resp                                         *v1.StartSourceUploadBuildResponse
}

func (s *fakeSourceUploadBuildStream) Send(req *v1.StartSourceUploadBuildRequest) error {
	s.sends = append(s.sends, req)
	return nil
}

func (s *fakeSourceUploadBuildStream) CloseSend() error { return nil }

func (s *fakeSourceUploadBuildStream) CloseAndRecv() (*v1.StartSourceUploadBuildResponse, error) {
	return s.resp, nil
}

func TestModuleBuildStartFromSource(t *testing.T) {
	// Lay out a source directory with a meta.json and a small "source" file so
	// createGitArchive has something non-trivial to package up.
	sourceDir := t.TempDir()
	manifestPath := filepath.Join(sourceDir, "meta.json")
	createTestManifest(t, manifestPath, map[string]any{
		"build": map[string]any{
			"setup":  "./setup.sh",
			"build":  "make build",
			"path":   "module",
			"arch":   []any{"linux/amd64", "linux/arm64"},
			"distro": "bookworm",
		},
	})
	err := os.WriteFile(filepath.Join(sourceDir, "hello.go"), []byte("package main\n"), 0o600)
	test.That(t, err, test.ShouldBeNil)

	// Stub ListOrganizations so getOrgByModuleIDPrefix resolves the "test"
	// public namespace from the manifest's module_id ("test:test").
	asc := &inject.AppServiceClient{
		ListOrganizationsFunc: func(
			ctx context.Context, in *apppb.ListOrganizationsRequest, opts ...grpc.CallOption,
		) (*apppb.ListOrganizationsResponse, error) {
			return &apppb.ListOrganizationsResponse{Organizations: []*apppb.Organization{
				{Id: "test-org-id", PublicNamespace: "test"},
			}}, nil
		},
	}

	stream := &fakeSourceUploadBuildStream{
		resp: &v1.StartSourceUploadBuildResponse{BuildId: "build-xyz"},
	}
	bsc := &inject.BuildServiceClient{
		StartSourceUploadBuildFunc: func(
			ctx context.Context, opts ...grpc.CallOption,
		) (v1.BuildService_StartSourceUploadBuildClient, error) {
			return stream, nil
		},
	}

	cCtx, ac, out, errOut := setup(asc, nil, bsc, map[string]any{
		moduleFlagPath:            manifestPath,
		generalFlagVersion:        "v1.2.3", // leading "v" should be stripped
		moduleBuildFlagFromSource: true,
		generalFlagPath:           sourceDir,
		generalFlagNoProgress:     true,
		moduleBuildFlagWorkdir:    ".",
	}, "token")

	buildID, err := ac.moduleBuildStartAction(
		context.Background(), cCtx, parseStructFromCtx[moduleBuildStartArgs](cCtx),
	)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, buildID, test.ShouldEqual, "build-xyz")

	// At minimum we expect: 1 BuildInfo header + 1 Package_Info header + at
	// least 1 chunk send (hello.go + meta.json should fit in a single chunk).
	test.That(t, len(stream.sends), test.ShouldBeGreaterThanOrEqualTo, 3)

	// First send: BuildInfo with platforms / workdir / module_id / distro plus
	// the top-level ModuleVersion (semver, no leading "v").
	first := stream.sends[0]
	test.That(t, first.GetBuildInfo(), test.ShouldNotBeNil)
	test.That(t, first.GetBuildInfo().GetPlatforms(), test.ShouldResemble, []string{"linux/amd64", "linux/arm64"})
	test.That(t, first.GetBuildInfo().GetModuleId(), test.ShouldEqual, "test:test")
	test.That(t, first.GetBuildInfo().GetWorkdir(), test.ShouldEqual, ".")
	test.That(t, first.GetBuildInfo().GetDistro(), test.ShouldEqual, "bookworm")
	test.That(t, first.GetModuleVersion(), test.ShouldEqual, "1.2.3")

	// Second send: Package_Info with org/name/version/type. The version on the
	// package mirrors the top-level ModuleVersion.
	second := stream.sends[1]
	test.That(t, second.GetPackage(), test.ShouldNotBeNil)
	info := second.GetPackage().GetInfo()
	test.That(t, info, test.ShouldNotBeNil)
	test.That(t, info.GetOrganizationId(), test.ShouldEqual, "test-org-id")
	test.That(t, info.GetName(), test.ShouldEqual, "test")
	test.That(t, info.GetVersion(), test.ShouldEqual, "1.2.3")
	test.That(t, info.GetType(), test.ShouldEqual, packagespb.PackageType_PACKAGE_TYPE_MODULE)

	// Remaining sends are chunked tarball contents.
	for _, req := range stream.sends[2:] {
		pkg := req.GetPackage()
		test.That(t, pkg, test.ShouldNotBeNil)
		test.That(t, pkg.GetContents(), test.ShouldNotBeNil)
	}

	// Stdout: just the buildID (machine-readable). Stderr: human-readable
	// follow-up instructions matching `module build start`.
	test.That(t, out.messages, test.ShouldResemble, []string{"build-xyz\n"})
	test.That(t, errOut.messages, test.ShouldHaveLength, 2)
	test.That(t, errOut.messages[0], test.ShouldEqual, "Build started, follow the logs with:\n")
	test.That(t, errOut.messages[1], test.ShouldEqual, "\tviam module build logs --id build-xyz\n")
}

func TestTargetsWindowsPython(t *testing.T) {
	withPyDir := func(t *testing.T) string {
		t.Helper()
		dir := t.TempDir()
		test.That(t, os.MkdirAll(filepath.Join(dir, "src"), 0o700), test.ShouldBeNil)
		test.That(t,
			os.WriteFile(filepath.Join(dir, "src", "main.py"), []byte("pass\n"), 0o600),
			test.ShouldBeNil,
		)
		return filepath.Join(dir, "meta.json")
	}
	withoutPyDir := func(t *testing.T) string {
		t.Helper()
		return filepath.Join(t.TempDir(), "meta.json")
	}

	cases := []struct {
		name      string
		manifest  func(*testing.T) string
		platforms []string
		want      bool
	}{
		{"windows target + python module → block", withPyDir, []string{"windows/amd64"}, true},
		{"any-arch windows target + python module → block", withPyDir, []string{"windows"}, true},
		{"windows in mixed targets + python → block", withPyDir, []string{"linux/amd64", "windows/arm64"}, true},
		{"non-windows targets + python → allow", withPyDir, []string{"linux/amd64", "darwin/arm64"}, false},
		{"windows target without python module → allow", withoutPyDir, []string{"windows/amd64"}, false},
		{"non-windows + no python → allow", withoutPyDir, []string{"linux/amd64"}, false},
		{"linux-prefixed (not 'windows/' prefix) → allow", withPyDir, []string{"linux/any"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := targetsWindowsPython(tc.manifest(t), tc.platforms)
			test.That(t, got, test.ShouldEqual, tc.want)
		})
	}
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
	err := ac.moduleBuildListAction(context.Background(), cCtx, parseStructFromCtx[moduleBuildListArgs](cCtx))
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
	statuses, err := ac.waitForBuildToFinish(context.Background(), "xyz123", "", nil)
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
	platforms, err := ac.getPlatformsForModuleBuild(context.Background(), "xyz123")
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

	setupScriptCmd, setupFile, setupContent := "./setup.sh", "setup.sh", "echo setup step msg"
	buildCmd, buildFile, buildContent := "make build", "Makefile", "make build:\n\techo build step msg"
	if runtime.GOOS == "windows" {
		setupScriptCmd, setupFile = "setup.bat", "setup.bat"
		buildCmd, buildFile, buildContent = "build.bat", "build.bat", "echo build step msg"
	}

	manifestPath := createTestManifest(t, "", map[string]any{
		"build": map[string]any{
			"setup": setupScriptCmd,
			"build": buildCmd,
			"path":  "module",
			"arch":  []any{"linux/amd64"},
		},
	})
	err := os.WriteFile(filepath.Join(testDir, setupFile), []byte(setupContent), 0o700)
	test.That(t, err, test.ShouldBeNil)

	err = os.WriteFile(filepath.Join(testDir, buildFile), []byte(buildContent), 0o700)
	test.That(t, err, test.ShouldBeNil)

	// run the build local action
	cCtx, _, out, errOut := setup(&inject.AppServiceClient{}, nil, &inject.BuildServiceClient{},
		map[string]any{moduleFlagPath: manifestPath, generalFlagVersion: "1.2.3"}, "token")
	manifest, err := loadManifest(manifestPath)
	test.That(t, err, test.ShouldBeNil)
	err = moduleBuildLocalAction(context.Background(), cCtx, &manifest, nil)
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
		orgID, err := vc.getOrgIDForPart(context.Background(), part)
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
		_, err := vc.getOrgIDForPart(context.Background(), part)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no primary org id found for location")
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
		_, err := vc.getOrgIDForPart(context.Background(), part)
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
		_, err := vc.getOrgIDForPart(context.Background(), part)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "location not found")
	})
}

// archiveFiles returns the sorted list of file names inside a .tar.gz archive.
func archiveFiles(t *testing.T, archivePath string) []string {
	t.Helper()
	f, err := os.Open(archivePath)
	test.That(t, err, test.ShouldBeNil)
	defer f.Close()
	gr, err := gzip.NewReader(f)
	test.That(t, err, test.ShouldBeNil)
	defer gr.Close()
	tr := tar.NewReader(gr)

	var names []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		test.That(t, err, test.ShouldBeNil)
		names = append(names, hdr.Name)
	}
	sort.Strings(names)
	return names
}

// writeFile is a test helper that creates parent dirs and writes content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	test.That(t, os.MkdirAll(filepath.Dir(path), 0o755), test.ShouldBeNil)
	test.That(t, os.WriteFile(path, []byte(content), 0o644), test.ShouldBeNil)
}

func TestCreateGitArchive(t *testing.T) {
	newClient := func() *viamClient {
		_, vc, _, _ := setup(&inject.AppServiceClient{}, nil, &inject.BuildServiceClient{}, map[string]any{}, "token")
		return vc
	}

	t.Run("basic inclusion", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "main.go"), "package main")
		writeFile(t, filepath.Join(root, "lib", "util.go"), "package lib")

		vc := newClient()
		archivePath, err := vc.createGitArchive(root)
		test.That(t, err, test.ShouldBeNil)
		t.Cleanup(func() { os.Remove(archivePath) })

		files := archiveFiles(t, archivePath)
		test.That(t, files, test.ShouldResemble, []string{"lib/util.go", "main.go"})
	})

	t.Run("root gitignore excludes files", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, ".gitignore"), "*.log\nbuild/\n")
		writeFile(t, filepath.Join(root, "main.go"), "package main")
		writeFile(t, filepath.Join(root, "debug.log"), "log data")
		writeFile(t, filepath.Join(root, "build", "output.bin"), "binary")

		vc := newClient()
		archivePath, err := vc.createGitArchive(root)
		test.That(t, err, test.ShouldBeNil)
		t.Cleanup(func() { os.Remove(archivePath) })

		files := archiveFiles(t, archivePath)
		test.That(t, files, test.ShouldResemble, []string{".gitignore", "main.go"})
	})

	t.Run("nested gitignore excludes files", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "main.go"), "package main")
		writeFile(t, filepath.Join(root, "web", ".gitignore"), "node_modules\ndist\n")
		writeFile(t, filepath.Join(root, "web", "index.html"), "<html>")
		writeFile(t, filepath.Join(root, "web", "node_modules", "pkg", "index.js"), "module.exports = {}")
		writeFile(t, filepath.Join(root, "web", "dist", "bundle.js"), "bundled")

		vc := newClient()
		archivePath, err := vc.createGitArchive(root)
		test.That(t, err, test.ShouldBeNil)
		t.Cleanup(func() { os.Remove(archivePath) })

		files := archiveFiles(t, archivePath)
		test.That(t, files, test.ShouldResemble, []string{
			"main.go",
			"web/.gitignore",
			"web/index.html",
		})
	})

	t.Run("default ignores are applied", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "main.go"), "package main")
		writeFile(t, filepath.Join(root, ".DS_Store"), "junk")
		writeFile(t, filepath.Join(root, "Thumbs.db"), "junk")

		// .git directory should be excluded
		writeFile(t, filepath.Join(root, ".git", "config"), "[core]")

		vc := newClient()
		archivePath, err := vc.createGitArchive(root)
		test.That(t, err, test.ShouldBeNil)
		t.Cleanup(func() { os.Remove(archivePath) })

		files := archiveFiles(t, archivePath)
		test.That(t, files, test.ShouldResemble, []string{"main.go"})
	})

	t.Run("negation pattern re-includes file", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, ".gitignore"), "*.log\n!important.log\n")
		writeFile(t, filepath.Join(root, "debug.log"), "debug")
		writeFile(t, filepath.Join(root, "important.log"), "keep this")
		writeFile(t, filepath.Join(root, "main.go"), "package main")

		vc := newClient()
		archivePath, err := vc.createGitArchive(root)
		test.That(t, err, test.ShouldBeNil)
		t.Cleanup(func() { os.Remove(archivePath) })

		files := archiveFiles(t, archivePath)
		test.That(t, files, test.ShouldResemble, []string{".gitignore", "important.log", "main.go"})
	})

	t.Run("no gitignore includes everything", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "a.txt"), "a")
		writeFile(t, filepath.Join(root, "sub", "b.txt"), "b")

		vc := newClient()
		archivePath, err := vc.createGitArchive(root)
		test.That(t, err, test.ShouldBeNil)
		t.Cleanup(func() { os.Remove(archivePath) })

		files := archiveFiles(t, archivePath)
		test.That(t, files, test.ShouldResemble, []string{"a.txt", "sub/b.txt"})
	})

	t.Run("viamboat-style nested gitignore", func(t *testing.T) {
		root := t.TempDir()

		// Root .gitignore (like viamboat: no node_modules rule)
		writeFile(t, filepath.Join(root, ".gitignore"), "bin\nmodule.tar.gz\n")
		writeFile(t, filepath.Join(root, "main.go"), "package main")
		writeFile(t, filepath.Join(root, "bin", "module"), "binary")

		// Nested web project with its own .gitignore
		writeFile(t, filepath.Join(root, "display", "onehelm-web", ".gitignore"), "node_modules\ndist\n")
		writeFile(t, filepath.Join(root, "display", "onehelm-web", "index.html"), "<html>")
		writeFile(t, filepath.Join(root, "display", "onehelm-web", "package.json"), "{}")
		writeFile(t, filepath.Join(root, "display", "onehelm-web", "node_modules", ".bin", "acorn"), "#!/bin/sh")
		writeFile(t, filepath.Join(root, "display", "onehelm-web", "node_modules", "acorn", "index.js"), "module")
		writeFile(t, filepath.Join(root, "display", "onehelm-web", "dist", "bundle.js"), "bundled")

		vc := newClient()
		archivePath, err := vc.createGitArchive(root)
		test.That(t, err, test.ShouldBeNil)
		t.Cleanup(func() { os.Remove(archivePath) })

		files := archiveFiles(t, archivePath)
		test.That(t, files, test.ShouldResemble, []string{
			".gitignore",
			"display/onehelm-web/.gitignore",
			"display/onehelm-web/index.html",
			"display/onehelm-web/package.json",
			"main.go",
		})
	})

	t.Run("directory-only pattern with trailing slash", func(t *testing.T) {
		root := t.TempDir()
		// "logs/" should only match the directory, not a file named "logs"
		writeFile(t, filepath.Join(root, ".gitignore"), "logs/\n")
		writeFile(t, filepath.Join(root, "main.go"), "package main")
		writeFile(t, filepath.Join(root, "logs", "app.log"), "log data")

		vc := newClient()
		archivePath, err := vc.createGitArchive(root)
		test.That(t, err, test.ShouldBeNil)
		t.Cleanup(func() { os.Remove(archivePath) })

		files := archiveFiles(t, archivePath)
		test.That(t, files, test.ShouldResemble, []string{".gitignore", "main.go"})
	})
}
