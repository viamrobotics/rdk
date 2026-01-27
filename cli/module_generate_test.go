package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	v1 "go.viam.com/api/app/build/v1"
	"go.viam.com/test"
	"google.golang.org/grpc"

	"go.viam.com/rdk/cli/module_generate/modulegen"
	modgen "go.viam.com/rdk/cli/module_generate/scripts"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

func TestGenerateModuleAction(t *testing.T) {
	t.Parallel()
	testModule := modulegen.ModuleInputs{
		ModuleName:       "my-module",
		Visibility:       moduleVisibilityPrivate,
		Namespace:        "my-org",
		Language:         "python",
		Resource:         "arm component",
		ResourceType:     "component",
		ResourceSubtype:  "arm",
		ModelName:        "my-model",
		GeneratorVersion: "0.1.0",
		GeneratedOn:      time.Now().UTC(),

		ModulePascal:          "MyModule",
		ModuleLowercase:       "mymodule",
		ResourceSubtypeAlias:  "arm",
		API:                   "rdk:component:arm",
		ResourceSubtypePascal: "Arm",
		ModelPascal:           "MyModel",
		ModelSnake:            "my-model",
		ModelTriple:           "my-org:my-module:my-model",
		ModelReadmeLink:       "my-org_my-module_my-model.md",

		SDKVersion: "0.0.0",
	}

	cCtx := newTestContext(t, map[string]any{"local": true})
	gArgs, _ := getGlobalArgs(cCtx)
	globalArgs := *gArgs

	testDir := t.TempDir()
	testChdir(t, testDir)
	modulePath := filepath.Join(testDir, testModule.ModuleName)

	t.Run("test setting up module directory", func(t *testing.T) {
		_, err := os.Stat(filepath.Join(testDir, testModule.ModuleName))
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, setupDirectories(cCtx, testModule.ModuleName, globalArgs), test.ShouldBeNil)
		_, err = os.Stat(filepath.Join(testDir, testModule.ModuleName))
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("test render common files", func(t *testing.T) {
		setupDirectories(cCtx, testModule.ModuleName, globalArgs)

		err := renderCommonFiles(cCtx, testModule, globalArgs)
		test.That(t, err, test.ShouldBeNil)
		_, err = os.Stat(filepath.Join(modulePath, ".viam-gen-info"))
		test.That(t, err, test.ShouldBeNil)

		viamGenInfo, err := os.Open(filepath.Join(modulePath, ".viam-gen-info"))
		test.That(t, err, test.ShouldBeNil)
		defer viamGenInfo.Close()
		bytes, err := io.ReadAll(viamGenInfo)
		test.That(t, err, test.ShouldBeNil)
		var module modulegen.ModuleInputs
		err = json.Unmarshal(bytes, &module)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, module.ModuleName, test.ShouldEqual, testModule.ModuleName)

		_, err = os.Stat(filepath.Join(modulePath, "README.md"))
		test.That(t, err, test.ShouldBeNil)

		readme, err := os.Open(filepath.Join(modulePath, "README.md"))
		test.That(t, err, test.ShouldBeNil)
		defer readme.Close()
		bytes, err = io.ReadAll(readme)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, string(bytes), test.ShouldContainSubstring, "Module "+testModule.ModuleName)
		test.That(t, string(bytes), test.ShouldContainSubstring, testModule.ModelReadmeLink)

		// Check that model documentation file was created
		_, err = os.Stat(filepath.Join(modulePath, testModule.ModelReadmeLink))
		test.That(t, err, test.ShouldBeNil)

		modelDoc, err := os.Open(filepath.Join(modulePath, testModule.ModelReadmeLink))
		test.That(t, err, test.ShouldBeNil)
		defer modelDoc.Close()
		bytes, err = io.ReadAll(modelDoc)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, string(bytes), test.ShouldContainSubstring, "Model "+testModule.ModelTriple)

		// cloud build enabled
		_, err = os.Stat(filepath.Join(modulePath, ".github"))
		test.That(t, err, test.ShouldBeNil)
		_, err = os.Stat(filepath.Join(modulePath, ".github", "workflows"))
		test.That(t, err, test.ShouldBeNil)
		_, err = os.Stat(filepath.Join(modulePath, ".github", "workflows", "deploy.yml"))
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("test copy python template", func(t *testing.T) {
		setupDirectories(cCtx, testModule.ModuleName, globalArgs)
		err := copyLanguageTemplate(cCtx, "python", testModule.ModuleName, globalArgs)
		test.That(t, err, test.ShouldBeNil)
		_, err = os.Stat(filepath.Join(modulePath, "src"))
		test.That(t, err, test.ShouldBeNil)
		_, err = os.Stat(filepath.Join(modulePath, "build.sh"))
		test.That(t, err, test.ShouldBeNil)
		_, err = os.Stat(filepath.Join(modulePath, "setup.sh"))
		test.That(t, err, test.ShouldBeNil)
		_, err = os.Stat(filepath.Join(modulePath, ".gitignore"))
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("test render template", func(t *testing.T) {
		setupDirectories(cCtx, testModule.ModuleName, globalArgs)
		_ = os.Mkdir(filepath.Join(modulePath, "src"), 0o755)
		_, err := os.Stat(filepath.Join(modulePath, "src"))
		test.That(t, err, test.ShouldBeNil)

		err = renderTemplate(cCtx, testModule, globalArgs)
		test.That(t, err, test.ShouldBeNil)
		_, err = os.Stat(filepath.Join(modulePath, "requirements.txt"))
		test.That(t, err, test.ShouldBeNil)
		_, err = os.Stat(filepath.Join(modulePath, "src", "main.py"))
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("test generate stubs", func(t *testing.T) {
		setupDirectories(cCtx, testModule.ModuleName, globalArgs)
		_ = os.Mkdir(filepath.Join(modulePath, "src"), 0o755)
		_, err := os.Stat(filepath.Join(modulePath, "src"))
		test.That(t, err, test.ShouldBeNil)

		err = generateStubs(cCtx, testModule, globalArgs)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("test generate go stubs", func(t *testing.T) {
		testModule.Language = "go"
		testModule.SDKVersion = "0.44.0"
		setupDirectories(cCtx, testModule.ModuleName, globalArgs)

		_, currentFile, _, ok := runtime.Caller(0)
		if !ok {
			t.Fatal("cannot get current test file path")
		}
		dir := filepath.Dir(currentFile)
		clientCode, err := os.ReadFile(filepath.Join(dir, "mock_client_arm.txt"))
		test.That(t, err, test.ShouldBeNil)

		serverClient := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(clientCode)
		}))
		defer serverClient.Close()

		resourceCode, err := os.ReadFile(filepath.Join(dir, "mock_resource_arm.txt"))
		test.That(t, err, test.ShouldBeNil)

		serverResource := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(resourceCode)
		}))
		defer serverResource.Close()
		modgen.CreateGetClientCodeRequest = func(module modulegen.ModuleInputs) (*http.Request, error) {
			return http.NewRequestWithContext(context.Background(), http.MethodGet, serverClient.URL, nil)
		}

		modgen.CreateGetResourceCodeRequest = func(module modulegen.ModuleInputs, tryagain bool) (*http.Request, error) {
			return http.NewRequestWithContext(context.Background(), http.MethodGet, serverResource.URL, nil)
		}

		err = generateGolangStubs(testModule)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("test generate python stubs", func(t *testing.T) {
		testModule.Language = "python"
		setupDirectories(cCtx, testModule.ModuleName, globalArgs)
		_ = os.Mkdir(filepath.Join(modulePath, "src"), 0o755)
		_, err := os.Stat(filepath.Join(modulePath, "src"))
		test.That(t, err, test.ShouldBeNil)

		err = generatePythonStubs(testModule)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("test create module and manifest", func(t *testing.T) {
		cCtx, ac, _, _ := setup(&inject.AppServiceClient{}, nil, &inject.BuildServiceClient{
			StartBuildFunc: func(ctx context.Context, in *v1.StartBuildRequest, opts ...grpc.CallOption) (*v1.StartBuildResponse, error) {
				return &v1.StartBuildResponse{BuildId: "xyz123"}, nil
			},
		}, map[string]any{}, "token")
		_, err := createModuleAndManifest(cCtx, ac, testModule, globalArgs)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("test render manifest", func(t *testing.T) {
		setupDirectories(cCtx, testModule.ModuleName, globalArgs)
		err := renderManifest(cCtx, "moduleId", testModule, globalArgs)
		test.That(t, err, test.ShouldBeNil)
		_, err = os.Stat(filepath.Join(testDir, testModule.ModuleName, "meta.json"))
		test.That(t, err, test.ShouldBeNil)

		manifestFile, err := os.Open(filepath.Join(testDir, testModule.ModuleName, "meta.json"))
		test.That(t, err, test.ShouldBeNil)
		defer manifestFile.Close()
		bytes, err := io.ReadAll(manifestFile)
		test.That(t, err, test.ShouldBeNil)
		var manifest ModuleManifest
		err = json.Unmarshal(bytes, &manifest)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(manifest.Models), test.ShouldEqual, 0)
	})

	t.Run("test all resources are included or excluded explicitly from module generation", func(t *testing.T) {
		// Build combined set directly from available + excluded resources
		combinedSet := make(map[string]bool)
		for _, res := range modulegen.Resources {
			combinedSet[res] = true
		}
		for _, res := range modulegen.ExcludedResources {
			combinedSet[res] = true
		}

		// Build registered set directly from registered APIs, updating the string format to match resource list format
		registeredSet := make(map[string]bool)
		for api := range resource.RegisteredAPIs() {
			resourceStr := api.SubtypeName
			if api.SubtypeName == "generic" {
				resourceStr = "generic_" + api.Type.Name
			}
			if api.SubtypeName == "input_controller" {
				resourceStr = "input"
			}
			resourceStr += " " + api.Type.Name
			registeredSet[resourceStr] = true
		}

		// Verify resources in combined list that are not in registered APIs
		for res := range combinedSet {
			if !registeredSet[res] {
				t.Errorf("Resource %q is in the module generator list (available + excluded) but is not a registered API", res)
			}
		}

		// Check: registered APIs that are not in combined list
		for res := range registeredSet {
			if !combinedSet[res] {
				t.Errorf("Registered API %q is not in the module generator list (available + excluded). It must be either added to "+
					"the Resources or ExcludedResources list in inputs.go file", res)
			}
		}
	})
}
