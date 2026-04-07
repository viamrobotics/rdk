package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pkg/errors"
	v1 "go.viam.com/api/app/build/v1"
	apppb "go.viam.com/api/app/v1"
	"go.viam.com/test"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	rdkConfig "go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/testutils/inject"
)

func TestConfigureModule(t *testing.T) {
	manifestPath := createTestManifest(t, "", nil)
	cCtx, ac, out, errOut := setup(&inject.AppServiceClient{}, nil, &inject.BuildServiceClient{
		StartBuildFunc: func(ctx context.Context, in *v1.StartBuildRequest, opts ...grpc.CallOption) (*v1.StartBuildResponse, error) {
			return &v1.StartBuildResponse{BuildId: "xyz123"}, nil
		},
	}, map[string]any{moduleFlagPath: manifestPath, generalFlagVersion: "1.2.3"}, "token")
	path, err := ac.moduleBuildStartAction(context.Background(), cCtx, parseStructFromCtx[moduleBuildStartArgs](cCtx))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, path, test.ShouldEqual, "xyz123")
	test.That(t, out.messages, test.ShouldHaveLength, 1)
	test.That(t, out.messages[0], test.ShouldEqual, "xyz123\n")
	test.That(t, errOut.messages, test.ShouldHaveLength, 2)
	test.That(t, errOut.messages[0], test.ShouldEqual, "Build started, follow the logs with:\n")
	test.That(t, errOut.messages[1], test.ShouldEqual, "\tviam module build logs --id xyz123\n")
}

