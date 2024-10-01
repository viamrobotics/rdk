package cli

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	v1 "go.viam.com/api/app/build/v1"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/test"
	"google.golang.org/grpc"
)

func TestGenerateModuleAction(t *testing.T) {
	expectedPythonTestModule := moduleInputs{
		ModuleName:       "my-module",
		IsPublic:         false,
		Namespace:        "my-org",
		Language:         "python",
		Resource:         "arm component",
		ResourceType:     "component",
		ResourceSubtype:  "arm",
		ModelName:        "my-model",
		EnableCloudBuild: true,
		InitializeGit:    false,
		GeneratorVersion: "0.1.0",
		GeneratedOn:      time.Now().UTC(),

		ModulePascal:          "MyModule",
		API:                   "rdk:component:arm",
		ResourceSubtypePascal: "Arm",
		ModelPascal:           "MyModel",
		ModelTriple:           "my-org:my-module:my-model",

		SDKVersion: "0.0.0",
	}

	cCtx := newTestContext(t, map[string]any{"local": true})

	testDir := t.TempDir()
	testChdir(t, testDir)
	modulePath := filepath.Join(testDir, expectedPythonTestModule.ModuleName)

	t.Run("test setting up module directory", func(t *testing.T) {
		_, err := os.Stat(filepath.Join(testDir, expectedPythonTestModule.ModuleName))
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, setupDirectories(cCtx, expectedPythonTestModule.ModuleName), test.ShouldBeNil)
		_, err = os.Stat(filepath.Join(testDir, expectedPythonTestModule.ModuleName))
		test.That(t, err, test.ShouldBeNil)

	})

	t.Run("test render common files", func(t *testing.T) {
		setupDirectories(cCtx, expectedPythonTestModule.ModuleName)

		err := renderCommonFiles(cCtx, expectedPythonTestModule)
		test.That(t, err, test.ShouldBeNil)
		_, err = os.Stat(filepath.Join(modulePath, ".viam-gen-info"))
		test.That(t, err, test.ShouldBeNil)

		viamGenInfo, err := os.Open(filepath.Join(modulePath, ".viam-gen-info"))
		test.That(t, err, test.ShouldBeNil)
		defer viamGenInfo.Close()
		bytes, err := io.ReadAll(viamGenInfo)
		test.That(t, err, test.ShouldBeNil)
		var module moduleInputs
		err = json.Unmarshal(bytes, &module)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, module.ModuleName, test.ShouldEqual, expectedPythonTestModule.ModuleName)

		//cloud build enabled
		_, err = os.Stat(filepath.Join(modulePath, ".github"))
		test.That(t, err, test.ShouldBeNil)
		_, err = os.Stat(filepath.Join(modulePath, ".github", "workflows"))
		test.That(t, err, test.ShouldBeNil)
		_, err = os.Stat(filepath.Join(modulePath, ".github", "workflows", "deploy.yml"))
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("test copy python template", func(t *testing.T) {
		setupDirectories(cCtx, expectedPythonTestModule.ModuleName)
		err := copyLanguageTemplate(cCtx, "python", expectedPythonTestModule.ModuleName)
		test.That(t, err, test.ShouldBeNil)
		_, err = os.Stat(filepath.Join(modulePath, "src"))
		test.That(t, err, test.ShouldBeNil)
		_, err = os.Stat(filepath.Join(modulePath, "build.sh"))
		test.That(t, err, test.ShouldBeNil)
		_, err = os.Stat(filepath.Join(modulePath, "setup.sh"))
		test.That(t, err, test.ShouldBeNil)
		_, err = os.Stat(filepath.Join(modulePath, "run.sh"))
		test.That(t, err, test.ShouldBeNil)
		_, err = os.Stat(filepath.Join(modulePath, ".gitignore"))
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("test render template", func(t *testing.T) {
		setupDirectories(cCtx, expectedPythonTestModule.ModuleName)
		_ = os.Mkdir(filepath.Join(modulePath, "src"), 0755)
		_, err := os.Stat(filepath.Join(modulePath, "src"))
		test.That(t, err, test.ShouldBeNil)

		err = renderTemplate(cCtx, expectedPythonTestModule)
		test.That(t, err, test.ShouldBeNil)
		_, err = os.Stat(filepath.Join(modulePath, "requirements.txt"))
		test.That(t, err, test.ShouldBeNil)
		_, err = os.Stat(filepath.Join(modulePath, "src", "main.py"))
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("test generate stubs", func(t *testing.T) {
		setupDirectories(cCtx, expectedPythonTestModule.ModuleName)
		_ = os.Mkdir(filepath.Join(modulePath, "src"), 0755)
		_, err := os.Stat(filepath.Join(modulePath, "src"))
		test.That(t, err, test.ShouldBeNil)

		err = generateStubs(cCtx, expectedPythonTestModule)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("test generate python stubs", func(t *testing.T) {
		setupDirectories(cCtx, expectedPythonTestModule.ModuleName)
		_ = os.Mkdir(filepath.Join(modulePath, "src"), 0755)
		_, err := os.Stat(filepath.Join(modulePath, "src"))
		test.That(t, err, test.ShouldBeNil)

		err = generatePythonStubs(expectedPythonTestModule)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("test get latest sdk tag", func(t *testing.T) {
		_, err := getLatestSDKTag(cCtx, expectedPythonTestModule.Language)
		test.That(t, err, test.ShouldBeNil)
	})
	t.Run("test generate cloud build", func(t *testing.T) {
		setupDirectories(cCtx, expectedPythonTestModule.ModuleName)
		err := generateCloudBuild(cCtx, expectedPythonTestModule)
		test.That(t, err, test.ShouldBeNil)

		_, err = os.Stat(filepath.Join(modulePath, "run.sh"))
		test.That(t, err, test.ShouldNotBeNil)
		_, err = os.Stat(filepath.Join(modulePath, "build.sh"))
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("test initialize git false", func(t *testing.T) {
		err := initializeGit(cCtx, expectedPythonTestModule.ModuleName, expectedPythonTestModule.InitializeGit)
		test.That(t, err, test.ShouldBeNil)
	})
	t.Run("test initialize git true", func(t *testing.T) {
		err := initializeGit(cCtx, expectedPythonTestModule.ModuleName, true)
		test.That(t, err, test.ShouldBeNil)
		_, err = os.Stat(filepath.Join(testDir, expectedPythonTestModule.ModuleName, ".git"))
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("test create module and manifest", func(t *testing.T) {
		cCtx, ac, _, _ := setup(&inject.AppServiceClient{}, nil, &inject.BuildServiceClient{
			StartBuildFunc: func(ctx context.Context, in *v1.StartBuildRequest, opts ...grpc.CallOption) (*v1.StartBuildResponse, error) {
				return &v1.StartBuildResponse{BuildId: "xyz123"}, nil
			},
		}, nil, map[string]any{}, "token")
		err := createModuleAndManifest(cCtx, ac, expectedPythonTestModule)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("test render manifest", func(t *testing.T) {
		setupDirectories(cCtx, expectedPythonTestModule.ModuleName)
		err := renderManifest(cCtx, "moduleId", expectedPythonTestModule)
		test.That(t, err, test.ShouldBeNil)
		_, err = os.Stat(filepath.Join(testDir, expectedPythonTestModule.ModuleName, "meta.json"))
		test.That(t, err, test.ShouldBeNil)
	})
}
