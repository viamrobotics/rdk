package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	v1 "go.viam.com/api/app/build/v1"
	apppb "go.viam.com/api/app/v1"
	"go.viam.com/test"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"

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
	path, err := ac.moduleBuildStartAction(cCtx, parseStructFromCtx[moduleBuildStartArgs](cCtx))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, path, test.ShouldEqual, "xyz123")
	test.That(t, out.messages, test.ShouldHaveLength, 1)
	test.That(t, out.messages[0], test.ShouldEqual, "xyz123\n")
	test.That(t, errOut.messages, test.ShouldHaveLength, 1)
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
	test.That(t, vc.loginAction(cCtx), test.ShouldBeNil)
	err = reloadModuleActionInner(cCtx, vc, parseStructFromCtx[reloadModuleArgs](cCtx), logger, false)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, updateCount, test.ShouldEqual, 1)

	t.Run("addShellService", func(t *testing.T) {
		t.Run("addsServiceWhenMissing", func(t *testing.T) {
			// Create isolated setup for this subtest to avoid shared state
			updateCount := 0
			cCtx2, vc2, _, _ := setup(
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
			part, _ := vc2.getRobotPart("id")
			_, err := addShellService(cCtx2, vc2, logging.NewTestLogger(t), part.Part, false)
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

			part, _ := vc2.getRobotPart("id")
			_, err = addShellService(cCtx2, vc2, logging.NewTestLogger(t), part.Part, false)
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

			// Create isolated logger for this subtest
			err = reloadModuleActionInner(cCtx, vc, parseStructFromCtx[reloadModuleArgs](cCtx), logging.NewTestLogger(t), false)
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

			// Create isolated logger for this subtest
			err = reloadModuleActionInner(cCtx, vc, parseStructFromCtx[reloadModuleArgs](cCtx), logging.NewTestLogger(t), false)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, updateCount, test.ShouldEqual, 1)
		})
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
	manifest := moduleManifest{
		ModuleID:     "viam-labs:test-module",
		JSONManifest: rdkConfig.JSONManifest{Entrypoint: "/bin/mod"},
		Build:        &manifestBuildInfo{Path: "module.tar.gz"},
	}
	expectedName := "viam-labs_test-module_from_reload"
	expectedVersion := "latest-with-prerelease"
	remoteReloadPath := ".viam/packages-local/viam-labs_test-module_from_reload-module.tar.gz"

	t.Run("correct_reload_path_and_enabled", func(t *testing.T) {
		// correct ReloadPath and ReloadEnabled (do nothing) in registry module
		modules := []ModuleMap{{
			"type":           string(rdkConfig.ModuleTypeRegistry),
			"module_id":      manifest.ModuleID,
			"reload_path":    manifest.Entrypoint,
			"reload_enabled": true,
		}}
		_, dirty, err := mutateModuleConfig(c, modules, manifest, true)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, dirty, test.ShouldBeFalse)
	})

	t.Run("correct_reload_path_and_disabled", func(t *testing.T) {
		// correct ReloadPath, but ReloadEnabled is false in registry module
		modules := []ModuleMap{{
			"type":           string(rdkConfig.ModuleTypeRegistry),
			"module_id":      manifest.ModuleID,
			"reload_path":    manifest.Entrypoint,
			"reload_enabled": false,
		}}
		_, dirty, err := mutateModuleConfig(c, modules, manifest, true)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, dirty, test.ShouldBeTrue)
		test.That(t, modules[0]["reload_enabled"], test.ShouldBeTrue)
	})

	t.Run("incorrect_reload_path_and_disabled", func(t *testing.T) {
		// incorrect ReloadPath and ReloadEnabled is false in registry module
		modules := []ModuleMap{{
			"type":           string(rdkConfig.ModuleTypeRegistry),
			"module_id":      manifest.ModuleID,
			"reload_path":    "incorrect/path",
			"reload_enabled": false,
		}}
		_, dirty, err := mutateModuleConfig(c, modules, manifest, true)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, dirty, test.ShouldBeTrue)
		test.That(t, modules[0]["reload_path"], test.ShouldEqual, manifest.Entrypoint)
		test.That(t, modules[0]["reload_enabled"], test.ShouldBeTrue)
	})

	t.Run("reload_fields_missing", func(t *testing.T) {
		// ReloadPath and ReloadEnabled are both missing from the module map in registry module
		modules := []ModuleMap{{
			"type":      string(rdkConfig.ModuleTypeRegistry),
			"module_id": manifest.ModuleID,
		}}
		_, dirty, err := mutateModuleConfig(c, modules, manifest, true)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, dirty, test.ShouldBeTrue)
		test.That(t, modules[0]["reload_path"], test.ShouldEqual, manifest.Entrypoint)
		test.That(t, modules[0]["reload_enabled"], test.ShouldBeTrue)
	})

	t.Run("insert_when_missing", func(t *testing.T) {
		modules := []ModuleMap{}
		modules, _, _ = mutateModuleConfig(c, modules, manifest, true)
		test.That(t, modules[0]["module_id"], test.ShouldEqual, manifest.ModuleID)
		test.That(t, modules[0]["name"], test.ShouldEqual, expectedName)
		test.That(t, modules[0]["reload_path"], test.ShouldEqual, manifest.Entrypoint)
		test.That(t, modules[0]["reload_enabled"], test.ShouldBeTrue)
		test.That(t, modules[0]["version"], test.ShouldEqual, expectedVersion)
	})

	t.Run("insert_when_local_module_found", func(t *testing.T) {
		// ReloadPath and ReloadEnabled are both missing from the module map in registry module
		modules := []ModuleMap{{
			"type":      string(rdkConfig.ModuleTypeLocal),
			"module_id": manifest.ModuleID,
		}}
		updatedModules, _, _ := mutateModuleConfig(c, modules, manifest, true)
		test.That(t, len(updatedModules), test.ShouldEqual, 2)
		test.That(t, updatedModules[1]["reload_path"], test.ShouldEqual, manifest.Entrypoint)
		test.That(t, updatedModules[1]["reload_enabled"], test.ShouldBeTrue)
		test.That(t, updatedModules[1]["version"], test.ShouldEqual, expectedVersion)
	})

	c = newTestContext(t, map[string]any{})
	t.Run("remote_insert", func(t *testing.T) {
		modules, _, _ := mutateModuleConfig(c, []ModuleMap{}, manifest, false)
		test.That(t, modules[0]["module_id"], test.ShouldEqual, manifest.ModuleID)
		test.That(t, modules[0]["name"], test.ShouldEqual, expectedName)
		test.That(t, modules[0]["reload_path"], test.ShouldEqual, remoteReloadPath)
		test.That(t, modules[0]["reload_enabled"], test.ShouldBeTrue)
		test.That(t, modules[0]["version"], test.ShouldEqual, expectedVersion)
	})
}