// Helper function to create a mock AppServiceClient with robot part.
func mockAppServiceClientWithRobotPart(
	robotConfig, userSuppliedInfo *structpb.Struct,
) *inject.AppServiceClient {
	return &inject.AppServiceClient{
		GetRobotPartFunc: func(ctx context.Context, req *apppb.GetRobotPartRequest,
			opts ...grpc.CallOption,
		) (*apppb.GetRobotPartResponse, error) {
			return &apppb.GetRobotPartResponse{Part: &apppb.RobotPart{
				RobotConfig:      robotConfig,
				Fqdn:             "test-robot.local",
				UserSuppliedInfo: userSuppliedInfo,
				LastUpdated:      timestamppb.New(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
			}, ConfigJson: ``}, nil
		},
	}
}

// Helper function to create full mock AppServiceClient with update and API keys.
func mockFullAppServiceClient(robotConfig, userSuppliedInfo *structpb.Struct, updateCount *int) *inject.AppServiceClient {
	client := mockAppServiceClientWithRobotPart(robotConfig, userSuppliedInfo)
	client.UpdateRobotPartFunc = func(ctx context.Context, req *apppb.UpdateRobotPartRequest,
		opts ...grpc.CallOption,
	) (*apppb.UpdateRobotPartResponse, error) {
		if updateCount != nil {
			(*updateCount)++
		}
		return &apppb.UpdateRobotPartResponse{Part: &apppb.RobotPart{}}, nil
	}
	client.GetRobotAPIKeysFunc = func(ctx context.Context, in *apppb.GetRobotAPIKeysRequest,
		opts ...grpc.CallOption,
	) (*apppb.GetRobotAPIKeysResponse, error) {
		return &apppb.GetRobotAPIKeysResponse{ApiKeys: []*apppb.APIKeyWithAuthorizations{
			{ApiKey: &apppb.APIKey{}},
		}}, nil
	}
	return client
}

// mockFullAppServiceClientWithLastKnownUpdate is like mockFullAppServiceClient but also
// captures the last_known_update sent in UpdateRobotPart requests.
func mockFullAppServiceClientWithLastKnownUpdate(
	robotConfig, userSuppliedInfo *structpb.Struct,
	updateCount *int,
	capturedLastKnownUpdate **timestamppb.Timestamp,
) *inject.AppServiceClient {
	client := mockAppServiceClientWithRobotPart(robotConfig, userSuppliedInfo)
	client.UpdateRobotPartFunc = func(ctx context.Context, req *apppb.UpdateRobotPartRequest,
		opts ...grpc.CallOption,
	) (*apppb.UpdateRobotPartResponse, error) {
		if updateCount != nil {
			(*updateCount)++
		}
		if capturedLastKnownUpdate != nil {
			*capturedLastKnownUpdate = req.LastKnownUpdate
		}
		return &apppb.UpdateRobotPartResponse{Part: &apppb.RobotPart{}}, nil
	}
	client.GetRobotAPIKeysFunc = func(ctx context.Context, in *apppb.GetRobotAPIKeysRequest,
		opts ...grpc.CallOption,
	) (*apppb.GetRobotAPIKeysResponse, error) {
		return &apppb.GetRobotAPIKeysResponse{ApiKeys: []*apppb.APIKeyWithAuthorizations{
			{ApiKey: &apppb.APIKey{}},
		}}, nil
	}
	return client
}

func TestFullReloadFlow(t *testing.T) {
	logger := logging.NewTestLogger(t)

	manifestPath := createTestManifest(t, "", nil)
	confStruct, err := structpb.NewStruct(map[string]any{
		"modules": []any{},
	})
	test.That(t, err, test.ShouldBeNil)

	updateCount := 0
	cCtx, vc, _, _ := setup(
		mockFullAppServiceClient(confStruct, nil, &updateCount),
		nil,
		&inject.BuildServiceClient{},
		map[string]any{
			moduleFlagPath: manifestPath, generalFlagPartID: "part-123",
			moduleBuildFlagNoBuild: true, moduleFlagLocal: true,
			generalFlagNoProgress: true, // Disable progress spinner to avoid race conditions in tests
		},
		"token",
	)
	test.That(t, vc.loginAction(context.Background(), cCtx), test.ShouldBeNil)
	err = reloadModuleActionInner(context.Background(), cCtx, vc, parseStructFromCtx[reloadModuleArgs](cCtx), logger, false)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, updateCount, test.ShouldEqual, 1)

	t.Run("addShellService", func(t *testing.T) {
		t.Run("addsServiceWhenMissing", func(t *testing.T) {
			part, _ := vc.getRobotPart(context.Background(), "id")
			_, err := addShellService(context.Background(), cCtx, vc, logging.NewTestLogger(t), part.Part, false)
			test.That(t, err, test.ShouldBeNil)
			services, ok := part.Part.RobotConfig.AsMap()["services"].([]any)
			test.That(t, ok, test.ShouldBeTrue)
			test.That(t, services, test.ShouldNotBeNil)
			test.That(t, len(services), test.ShouldEqual, 1)
			service := services[0].(map[string]any)
			test.That(t, service["name"], test.ShouldEqual, "shell")
			test.That(t, service["api"], test.ShouldEqual, "rdk:service:shell")
		})

		// Helper function to test detection of existing shell service
		testExistingShellService := func(t *testing.T, serviceConfig map[string]any) {
			t.Helper()
			confWithService, err := structpb.NewStruct(map[string]any{
				"modules":  []any{},
				"services": []any{serviceConfig},
			})
			test.That(t, err, test.ShouldBeNil)

			cCtx2, vc2, _, _ := setup(
				mockAppServiceClientWithRobotPart(confWithService, nil),
				nil,
				&inject.BuildServiceClient{},
				map[string]any{moduleFlagPath: manifestPath},
				"token",
			)

			part, _ := vc2.getRobotPart(context.Background(), "id")
			_, err = addShellService(context.Background(), cCtx2, vc2, logging.NewTestLogger(t), part.Part, false)
			test.That(t, err, test.ShouldBeNil)
			services, ok := part.Part.RobotConfig.AsMap()["services"].([]any)
			test.That(t, ok, test.ShouldBeTrue)
			// Should still have only 1 service (not added again)
			test.That(t, len(services), test.ShouldEqual, 1)
		}

		t.Run("detectsExistingServiceWithTypeField", func(t *testing.T) {
			testExistingShellService(t, map[string]any{
				"type": "shell",
				"name": "existing-shell",
			})
		})

		t.Run("detectsExistingServiceWithApiField", func(t *testing.T) {
			testExistingShellService(t, map[string]any{
				"api":  "rdk:service:shell",
				"name": "existing-shell",
			})
		})
	})

	t.Run("versionCheck", func(t *testing.T) {
		// Test with unsupported version (too old)
		t.Run("unsupportedVersion", func(t *testing.T) {
			userInfo, err := structpb.NewStruct(map[string]any{
				"version":  "0.89.0",
				"platform": "linux/amd64",
			})
			test.That(t, err, test.ShouldBeNil)

			cCtx, vc, _, _ := setup(
				mockAppServiceClientWithRobotPart(confStruct, userInfo),
				nil,
				&inject.BuildServiceClient{},
				map[string]any{
					moduleFlagPath: manifestPath, generalFlagPartID: "part-123",
					moduleBuildFlagNoBuild: true, moduleFlagLocal: true,
					generalFlagNoProgress: true, // Disable progress spinner to avoid race conditions in tests
				},
				"token",
			)

			err = reloadModuleActionInner(context.Background(), cCtx, vc, parseStructFromCtx[reloadModuleArgs](cCtx), logger, false)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "not supported for hot reloading")
		})

		// Test with supported version
		t.Run("supportedVersion", func(t *testing.T) {
			userInfo, err := structpb.NewStruct(map[string]any{
				"version":  "0.90.0",
				"platform": "linux/amd64",
			})
			test.That(t, err, test.ShouldBeNil)

			updateCount := 0
			cCtx, vc, _, _ := setup(
				mockFullAppServiceClient(confStruct, userInfo, &updateCount),
				nil,
				&inject.BuildServiceClient{},
				map[string]any{
					moduleFlagPath: manifestPath, generalFlagPartID: "part-123",
					moduleBuildFlagNoBuild: true, moduleFlagLocal: true,
					generalFlagNoProgress: true, // Disable progress spinner to avoid race conditions in tests
				},
				"token",
			)

			err = reloadModuleActionInner(context.Background(), cCtx, vc, parseStructFromCtx[reloadModuleArgs](cCtx), logger, false)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, updateCount, test.ShouldEqual, 1)
		})
	})
}

