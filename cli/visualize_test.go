package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	apppb "go.viam.com/api/app/v1"
	"go.viam.com/test"
	"google.golang.org/grpc"

	"go.viam.com/rdk/testutils/inject"
)

func TestBuildLaunchConfig(t *testing.T) {
	part := &apppb.RobotPart{Id: "part-id", Name: "main", Fqdn: "main.loc.viam.cloud"}
	expiry := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)

	t.Run("token", func(t *testing.T) {
		conf := &Config{Auth: &token{AccessToken: "tok", ExpiresAt: expiry}}
		cfg, err := buildLaunchConfig(part, "mach", conf)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, cfg.Version, test.ShouldEqual, 1)
		test.That(t, cfg.Host, test.ShouldEqual, "main.loc.viam.cloud")
		test.That(t, cfg.PartID, test.ShouldEqual, "part-id")
		test.That(t, cfg.PartName, test.ShouldEqual, "main")
		test.That(t, cfg.MachineName, test.ShouldEqual, "mach")
		test.That(t, cfg.Credentials.Type, test.ShouldEqual, "access-token")
		test.That(t, cfg.Credentials.Payload, test.ShouldEqual, "tok")
		test.That(t, cfg.Credentials.AuthEntity, test.ShouldEqual, "")
		test.That(t, cfg.ExpiresAt, test.ShouldEqual, "2030-01-02T03:04:05Z")
		test.That(t, cfg.SignalingAddress, test.ShouldEqual, defaultBaseURL)
		test.That(t, cfg.ServiceHost, test.ShouldEqual, "https://app.viam.com")
	})

	t.Run("api key", func(t *testing.T) {
		conf := &Config{Auth: &apiKey{KeyID: "kid", KeyCrypto: "secret"}}
		cfg, err := buildLaunchConfig(part, "mach", conf)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, cfg.Credentials.Type, test.ShouldEqual, "api-key")
		test.That(t, cfg.Credentials.AuthEntity, test.ShouldEqual, "kid")
		test.That(t, cfg.Credentials.Payload, test.ShouldEqual, "secret")
		test.That(t, cfg.ExpiresAt, test.ShouldEqual, "")
	})

	t.Run("staging base url", func(t *testing.T) {
		conf := &Config{BaseURL: "https://app.viam.dev:443", Auth: &apiKey{}}
		cfg, err := buildLaunchConfig(part, "mach", conf)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, cfg.SignalingAddress, test.ShouldEqual, "https://app.viam.dev:443")
		test.That(t, cfg.ServiceHost, test.ShouldEqual, "https://app.viam.dev")
	})

	t.Run("nil auth", func(t *testing.T) {
		_, err := buildLaunchConfig(part, "mach", &Config{})
		test.That(t, err, test.ShouldEqual, errLoggedOut)
	})
}

func TestResolveVisualizerBinary(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "viz")
	test.That(t, os.WriteFile(file, []byte("x"), 0o755), test.ShouldBeNil)
	otherFile := filepath.Join(dir, "other")
	test.That(t, os.WriteFile(otherFile, []byte("x"), 0o755), test.ShouldBeNil)
	bundle := filepath.Join(dir, "Viam Visualizer.app")
	test.That(t, os.Mkdir(bundle, 0o755), test.ShouldBeNil)

	t.Run("flag over env", func(t *testing.T) {
		got, err := resolveVisualizerBinary(file, otherFile, os.Stat)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, got, test.ShouldEqual, file)
	})
	t.Run("env alone", func(t *testing.T) {
		got, err := resolveVisualizerBinary("", otherFile, os.Stat)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, got, test.ShouldEqual, otherFile)
	})
	t.Run("neither", func(t *testing.T) {
		_, err := resolveVisualizerBinary("", "", os.Stat)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "--app-path")
		test.That(t, err.Error(), test.ShouldContainSubstring, "VIAM_VISUALIZER_PATH")
		test.That(t, err.Error(), test.ShouldContainSubstring, visualizerReleasesURL)
	})
	t.Run("app bundle", func(t *testing.T) {
		got, err := resolveVisualizerBinary(bundle, "", os.Stat)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, got, test.ShouldEqual, filepath.Join(bundle, "Contents", "MacOS", "viam-visualizer"))
	})
	t.Run("missing", func(t *testing.T) {
		missing := filepath.Join(dir, "nope")
		_, err := resolveVisualizerBinary(missing, "", os.Stat)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, missing)
	})
}

func TestWriteLaunchFile(t *testing.T) {
	dir := t.TempDir()
	cfg := launchConfig{
		Version:     launchFileVersion,
		PartID:      "part-id",
		Credentials: launchCredentials{Type: "api-key", AuthEntity: "kid", Payload: "secret"},
	}
	path, err := writeLaunchFile(dir, cfg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, filepath.Dir(path), test.ShouldEqual, dir)

	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, info.Mode().Perm(), test.ShouldEqual, os.FileMode(0o600))
	}

	data, err := os.ReadFile(path)
	test.That(t, err, test.ShouldBeNil)
	var raw map[string]any
	test.That(t, json.Unmarshal(data, &raw), test.ShouldBeNil)
	test.That(t, raw["version"], test.ShouldEqual, float64(1))
	var got launchConfig
	test.That(t, json.Unmarshal(data, &got), test.ShouldBeNil)
	test.That(t, got, test.ShouldResemble, cfg)
}

func TestMachinesPartVisualizeAction(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a shell script as the fake app")
	}
	const partID = "part-id"
	asc := &inject.AppServiceClient{
		ListOrganizationsFunc: func(ctx context.Context, in *apppb.ListOrganizationsRequest,
			opts ...grpc.CallOption,
		) (*apppb.ListOrganizationsResponse, error) {
			return nil, errors.New("not used in this test")
		},
		ListRobotsFunc: func(ctx context.Context, in *apppb.ListRobotsRequest,
			opts ...grpc.CallOption,
		) (*apppb.ListRobotsResponse, error) {
			return nil, errors.New("not used in this test")
		},
		GetRobotPartFunc: func(ctx context.Context, in *apppb.GetRobotPartRequest,
			opts ...grpc.CallOption,
		) (*apppb.GetRobotPartResponse, error) {
			return &apppb.GetRobotPartResponse{Part: &apppb.RobotPart{
				Id: partID, Name: "main-part", Robot: "robot-id", Fqdn: "main.loc.viam.cloud",
			}}, nil
		},
		GetRobotFunc: func(ctx context.Context, in *apppb.GetRobotRequest,
			opts ...grpc.CallOption,
		) (*apppb.GetRobotResponse, error) {
			return &apppb.GetRobotResponse{Robot: &apppb.Robot{Id: in.Id, Name: "my-machine"}}, nil
		},
	}

	script := filepath.Join(t.TempDir(), "fake-app.sh")
	test.That(t, os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0o755), test.ShouldBeNil)

	cCtx, ac, out, _ := setup(asc, nil, nil, nil, "token")
	ac.selectedOrg = &apppb.Organization{Id: "org-id"}
	ac.selectedLoc = &apppb.Location{Id: "loc-id"}
	err := ac.visualizePart(context.Background(), cCtx, visualizeArgs{
		Organization: "org-id", Location: "loc-id", Part: partID, AppPath: script,
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(out.messages), test.ShouldBeGreaterThan, 0)
	test.That(t, out.messages[0], test.ShouldContainSubstring, "main-part")
	test.That(t, out.messages[0], test.ShouldContainSubstring, "my-machine")
	test.That(t, out.messages[0], test.ShouldNotContainSubstring, testToken)
}