func TestReloadWithCloudConfig(t *testing.T) {
	logger := logging.NewTestLogger(t)

	manifestPath := createTestManifest(t, "", nil)
	confStruct, err := structpb.NewStruct(map[string]any{
		"modules": []any{},
	})
	test.That(t, err, test.ShouldBeNil)

	// Create a temporary cloud config file
	cloudConfigPath := filepath.Join(t.TempDir(), "viam.json")
	cloudConfigFile, err := os.Create(cloudConfigPath)
	test.That(t, err, test.ShouldBeNil)
	_, err = cloudConfigFile.WriteString(`{"cloud":{"app_address":"https://app.viam.com:443","id":"cloud-config-part-id","secret":"SECRET"}}`)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cloudConfigFile.Close(), test.ShouldBeNil)

	t.Run("reloadWithCloudConfigLocal", func(t *testing.T) {
		updateCount := 0
		cCtx, vc, _, _ := setup(
			mockFullAppServiceClient(confStruct, nil, &updateCount),
			nil,
			&inject.BuildServiceClient{},
			map[string]any{
				moduleFlagPath:             manifestPath,
				moduleBuildFlagCloudConfig: cloudConfigPath,
				moduleBuildFlagNoBuild:     true,
				moduleFlagLocal:            true,
				generalFlagNoProgress:      true, // Disable progress spinner to avoid race conditions in tests
			},
			"token",
		)
		test.That(t, vc.loginAction(context.Background(), cCtx), test.ShouldBeNil)

		// Test that reloadModuleActionInner correctly uses cloud-config to resolve part ID
		err = reloadModuleActionInner(context.Background(), cCtx, vc, parseStructFromCtx[reloadModuleArgs](cCtx), logger, false)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, updateCount, test.ShouldEqual, 1)
	})

	t.Run("verifyPartIDResolution", func(t *testing.T) {
		// Test that the part ID is correctly resolved from cloud-config
		// and that args.PartID remains empty when only cloud-config is provided
		cCtx, _, _, _ := setup(
			mockFullAppServiceClient(confStruct, nil, nil),
			nil,
			&inject.BuildServiceClient{},
			map[string]any{
				moduleFlagPath:             manifestPath,
				moduleBuildFlagCloudConfig: cloudConfigPath,
				moduleBuildFlagNoBuild:     true,
				moduleFlagLocal:            true,
			},
			"token",
		)

		args := parseStructFromCtx[reloadModuleArgs](cCtx)
		// Verify that args.PartID is empty (not set via --part-id flag) and CloudConfig is set
		test.That(t, args.PartID, test.ShouldBeEmpty)
		test.That(t, args.CloudConfig, test.ShouldEqual, cloudConfigPath)

		// Verify that resolvePartID correctly extracts the part ID from the cloud config
		partID, err := resolvePartID(args.PartID, args.CloudConfig)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, partID, test.ShouldEqual, "cloud-config-part-id")
	})
}

func TestRestartModule(t *testing.T) {
	t.Skip("restartModule test requires fake robot client")
}

func TestResolvePartId(t *testing.T) {
	c := newTestContext(t, map[string]any{})
	// empty flag, no path
	partID, err := resolvePartID(c.String(generalFlagPartID), "")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, partID, test.ShouldBeEmpty)

	// empty flag, fake path
	missingPath := filepath.Join(t.TempDir(), "MISSING.json")
	_, err = resolvePartID(c.String(generalFlagPartID), missingPath)
	test.That(t, err, test.ShouldNotBeNil)

	// empty flag, valid path
	path := filepath.Join(t.TempDir(), "viam.json")
	fi, err := os.Create(path)
	test.That(t, err, test.ShouldBeNil)
	_, err = fi.WriteString(`{"cloud":{"app_address":"https://app.viam.com:443","id":"JSON-PART","secret":"SECRET"}}`)
	test.That(t, err, test.ShouldBeNil)
	partID, err = resolvePartID(c.String(generalFlagPartID), path)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, partID, test.ShouldEqual, "JSON-PART")

	// given flag, valid path
	c = newTestContext(t, map[string]any{generalFlagPartID: "FLAG-PART"})
	partID, err = resolvePartID(c.String(generalFlagPartID), path)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, partID, test.ShouldEqual, "FLAG-PART")
}

func TestMutateModuleConfig(t *testing.T) {
	c := newTestContext(t, map[string]any{"local": true})
	manifest := ModuleManifest{
		ModuleID:     "viam-labs:test-module",
		JSONManifest: rdkConfig.JSONManifest{Entrypoint: "/bin/mod"},
		Build:        &manifestBuildInfo{Path: "module.tar.gz"},
	}
	expectedName := "viam-labs_test-module_from_reload"
	expectedVersion := "latest-with-prerelease"
	remoteReloadPath := ".viam/packages-local/viam-labs_test-module_from_reload-module.tar.gz"
	testUser := "test@viam.com"
	testReloadUnixTS := time.Date(2024, 3, 18, 12, 0, 0, 0, time.UTC).Unix()

	t.Run("correct_reload_path_and_enabled", func(t *testing.T) {
		// correct ReloadPath and ReloadEnabled -- still dirty because user/time are always updated
		modules := []ModuleMap{{
			"type":           string(rdkConfig.ModuleTypeRegistry),
			"module_id":      manifest.ModuleID,
			"reload_path":    manifest.Entrypoint,
			"reload_enabled": true,
		}}
		_, dirty, needsRestart, err := mutateModuleConfig(c, modules, manifest, true, false, testUser, testReloadUnixTS)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, dirty, test.ShouldBeTrue)
		test.That(t, needsRestart, test.ShouldBeTrue)
		test.That(t, modules[0]["reload_user"], test.ShouldEqual, testUser)
		test.That(t, modules[0]["reload_time"], test.ShouldNotBeEmpty)
	})

	t.Run("correct_reload_path_and_disabled", func(t *testing.T) {
		// correct ReloadPath, but ReloadEnabled is false in registry module
		modules := []ModuleMap{{
			"type":           string(rdkConfig.ModuleTypeRegistry),
			"module_id":      manifest.ModuleID,
			"reload_path":    manifest.Entrypoint,
			"reload_enabled": false,
		}}
		_, dirty, needsRestart, err := mutateModuleConfig(c, modules, manifest, true, false, testUser, testReloadUnixTS)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, dirty, test.ShouldBeTrue)
		test.That(t, needsRestart, test.ShouldBeFalse)
		test.That(t, modules[0]["reload_enabled"], test.ShouldBeTrue)
		test.That(t, modules[0]["reload_user"], test.ShouldEqual, testUser)
		test.That(t, modules[0]["reload_time"], test.ShouldNotBeEmpty)
	})

	t.Run("incorrect_reload_path_and_disabled", func(t *testing.T) {
		// incorrect ReloadPath and ReloadEnabled is false in registry module
		modules := []ModuleMap{{
			"type":           string(rdkConfig.ModuleTypeRegistry),
			"module_id":      manifest.ModuleID,
			"reload_path":    "incorrect/path",
			"reload_enabled": false,
		}}
		_, dirty, needsRestart, err := mutateModuleConfig(c, modules, manifest, true, false, testUser, testReloadUnixTS)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, dirty, test.ShouldBeTrue)
		test.That(t, needsRestart, test.ShouldBeFalse)
		test.That(t, modules[0]["reload_path"], test.ShouldEqual, manifest.Entrypoint)
		test.That(t, modules[0]["reload_enabled"], test.ShouldBeTrue)
		test.That(t, modules[0]["reload_user"], test.ShouldEqual, testUser)
		test.That(t, modules[0]["reload_time"], test.ShouldNotBeEmpty)
	})

	t.Run("reload_fields_missing", func(t *testing.T) {
		// ReloadPath and ReloadEnabled are both missing from the module map in registry module
		modules := []ModuleMap{{
			"type":      string(rdkConfig.ModuleTypeRegistry),
			"module_id": manifest.ModuleID,
		}}
		_, dirty, needsRestart, err := mutateModuleConfig(c, modules, manifest, true, false, testUser, testReloadUnixTS)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, dirty, test.ShouldBeTrue)
		test.That(t, needsRestart, test.ShouldBeFalse)
		test.That(t, modules[0]["reload_path"], test.ShouldEqual, manifest.Entrypoint)
		test.That(t, modules[0]["reload_enabled"], test.ShouldBeTrue)
		test.That(t, modules[0]["reload_user"], test.ShouldEqual, testUser)
		test.That(t, modules[0]["reload_time"], test.ShouldNotBeEmpty)
	})

	t.Run("insert_when_missing", func(t *testing.T) {
		modules := []ModuleMap{}
		modules, _, _, _ = mutateModuleConfig(c, modules, manifest, true, false, testUser, testReloadUnixTS)
		test.That(t, modules[0]["module_id"], test.ShouldEqual, manifest.ModuleID)
		test.That(t, modules[0]["name"], test.ShouldEqual, expectedName)
		test.That(t, modules[0]["reload_path"], test.ShouldEqual, manifest.Entrypoint)
		test.That(t, modules[0]["reload_enabled"], test.ShouldBeTrue)
		test.That(t, modules[0]["version"], test.ShouldEqual, expectedVersion)
		test.That(t, modules[0]["reload_user"], test.ShouldEqual, testUser)
		test.That(t, modules[0]["reload_time"], test.ShouldNotBeEmpty)
	})

	t.Run("insert_when_local_module_found", func(t *testing.T) {
		// ReloadPath and ReloadEnabled are both missing from the module map in registry module
		modules := []ModuleMap{{
			"type":      string(rdkConfig.ModuleTypeLocal),
			"module_id": manifest.ModuleID,
		}}
		updatedModules, _, _, _ := mutateModuleConfig(c, modules, manifest, true, false, testUser, testReloadUnixTS)
		test.That(t, len(updatedModules), test.ShouldEqual, 2)
		test.That(t, updatedModules[1]["reload_path"], test.ShouldEqual, manifest.Entrypoint)
		test.That(t, updatedModules[1]["reload_enabled"], test.ShouldBeTrue)
		test.That(t, updatedModules[1]["version"], test.ShouldEqual, expectedVersion)
		test.That(t, updatedModules[1]["reload_user"], test.ShouldEqual, testUser)
		test.That(t, updatedModules[1]["reload_time"], test.ShouldNotBeEmpty)
	})

	c = newTestContext(t, map[string]any{})
	t.Run("remote_insert", func(t *testing.T) {
		modules, _, _, _ := mutateModuleConfig(c, []ModuleMap{}, manifest, false, false, testUser, testReloadUnixTS)
		test.That(t, modules[0]["module_id"], test.ShouldEqual, manifest.ModuleID)
		test.That(t, modules[0]["name"], test.ShouldEqual, expectedName)
		test.That(t, modules[0]["reload_path"], test.ShouldEqual, remoteReloadPath)
		test.That(t, modules[0]["reload_enabled"], test.ShouldBeTrue)
		test.That(t, modules[0]["version"], test.ShouldEqual, expectedVersion)
		test.That(t, modules[0]["reload_user"], test.ShouldEqual, testUser)
		test.That(t, modules[0]["reload_time"], test.ShouldNotBeEmpty)
	})
}

func TestReloadWithMissingBuildSection(t *testing.T) {
	logger := logging.NewTestLogger(t)

	t.Run("reload-local with missing build section", func(t *testing.T) {
		// Create manifest without build section (nil value deletes the key)
		manifestPath := createTestManifest(t, "", map[string]any{
			"build": nil,
		})

		confStruct, err := structpb.NewStruct(map[string]any{
			"modules": []any{},
		})
		test.That(t, err, test.ShouldBeNil)

		userInfo, err := structpb.NewStruct(map[string]any{
			"version":  "0.90.0",
			"platform": "linux/amd64",
		})
		test.That(t, err, test.ShouldBeNil)

		cCtx, vc, _, _ := setup(
			mockFullAppServiceClient(confStruct, userInfo, nil),
			nil,
			&inject.BuildServiceClient{},
			map[string]any{
				moduleFlagPath:        manifestPath,
				generalFlagPartID:     "part-123",
				moduleFlagLocal:       true,
				generalFlagNoProgress: true,
			},
			"token",
		)

		// Test reload-local (cloudBuild=false)
		err = reloadModuleActionInner(context.Background(), cCtx, vc, parseStructFromCtx[reloadModuleArgs](cCtx), logger, false)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "cannot have an empty build step")
		test.That(t, err.Error(), test.ShouldContainSubstring, "required for 'reload' and 'reload-local' commands")
	})

	t.Run("reload (cloud) with missing build section", func(t *testing.T) {
		// Create manifest without build section (nil value deletes the key)
		manifestPath := createTestManifest(t, "", map[string]any{
			"build": nil,
		})

		confStruct, err := structpb.NewStruct(map[string]any{
			"modules": []any{},
		})
		test.That(t, err, test.ShouldBeNil)

		userInfo, err := structpb.NewStruct(map[string]any{
			"version":  "0.90.0",
			"platform": "linux/amd64",
		})
		test.That(t, err, test.ShouldBeNil)

		cCtx, vc, _, _ := setup(
			mockFullAppServiceClient(confStruct, userInfo, nil),
			nil,
			&inject.BuildServiceClient{},
			map[string]any{
				moduleFlagPath:        manifestPath,
				generalFlagPartID:     "part-123",
				generalFlagNoProgress: true,
			},
			"token",
		)

		// Test reload with cloud build (cloudBuild=true)
		err = reloadModuleActionInner(context.Background(), cCtx, vc, parseStructFromCtx[reloadModuleArgs](cCtx), logger, true)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "cannot have an empty build step")
		test.That(t, err.Error(), test.ShouldContainSubstring, "required for 'reload' and 'reload-local' commands")
	})

	t.Run("reload-local with empty build command", func(t *testing.T) {
		// Create manifest with build section but empty build command
		manifestPath := createTestManifest(t, "", map[string]any{
			"build": map[string]any{
				"path": "module",
				"arch": []any{"linux/amd64"},
				// "build" field is missing or empty
			},
		})

		confStruct, err := structpb.NewStruct(map[string]any{
			"modules": []any{},
		})
		test.That(t, err, test.ShouldBeNil)

		userInfo, err := structpb.NewStruct(map[string]any{
			"version":  "0.90.0",
			"platform": "linux/amd64",
		})
		test.That(t, err, test.ShouldBeNil)

		cCtx, vc, _, _ := setup(
			mockFullAppServiceClient(confStruct, userInfo, nil),
			nil,
			&inject.BuildServiceClient{},
			map[string]any{
				moduleFlagPath:        manifestPath,
				generalFlagPartID:     "part-123",
				moduleFlagLocal:       true,
				generalFlagNoProgress: true,
			},
			"token",
		)

		err = reloadModuleActionInner(context.Background(), cCtx, vc, parseStructFromCtx[reloadModuleArgs](cCtx), logger, false)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "cannot have an empty build step")
	})

	t.Run("reload-local with --no-build flag still requires manifest", func(t *testing.T) {
		// Create manifest without build section (nil value deletes the key)
		manifestPath := createTestManifest(t, "", map[string]any{
			"build": nil,
		})

		confStruct, err := structpb.NewStruct(map[string]any{
			"modules": []any{},
		})
		test.That(t, err, test.ShouldBeNil)

		userInfo, err := structpb.NewStruct(map[string]any{
			"version":  "0.90.0",
			"platform": "linux/amd64",
		})
		test.That(t, err, test.ShouldBeNil)

		cCtx, vc, _, _ := setup(
			mockFullAppServiceClient(confStruct, userInfo, nil),
			nil,
			&inject.BuildServiceClient{},
			map[string]any{
				moduleFlagPath:         manifestPath,
				generalFlagPartID:      "part-123",
				moduleBuildFlagNoBuild: true, // --no-build flag
				moduleFlagLocal:        true,
				generalFlagNoProgress:  true,
			},
			"token",
		)

		// Even with --no-build flag, manifest with build section is still required
		// because it needs to know where to find the already-built artifact
		err = reloadModuleActionInner(context.Background(), cCtx, vc, parseStructFromCtx[reloadModuleArgs](cCtx), logger, false)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "manifest required for reload")
	})
}

func TestUpdateRobotPartPassesLastKnownUpdate(t *testing.T) {
	logger := logging.NewTestLogger(t)
	manifestPath := createTestManifest(t, "", nil)
	confStruct, err := structpb.NewStruct(map[string]any{
		"modules": []any{},
	})
	test.That(t, err, test.ShouldBeNil)

	updateCount := 0
	var capturedTimestamp *timestamppb.Timestamp
	expectedTimestamp := timestamppb.New(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	cCtx, vc, _, _ := setup(
		mockFullAppServiceClientWithLastKnownUpdate(confStruct, nil, &updateCount, &capturedTimestamp),
		nil,
		&inject.BuildServiceClient{},
		map[string]any{
			moduleFlagPath: manifestPath, generalFlagPartID: "part-123",
			moduleBuildFlagNoBuild: true, moduleFlagLocal: true,
			generalFlagNoProgress: true,
		},
		"token",
	)
	test.That(t, vc.loginAction(context.Background(), cCtx), test.ShouldBeNil)
	err = reloadModuleActionInner(context.Background(), cCtx, vc, parseStructFromCtx[reloadModuleArgs](cCtx), logger, false)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, updateCount, test.ShouldEqual, 1)
	test.That(t, capturedTimestamp, test.ShouldNotBeNil)
	test.That(t, capturedTimestamp.AsTime().Equal(expectedTimestamp.AsTime()), test.ShouldBeTrue)
}

func TestUpdateRobotPartRetryOnConflict(t *testing.T) {
	manifestPath := createTestManifest(t, "", nil)
	confStruct, err := structpb.NewStruct(map[string]any{
		"modules": []any{},
	})
	test.That(t, err, test.ShouldBeNil)

	updateCount := 0
	client := mockAppServiceClientWithRobotPart(confStruct, nil)
	client.UpdateRobotPartFunc = func(ctx context.Context, req *apppb.UpdateRobotPartRequest,
		opts ...grpc.CallOption,
	) (*apppb.UpdateRobotPartResponse, error) {
		updateCount++
		if updateCount == 1 {
			return nil, errors.New("concurrent modification")
		}
		return &apppb.UpdateRobotPartResponse{Part: &apppb.RobotPart{}}, nil
	}
	client.GetRobotAPIKeysFunc = func(ctx context.Context, in *apppb.GetRobotAPIKeysRequest,
		opts ...grpc.CallOption,
	) (*apppb.GetRobotAPIKeysResponse, error) {
		return &apppb.GetRobotAPIKeysResponse{ApiKeys: []*apppb.APIKeyWithAuthorizations{
			{ApiKey: &apppb.APIKey{}},
		}}, nil
	}

	cCtx, vc, _, _ := setup(
		client,
		nil,
		&inject.BuildServiceClient{},
		map[string]any{
			moduleFlagPath: manifestPath, generalFlagPartID: "part-123",
			moduleBuildFlagNoBuild: true, moduleFlagLocal: true,
			generalFlagNoProgress: true,
		},
		"token",
	)
	test.That(t, vc.loginAction(context.Background(), cCtx), test.ShouldBeNil)
	logger := logging.NewTestLogger(t)
	err = reloadModuleActionInner(context.Background(), cCtx, vc, parseStructFromCtx[reloadModuleArgs](cCtx), logger, false)
	test.That(t, err, test.ShouldBeNil)
	// First call fails, triggers re-fetch + retry which succeeds
	test.That(t, updateCount, test.ShouldEqual, 2)
}

// TestConfigureModuleNeedsRestart verifies that configureModule returns the correct needsRestart
// signal. When the module path and reload_enabled are already correct, the caller must still
// restart the module binary even though we write reload_user/reload_time metadata.
// Regression test for: reload_user/reload_time always setting dirty=true caused needsRestart
// to be permanently false when it was derived as !dirty.
func TestConfigureModuleNeedsRestart(t *testing.T) {
	manifest := &ModuleManifest{
		ModuleID:     "viam-labs:test-module",
		JSONManifest: rdkConfig.JSONManifest{Entrypoint: "/bin/mod"},
		Build:        &manifestBuildInfo{Path: "module.tar.gz"},
	}
	testUser := "test@viam.com"
	testReloadUnixTS := time.Date(2024, 3, 18, 12, 0, 0, 0, time.UTC).Unix()
	localizedName := localizeModuleID(manifest.ModuleID)

	mockClient := func(t *testing.T, part *apppb.RobotPart) *inject.AppServiceClient {
		t.Helper()
		return &inject.AppServiceClient{
			GetRobotPartFunc: func(ctx context.Context, req *apppb.GetRobotPartRequest,
				opts ...grpc.CallOption,
			) (*apppb.GetRobotPartResponse, error) {
				return &apppb.GetRobotPartResponse{Part: part}, nil
			},
			UpdateRobotPartFunc: func(ctx context.Context, req *apppb.UpdateRobotPartRequest,
				opts ...grpc.CallOption,
			) (*apppb.UpdateRobotPartResponse, error) {
				return &apppb.UpdateRobotPartResponse{Part: part}, nil
			},
		}
	}

	makePart := func(t *testing.T, modules []any) *apppb.RobotPart {
		t.Helper()
		confStruct, err := structpb.NewStruct(map[string]any{"modules": modules})
		test.That(t, err, test.ShouldBeNil)
		return &apppb.RobotPart{
			Id:          "part-123",
			Name:        "test-part",
			RobotConfig: confStruct,
			LastUpdated: timestamppb.New(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
		}
	}

	t.Run("already_configured_needs_restart", func(t *testing.T) {
		part := makePart(t, []any{map[string]any{
			"type":           string(rdkConfig.ModuleTypeRegistry),
			"module_id":      manifest.ModuleID,
			"name":           localizedName,
			"reload_path":    manifest.Entrypoint,
			"reload_enabled": true,
			"version":        "latest-with-prerelease",
		}})
		cmd, vc, _, _ := setup(mockClient(t, part), nil, &inject.BuildServiceClient{},
			map[string]any{moduleFlagLocal: true}, "token")
		_, needsRestart, err := configureModule(context.Background(), cmd, vc, manifest, part, true, false, testUser, testReloadUnixTS)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, needsRestart, test.ShouldBeTrue)
	})

	t.Run("first_time_config_no_restart", func(t *testing.T) {
		part := makePart(t, []any{})
		cmd, vc, _, _ := setup(mockClient(t, part), nil, &inject.BuildServiceClient{},
			map[string]any{moduleFlagLocal: true}, "token")
		_, needsRestart, err := configureModule(context.Background(), cmd, vc, manifest, part, true, false, testUser, testReloadUnixTS)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, needsRestart, test.ShouldBeFalse)
	})

	t.Run("path_changed_no_restart", func(t *testing.T) {
		part := makePart(t, []any{map[string]any{
			"type":           string(rdkConfig.ModuleTypeRegistry),
			"module_id":      manifest.ModuleID,
			"name":           localizedName,
			"reload_path":    "/old/path",
			"reload_enabled": true,
			"version":        "latest-with-prerelease",
		}})
		cmd, vc, _, _ := setup(mockClient(t, part), nil, &inject.BuildServiceClient{},
			map[string]any{moduleFlagLocal: true}, "token")
		_, needsRestart, err := configureModule(context.Background(), cmd, vc, manifest, part, true, false, testUser, testReloadUnixTS)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, needsRestart, test.ShouldBeFalse)
	})

	t.Run("disabled_to_enabled_no_restart", func(t *testing.T) {
		part := makePart(t, []any{map[string]any{
			"type":           string(rdkConfig.ModuleTypeRegistry),
			"module_id":      manifest.ModuleID,
			"name":           localizedName,
			"reload_path":    manifest.Entrypoint,
			"reload_enabled": false,
			"version":        "latest-with-prerelease",
		}})
		cmd, vc, _, _ := setup(mockClient(t, part), nil, &inject.BuildServiceClient{},
			map[string]any{moduleFlagLocal: true}, "token")
		_, needsRestart, err := configureModule(context.Background(), cmd, vc, manifest, part, true, false, testUser, testReloadUnixTS)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, needsRestart, test.ShouldBeFalse)
	})

	t.Run("already_configured_still_writes_update", func(t *testing.T) {
		updateCount := 0
		part := makePart(t, []any{map[string]any{
			"type":           string(rdkConfig.ModuleTypeRegistry),
			"module_id":      manifest.ModuleID,
			"name":           localizedName,
			"reload_path":    manifest.Entrypoint,
			"reload_enabled": true,
			"version":        "latest-with-prerelease",
		}})
		client := &inject.AppServiceClient{
			GetRobotPartFunc: func(ctx context.Context, req *apppb.GetRobotPartRequest,
				opts ...grpc.CallOption,
			) (*apppb.GetRobotPartResponse, error) {
				return &apppb.GetRobotPartResponse{Part: part}, nil
			},
			UpdateRobotPartFunc: func(ctx context.Context, req *apppb.UpdateRobotPartRequest,
				opts ...grpc.CallOption,
			) (*apppb.UpdateRobotPartResponse, error) {
				updateCount++
				return &apppb.UpdateRobotPartResponse{Part: part}, nil
			},
		}
		cmd, vc, _, _ := setup(client, nil, &inject.BuildServiceClient{},
			map[string]any{moduleFlagLocal: true}, "token")
		_, needsRestart, err := configureModule(context.Background(), cmd, vc, manifest, part, true, false, testUser, testReloadUnixTS)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, needsRestart, test.ShouldBeTrue)
		test.That(t, updateCount, test.ShouldEqual, 1)
	})
}

// TestRepeatedReloadNeedsRestart is an integration-level test that simulates two consecutive
// reload cycles. The first reload adds the module (no restart needed because the config change
// itself triggers reconfiguration). The second reload finds the module already configured and
// must signal that a restart is needed.
func TestRepeatedReloadNeedsRestart(t *testing.T) {
	manifest := &ModuleManifest{
		ModuleID:     "viam-labs:test-module",
		JSONManifest: rdkConfig.JSONManifest{Entrypoint: "/bin/mod"},
		Build:        &manifestBuildInfo{Path: "module.tar.gz"},
	}
	testUser := "test@viam.com"
	testReloadUnixTS := time.Date(2024, 3, 18, 12, 0, 0, 0, time.UTC).Unix()

	initialConf, err := structpb.NewStruct(map[string]any{"modules": []any{}})
	test.That(t, err, test.ShouldBeNil)

	// Track the latest config written by UpdateRobotPart so subsequent GetRobotPart
	// calls reflect the changes (simulating server persistence).
	latestConfig := initialConf
	latestTimestamp := timestamppb.New(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	client := &inject.AppServiceClient{
		GetRobotPartFunc: func(ctx context.Context, req *apppb.GetRobotPartRequest,
			opts ...grpc.CallOption,
		) (*apppb.GetRobotPartResponse, error) {
			return &apppb.GetRobotPartResponse{Part: &apppb.RobotPart{
				Id:          "part-123",
				Name:        "test-part",
				RobotConfig: latestConfig,
				LastUpdated: latestTimestamp,
			}}, nil
		},
		UpdateRobotPartFunc: func(ctx context.Context, req *apppb.UpdateRobotPartRequest,
			opts ...grpc.CallOption,
		) (*apppb.UpdateRobotPartResponse, error) {
			latestConfig = req.RobotConfig
			latestTimestamp = timestamppb.Now()
			return &apppb.UpdateRobotPartResponse{Part: &apppb.RobotPart{
				RobotConfig: latestConfig,
				LastUpdated: latestTimestamp,
			}}, nil
		},
	}

	cmd, vc, _, _ := setup(client, nil, &inject.BuildServiceClient{},
		map[string]any{moduleFlagLocal: true}, "token")

	// First reload: module is new, so needsRestart should be false.
	part, err := vc.getRobotPart(context.Background(), "part-123")
	test.That(t, err, test.ShouldBeNil)
	_, needsRestart, err := configureModule(context.Background(), cmd, vc, manifest, part.Part, true, false, testUser, testReloadUnixTS)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, needsRestart, test.ShouldBeFalse)

	// Second reload: module is already configured with correct path and enabled.
	part, err = vc.getRobotPart(context.Background(), "part-123")
	test.That(t, err, test.ShouldBeNil)
	_, needsRestart, err = configureModule(context.Background(), cmd, vc, manifest, part.Part, true, false, testUser, testReloadUnixTS)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, needsRestart, test.ShouldBeTrue)
}

func TestReloadUserAndTimeInModuleConfig(t *testing.T) {
	logger := logging.NewTestLogger(t)
	manifestPath := createTestManifest(t, "", nil)
	confStruct, err := structpb.NewStruct(map[string]any{
		"modules": []any{},
	})
	test.That(t, err, test.ShouldBeNil)

	var capturedConfig *structpb.Struct
	updateCount := 0
	client := mockAppServiceClientWithRobotPart(confStruct, nil)
	client.UpdateRobotPartFunc = func(ctx context.Context, req *apppb.UpdateRobotPartRequest,
		opts ...grpc.CallOption,
	) (*apppb.UpdateRobotPartResponse, error) {
		updateCount++
		capturedConfig = req.RobotConfig
		return &apppb.UpdateRobotPartResponse{Part: &apppb.RobotPart{}}, nil
	}
	client.GetRobotAPIKeysFunc = func(ctx context.Context, in *apppb.GetRobotAPIKeysRequest,
		opts ...grpc.CallOption,
	) (*apppb.GetRobotAPIKeysResponse, error) {
		return &apppb.GetRobotAPIKeysResponse{ApiKeys: []*apppb.APIKeyWithAuthorizations{
			{ApiKey: &apppb.APIKey{}},
		}}, nil
	}

	cmd, vc, _, _ := setup(
		client,
		nil,
		&inject.BuildServiceClient{},
		map[string]any{
			moduleFlagPath: manifestPath, generalFlagPartID: "part-123",
			moduleBuildFlagNoBuild: true, moduleFlagLocal: true,
			generalFlagNoProgress: true,
		},
		"token",
	)
	test.That(t, vc.loginAction(context.Background(), cmd), test.ShouldBeNil)
	err = reloadModuleActionInner(context.Background(), cmd, vc, parseStructFromCtx[reloadModuleArgs](cmd), logger, false)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, updateCount, test.ShouldEqual, 1)
	test.That(t, capturedConfig, test.ShouldNotBeNil)

	configMap := capturedConfig.AsMap()
	modules := configMap["modules"].([]any)
	test.That(t, len(modules), test.ShouldBeGreaterThan, 0)
	mod := modules[0].(map[string]any)
	test.That(t, mod["reload_user"], test.ShouldEqual, testEmail)
	test.That(t, mod["reload_time"], test.ShouldNotBeEmpty)
}
