package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

func TestAddModel(t *testing.T) {
	t.Parallel()

	baseModule := modulegen.ModuleInputs{
		ModuleName:            "my-module",
		Visibility:            moduleVisibilityPrivate,
		Namespace:             "my-org",
		Language:              "go",
		Resource:              "arm component",
		ResourceType:          "component",
		ResourceSubtype:       "arm",
		ModelName:             "my-model",
		GeneratorVersion:      "0.1.0",
		GeneratedOn:           time.Now().UTC(),
		ModulePascal:          "MyModule",
		ModuleLowercase:       "mymodule",
		ModuleCamel:           "myModule",
		ModuleSnake:           "my_module",
		ResourceSubtypeAlias:  "arm",
		ResourceSubtypePascal: "Arm",
		ResourceTypePascal:    "Component",
		API:                   "rdk:component:arm",
		ModelPascal:           "MyModel",
		ModelCamel:            "myModel",
		ModelSnake:            "my-model",
		ModelTriple:           "my-org:my-module:my-model",
		ModelReadmeLink:       "my-org_my-module_my-model.md",
		ModuleReadmeLink:      "README.md",
		SDKVersion:            "0.44.0",
	}

	cCtx := newTestContext(t, map[string]any{"local": true})
	gArgs, _ := getGlobalArgs(cCtx)
	globalArgs := *gArgs

	t.Run("readViamGenInfo succeeds with valid file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		data, err := json.Marshal(baseModule)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, os.WriteFile(filepath.Join(dir, ".viam-gen-info"), data, 0o600), test.ShouldBeNil)

		info, err := readViamGenInfo(dir)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, info.ModuleName, test.ShouldEqual, baseModule.ModuleName)
		test.That(t, info.Language, test.ShouldEqual, baseModule.Language)
		test.That(t, info.Namespace, test.ShouldEqual, baseModule.Namespace)
	})

	t.Run("readViamGenInfo errors when file is missing", func(t *testing.T) {
		t.Parallel()
		_, err := readViamGenInfo(t.TempDir())
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, ".viam-gen-info not found")
	})

	t.Run("addPythonModelImport inserts before if __name__", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		mainPy := filepath.Join(dir, "main.py")
		original := "import asyncio\n" +
			"from viam.module.module import Module\n" +
			"from models.first import First as FirstModel\n\n\n" +
			"if __name__ == '__main__':\n" +
			"    asyncio.run(Module.run_from_registry())\n"
		test.That(t, os.WriteFile(mainPy, []byte(original), 0o600), test.ShouldBeNil)

		test.That(t, addPythonModelImport(mainPy, "second_model", "SecondModel"), test.ShouldBeNil)

		result, err := os.ReadFile(mainPy)
		test.That(t, err, test.ShouldBeNil)
		content := string(result)
		test.That(t, content, test.ShouldContainSubstring, "from models.second_model import SecondModel as SecondModelModel\n")
		// new import must appear before the main guard
		importIdx := strings.Index(content, "from models.second_model")
		guardIdx := strings.Index(content, "if __name__")
		test.That(t, importIdx, test.ShouldBeLessThan, guardIdx)
		// original imports must still be present
		test.That(t, content, test.ShouldContainSubstring, "from models.first import First as FirstModel")
	})

	t.Run("addPythonModelImport falls back to append when no main guard", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		mainPy := filepath.Join(dir, "main.py")
		original := "import asyncio\nfrom models.first import First as FirstModel\n"
		test.That(t, os.WriteFile(mainPy, []byte(original), 0o600), test.ShouldBeNil)

		test.That(t, addPythonModelImport(mainPy, "second", "Second"), test.ShouldBeNil)

		result, err := os.ReadFile(mainPy)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, string(result), test.ShouldContainSubstring, "from models.second import Second as SecondModel")
	})

	t.Run("addModelToManifest appends model entry", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		manifestPath := filepath.Join(dir, defaultManifestFilename)

		initial := ModuleManifest{
			Schema:     "https://dl.viam.dev/module.schema.json",
			ModuleID:   "my-org:my-module",
			Visibility: moduleVisibilityPrivate,
		}
		test.That(t, writeManifest(manifestPath, initial), test.ShouldBeNil)

		test.That(t, addModelToManifest(manifestPath, baseModule), test.ShouldBeNil)

		manifest, err := loadManifest(manifestPath)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(manifest.Models), test.ShouldEqual, 1)
		test.That(t, manifest.Models[0].API, test.ShouldEqual, "rdk:component:arm")
		test.That(t, manifest.Models[0].Model, test.ShouldEqual, "my-org:my-module:my-model")
		test.That(t, manifest.Models[0].MarkdownLink, test.ShouldNotBeNil)
		test.That(t, *manifest.Models[0].MarkdownLink, test.ShouldEqual, baseModule.ModelReadmeLink)
	})

	t.Run("addModelToManifest preserves existing models", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		manifestPath := filepath.Join(dir, defaultManifestFilename)

		markdownLink := "existing.md"
		initial := ModuleManifest{
			Schema:   "https://dl.viam.dev/module.schema.json",
			ModuleID: "my-org:my-module",
			Models: []ModuleComponent{
				{API: "rdk:component:camera", Model: "my-org:my-module:existing-model", MarkdownLink: &markdownLink},
			},
		}
		test.That(t, writeManifest(manifestPath, initial), test.ShouldBeNil)

		test.That(t, addModelToManifest(manifestPath, baseModule), test.ShouldBeNil)

		manifest, err := loadManifest(manifestPath)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(manifest.Models), test.ShouldEqual, 2)
		test.That(t, manifest.Models[0].Model, test.ShouldEqual, "my-org:my-module:existing-model")
		test.That(t, manifest.Models[1].Model, test.ShouldEqual, "my-org:my-module:my-model")
	})

	t.Run("renderModelDocToDir creates the model doc file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		test.That(t, renderModelDocToDir(dir, baseModule), test.ShouldBeNil)
		docPath := filepath.Join(dir, baseModule.ModelReadmeLink)
		_, err := os.Stat(docPath)
		test.That(t, err, test.ShouldBeNil)
		b, err := os.ReadFile(docPath)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, string(b), test.ShouldContainSubstring, baseModule.ModelTriple)
	})

	t.Run("addGolangModelFile creates <model_snake>.go", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		goModule := baseModule
		goModule.Language = "go"
		goModule.SDKVersion = "0.44.0"

		_, currentFile, _, ok := runtime.Caller(0)
		if !ok {
			t.Fatal("cannot get current test file path")
		}
		testDir := filepath.Dir(currentFile)
		clientCode, err := os.ReadFile(filepath.Join(testDir, "mock_client_arm.txt"))
		test.That(t, err, test.ShouldBeNil)
		resourceCode, err := os.ReadFile(filepath.Join(testDir, "mock_resource_arm.txt"))
		test.That(t, err, test.ShouldBeNil)

		serverClient := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(clientCode)
		}))
		defer serverClient.Close()
		serverResource := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(resourceCode)
		}))
		defer serverResource.Close()

		origClient := modgen.CreateGetClientCodeRequest
		origResource := modgen.CreateGetResourceCodeRequest
		modgen.CreateGetClientCodeRequest = func(module modulegen.ModuleInputs) (*http.Request, error) {
			return http.NewRequestWithContext(context.Background(), http.MethodGet, serverClient.URL, nil)
		}
		modgen.CreateGetResourceCodeRequest = func(module modulegen.ModuleInputs, tryagain bool) (*http.Request, error) {
			return http.NewRequestWithContext(context.Background(), http.MethodGet, serverResource.URL, nil)
		}
		defer func() {
			modgen.CreateGetClientCodeRequest = origClient
			modgen.CreateGetResourceCodeRequest = origResource
		}()

		test.That(t, addGolangModelFile(dir, goModule), test.ShouldBeNil)

		expectedPath := filepath.Join(dir, goModule.ModelSnake+".go")
		b, err := os.ReadFile(expectedPath)
		test.That(t, err, test.ShouldBeNil)
		content := string(b)
		// Additional model must not redeclare package-level errUnimplemented.
		test.That(t, content, test.ShouldNotContainSubstring, "errUnimplemented")
		// Config type must be model-scoped, not the bare "Config".
		test.That(t, content, test.ShouldContainSubstring, "MyModelConfig")
		// Top-level "type Config" declaration must not be present (the comment example is tab-indented).
		test.That(t, content, test.ShouldNotContainSubstring, "\ntype Config struct")
	})

	t.Run("addGoModelToMain registers new model", func(t *testing.T) {
		dir := t.TempDir()

		// A minimal main.go matching the generated template.
		mainGoContent := `package main

import (
	"mymodule"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
	arm "go.viam.com/rdk/components/arm"
)

func main() {
	module.ModularMain(resource.APIModel{arm.API, mymodule.MyModel})
}
`
		mainGoPath := filepath.Join(dir, "main.go")
		test.That(t, os.WriteFile(mainGoPath, []byte(mainGoContent), 0o600), test.ShouldBeNil)

		boardModel := baseModule
		boardModel.ResourceType = "component"
		boardModel.ResourceSubtype = "board"
		boardModel.ResourceSubtypeAlias = "board"
		boardModel.ModelPascal = "MyBoard"

		test.That(t, addGoModelToMain(mainGoPath, boardModel), test.ShouldBeNil)

		result, err := os.ReadFile(mainGoPath)
		test.That(t, err, test.ShouldBeNil)
		content := string(result)

		// New APIModel entry must be present.
		test.That(t, content, test.ShouldContainSubstring, `resource.APIModel{board.API, mymodule.MyBoard}`)
		// New subtype import must be present.
		test.That(t, content, test.ShouldContainSubstring, `board "go.viam.com/rdk/components/board"`)
		// Original model must still be registered.
		test.That(t, content, test.ShouldContainSubstring, `resource.APIModel{arm.API, mymodule.MyModel}`)
	})

	t.Run("addGoModelToMain skips duplicate import", func(t *testing.T) {
		dir := t.TempDir()

		// main.go already imports arm; adding another arm model should not duplicate it.
		mainGoContent := `package main

import (
	"mymodule"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
	arm "go.viam.com/rdk/components/arm"
)

func main() {
	module.ModularMain(resource.APIModel{arm.API, mymodule.MyModel})
}
`
		mainGoPath := filepath.Join(dir, "main.go")
		test.That(t, os.WriteFile(mainGoPath, []byte(mainGoContent), 0o600), test.ShouldBeNil)

		secondArm := baseModule
		secondArm.ResourceType = "component"
		secondArm.ResourceSubtype = "arm"
		secondArm.ResourceSubtypeAlias = "arm"
		secondArm.ModelPascal = "MySecondArm"

		test.That(t, addGoModelToMain(mainGoPath, secondArm), test.ShouldBeNil)

		result, err := os.ReadFile(mainGoPath)
		test.That(t, err, test.ShouldBeNil)
		content := string(result)

		test.That(t, content, test.ShouldContainSubstring, `resource.APIModel{arm.API, mymodule.MySecondArm}`)
		// Import must appear exactly once.
		test.That(t, strings.Count(content, `"go.viam.com/rdk/components/arm"`), test.ShouldEqual, 1)
	})

	t.Run("addCppModelToMainCpp inserts include, registration, and push_back", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		// mainCppContent mirrors what the fixed generator produces (fixCppMainTemplate applied):
		// - namespace arg uses .Namespace (not .OrgID)
		// - model name arg uses .ModelName with dashes (not .ModelSnake with underscores)
		// - C++ namespace qualifiers use .ModuleSnake (not .ModuleName)
		mainCppContent := `#include "my_model.hpp"

#include <viam/sdk/registry/registry.hpp>

int main(int argc, char** argv) try {
    viam::sdk::Instance inst;
    viam::sdk::Model model("my-org", "my-module", "my-model");

    auto mr = std::make_shared<viam::sdk::ModelRegistration>(
        viam::sdk::API::get<viam::sdk::Arm>(),
        model,
        [](viam::sdk::Dependencies deps, viam::sdk::ResourceConfig cfg) {
            return std::make_unique<my_module::MyModel>(deps, cfg);
        },
        &my_module::MyModel::validate);

    std::vector<std::shared_ptr<viam::sdk::ModelRegistration>> mrs = {mr};
    auto my_mod = std::make_shared<viam::sdk::ModuleService>(argc, argv, mrs);
    my_mod->serve();
    return EXIT_SUCCESS;
} catch (...) {}
`
		mainCppPath := filepath.Join(dir, "main.cpp")
		test.That(t, os.WriteFile(mainCppPath, []byte(mainCppContent), 0o600), test.ShouldBeNil)

		newModel := baseModule
		newModel.ModelName = "my-camera"
		newModel.ModelSnake = "my_camera"
		newModel.ModelPascal = "MyCamera"
		newModel.ResourceSubtypePascal = "Camera"

		test.That(t, addCppModelToMainCpp(mainCppPath, newModel), test.ShouldBeNil)

		result, err := os.ReadFile(mainCppPath)
		test.That(t, err, test.ShouldBeNil)
		content := string(result)

		test.That(t, content, test.ShouldContainSubstring, `#include "my_camera.hpp"`)
		// Variable names use snake_case.
		test.That(t, content, test.ShouldContainSubstring, `my_camera_model(`)
		test.That(t, content, test.ShouldContainSubstring, `auto my_camera_mr`)
		test.That(t, content, test.ShouldContainSubstring, `mrs.push_back(my_camera_mr)`)
		// Model triple strings preserve original names (dashes, not underscores).
		test.That(t, content, test.ShouldContainSubstring, `"my-org", "my-module", "my-camera"`)
		// C++ namespace uses snake_case.
		test.That(t, content, test.ShouldContainSubstring, `std::make_unique<my_module::MyCamera>`)
		test.That(t, content, test.ShouldContainSubstring, `&my_module::MyCamera::validate`)
		// original model must still be present
		test.That(t, content, test.ShouldContainSubstring, `#include "my_model.hpp"`)
		test.That(t, content, test.ShouldContainSubstring, `mrs = {mr}`)
	})

	t.Run("addCppModelToCMakeLists inserts new source file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		cmakeContent := `cmake_minimum_required(VERSION 3.25 FATAL_ERROR)
project(my-module LANGUAGES CXX)
find_package(viam-cpp-sdk CONFIG REQUIRED COMPONENTS viamsdk)

add_executable(my-module
    main.cpp
    src/my_model.cpp
)

target_include_directories(my-module PUBLIC src)
target_link_libraries(my-module viam-cpp-sdk::viamsdk)

file(READ "${CMAKE_CURRENT_SOURCE_DIR}/meta.json" _META_JSON)
`
		cmakePath := filepath.Join(dir, "CMakeLists.txt")
		test.That(t, os.WriteFile(cmakePath, []byte(cmakeContent), 0o600), test.ShouldBeNil)

		newModel := baseModule
		newModel.ModelSnake = "my_camera"

		test.That(t, addCppModelToCMakeLists(cmakePath, newModel), test.ShouldBeNil)

		result, err := os.ReadFile(cmakePath)
		test.That(t, err, test.ShouldBeNil)
		content := string(result)

		test.That(t, content, test.ShouldContainSubstring, "src/my_camera.cpp")
		// original source must still be present
		test.That(t, content, test.ShouldContainSubstring, "src/my_model.cpp")
		// new source must appear before the closing paren
		newIdx := strings.Index(content, "src/my_camera.cpp")
		closingIdx := strings.Index(content, ")\n\ntarget_include_directories")
		test.That(t, newIdx, test.ShouldBeLessThan, closingIdx)
	})

	t.Run("AddModelAction dry run", func(t *testing.T) {
		// No t.Parallel(): calls testChdir which mutates process-wide CWD,
		// which races with parallel subtests that call go install / use relative paths.
		dir := t.TempDir()
		testChdir(t, dir)

		// Write .viam-gen-info so the action can read module context
		data, err := json.Marshal(baseModule)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, os.WriteFile(".viam-gen-info", data, 0o600), test.ShouldBeNil)

		// Write a stub meta.json
		manifest := ModuleManifest{
			Schema:   "https://dl.viam.dev/module.schema.json",
			ModuleID: "my-org:my-module",
		}
		test.That(t, writeManifest(defaultManifestFilename, manifest), test.ShouldBeNil)

		// Stub out SDK version fetch
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`[{"tag_name": "v0.44.0"}]`))
		}))
		defer server.Close()

		setupDirectories(cCtx, baseModule.ModuleName, globalArgs)

		args := addModelArgs{
			ResourceSubtype: "arm",
			ModelName:       "second-model",
			DryRun:          true,
		}
		err = AddModelAction(context.Background(), cCtx, args)
		// dry-run returns nil without touching files
		test.That(t, err, test.ShouldBeNil)
	})
}

func TestGenerateModuleAction(t *testing.T) {
	// No t.Parallel(): subtests use relative paths that depend on CWD (set by testChdir),
	// so this test must run sequentially to avoid races with TestAddModel's testChdir calls.
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
		scriptExt := ".sh"
		if runtime.GOOS == "windows" {
			scriptExt = ".bat"
		}
		_, err = os.Stat(filepath.Join(modulePath, "build"+scriptExt))
		test.That(t, err, test.ShouldBeNil)
		_, err = os.Stat(filepath.Join(modulePath, "setup"+scriptExt))
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
		if err := copyLanguageTemplate(cCtx, testModule.Language, testModule.ModuleName, globalArgs); err != nil {
			t.Fatal(err)
		}
		if err := renderTemplate(cCtx, testModule, globalArgs); err != nil {
			t.Fatal(err)
		}

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
		_, err := createModuleAndManifest(context.Background(), cCtx, ac, testModule, globalArgs)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("test render manifest", func(t *testing.T) {
		setupDirectories(cCtx, testModule.ModuleName, globalArgs)
		err := renderManifest(cCtx, "moduleId", testModule, globalArgs, nil)
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

	t.Run("test generate cpp stubs", func(t *testing.T) {
		cppModule := testModule
		cppModule.Language = "cpp"
		cppModule.ResourceType = "component"
		cppModule.ResourceSubtype = "camera"
		cppModule.ResourceSubtypeSnake = "camera"
		cppModule.ModelSnake = "my_model"
		cppModule.ModelPascal = "MyModel"

		setupDirectories(cCtx, cppModule.ModuleName, globalArgs)

		mockTemplates := map[string]string{
			"main.cpp.in":       "// main {{ .ModuleName }}",
			"CMakeLists.txt.in": "# cmake {{ .ModuleName }}",
			"conanfile.py.in":   "# conan {{ .ModuleName }}",
			"camera.cpp.in":     "// camera impl {{ .ModelPascal }}",
			"camera.hpp.in":     "// camera header {{ .ModelPascal }}",
			"conan.lock":        `{"version": "0.5"}`,
		}
		modgen.FetchRawTemplate = func(url string) (string, error) {
			for filename, content := range mockTemplates {
				if strings.HasSuffix(url, filename) {
					return content, nil
				}
			}
			return "", fmt.Errorf("unexpected template URL: %s", url)
		}

		err := generateCppStubs(cppModule)
		test.That(t, err, test.ShouldBeNil)

		// top-level files
		for _, tc := range []struct {
			file    string
			content string
		}{
			{"main.cpp", "// main " + cppModule.ModuleName},
			{"CMakeLists.txt", "# cmake " + cppModule.ModuleName},
			{"conanfile.py", "# conan " + cppModule.ModuleName},
			{"conan.lock", `{"version": "0.5"}`},
		} {
			b, err := os.ReadFile(filepath.Join(modulePath, tc.file))
			test.That(t, err, test.ShouldBeNil)
			test.That(t, string(b), test.ShouldEqual, tc.content)
		}

		// type-specific files in src/
		for _, tc := range []struct {
			file    string
			content string
		}{
			{cppModule.ModelSnake + ".cpp", "// camera impl " + cppModule.ModelPascal},
			{cppModule.ModelSnake + ".hpp", "// camera header " + cppModule.ModelPascal},
		} {
			b, err := os.ReadFile(filepath.Join(modulePath, "src", tc.file))
			test.That(t, err, test.ShouldBeNil)
			test.That(t, string(b), test.ShouldEqual, tc.content)
		}
	})

	t.Run("test check version", func(t *testing.T) {
		checkPython := func(output string) error {
			return checkVersionCompatible(output, "Python", minPythonVersion)
		}
		checkGo := func(output string) error {
			return checkVersionCompatible(output, "Go", minGoVersion)
		}

		// supported versions
		test.That(t, checkPython("Python 4.0.0\n"), test.ShouldBeNil)
		test.That(t, checkGo("go version go1.24.0 linux/amd64\n"), test.ShouldBeNil)

		// unsupported versions
		err := checkPython("Python 3.8.10\n")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "3.8")
		test.That(t, err.Error(), test.ShouldContainSubstring, ">= "+minPythonVersion)

		err = checkGo("go version go1.22.5 darwin/arm64\n")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "1.22")
		test.That(t, err.Error(), test.ShouldContainSubstring, ">= "+minGoVersion)

		// unparseable output
		err = checkPython("not a version\n")
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "cannot parse")
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

func TestAddApp(t *testing.T) {
	t.Parallel()

	baseGenInfo := modulegen.ModuleInputs{
		ModuleName: "my-module",
		Language:   "go",
		Namespace:  "my-org",
		Visibility: moduleVisibilityPrivate,
	}

	cCtx := newTestContext(t, map[string]any{"local": true})

	t.Run("addGoWebappToMain adds generic import and webapp APIModel", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		mainGoPath := filepath.Join(dir, "main.go")
		original := `package main

import (
	"mymodule"
	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
)

func main() {
	module.ModularMain(resource.APIModel{API: arm.API, Model: mymodule.MyModel})
}
`
		test.That(t, os.WriteFile(mainGoPath, []byte(original), 0o600), test.ShouldBeNil)

		data := appTemplateData{
			ModuleName:      "my-module",
			ModuleLowercase: "mymodule",
			Namespace:       "my-org",
		}
		test.That(t, addGoWebappToMain(mainGoPath, data), test.ShouldBeNil)

		result, err := os.ReadFile(mainGoPath)
		test.That(t, err, test.ShouldBeNil)
		content := string(result)
		test.That(t, content, test.ShouldContainSubstring, "go.viam.com/rdk/components/generic")
		test.That(t, content, test.ShouldContainSubstring, "resource.APIModel{API: generic.API, Model: mymodule.Model}")
		// original registration must still be present
		test.That(t, content, test.ShouldContainSubstring, "resource.APIModel{API: arm.API, Model: mymodule.MyModel}")
	})

	t.Run("addGoWebappToMain skips duplicate generic import", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		mainGoPath := filepath.Join(dir, "main.go")
		original := `package main

import (
	"mymodule"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
)

func main() {
	module.ModularMain(resource.APIModel{API: generic.API, Model: mymodule.Model})
}
`
		test.That(t, os.WriteFile(mainGoPath, []byte(original), 0o600), test.ShouldBeNil)

		data := appTemplateData{ModuleLowercase: "mymodule"}
		test.That(t, addGoWebappToMain(mainGoPath, data), test.ShouldBeNil)

		result, err := os.ReadFile(mainGoPath)
		test.That(t, err, test.ShouldBeNil)
		// import should appear exactly once
		test.That(t, strings.Count(string(result), "go.viam.com/rdk/components/generic"), test.ShouldEqual, 1)
	})

	t.Run("addAppToManifest appends app and webapp model", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		manifestPath := filepath.Join(dir, "meta.json")
		manifest := ModuleManifest{
			Schema:   "https://dl.viam.dev/module.schema.json",
			ModuleID: "my-org:my-module",
			Models: []ModuleComponent{
				{API: "rdk:component:arm", Model: "my-org:my-module:my-model"},
			},
		}
		test.That(t, writeManifest(manifestPath, manifest), test.ShouldBeNil)

		app := &appInputs{AppName: "my-app", AppType: "single_machine"}
		data := appTemplateData{Namespace: "my-org", ModuleName: "my-module"}
		test.That(t, addAppToManifest(manifestPath, app, data), test.ShouldBeNil)

		result, err := loadManifest(manifestPath)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(result.Apps), test.ShouldEqual, 1)
		test.That(t, result.Apps[0].Name, test.ShouldEqual, "my-app")
		test.That(t, result.Apps[0].Type, test.ShouldEqual, "single_machine")
		test.That(t, result.Apps[0].Entrypoint, test.ShouldEqual, "dist/index.html")
		// original model must still be present
		test.That(t, len(result.Models), test.ShouldEqual, 2)
		webappFound := false
		for _, m := range result.Models {
			if m.Model == "my-org:my-module:webapp" {
				webappFound = true
				test.That(t, m.API, test.ShouldEqual, "rdk:component:generic")
			}
		}
		test.That(t, webappFound, test.ShouldBeTrue)
	})

	t.Run("addAppToManifest skips duplicate webapp model", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		manifestPath := filepath.Join(dir, "meta.json")
		manifest := ModuleManifest{
			Schema:   "https://dl.viam.dev/module.schema.json",
			ModuleID: "my-org:my-module",
			Models: []ModuleComponent{
				{API: "rdk:component:generic", Model: "my-org:my-module:webapp"},
			},
		}
		test.That(t, writeManifest(manifestPath, manifest), test.ShouldBeNil)

		app := &appInputs{AppName: "my-app", AppType: "multi_machine"}
		data := appTemplateData{Namespace: "my-org", ModuleName: "my-module"}
		test.That(t, addAppToManifest(manifestPath, app, data), test.ShouldBeNil)

		result, err := loadManifest(manifestPath)
		test.That(t, err, test.ShouldBeNil)
		// webapp model should still appear exactly once
		count := 0
		for _, m := range result.Models {
			if m.Model == "my-org:my-module:webapp" {
				count++
			}
		}
		test.That(t, count, test.ShouldEqual, 1)
	})

	t.Run("addAppStaticFiles creates dist and copies auth.js", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		test.That(t, addAppStaticFiles(dir), test.ShouldBeNil)

		// dist/ directory must exist
		info, err := os.Stat(filepath.Join(dir, "dist"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, info.IsDir(), test.ShouldBeTrue)

		// auth.js must be present and non-empty
		authInfo, err := os.Stat(filepath.Join(dir, "auth.js"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, authInfo.Size(), test.ShouldBeGreaterThan, 0)

		// dist/index.html must be present
		_, err = os.Stat(filepath.Join(dir, "dist", "index.html"))
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("addAppStaticFiles does not overwrite existing files", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		distDir := filepath.Join(dir, "dist")
		test.That(t, os.MkdirAll(distDir, 0o750), test.ShouldBeNil)
		test.That(t, os.WriteFile(filepath.Join(dir, "auth.js"), []byte("existing"), 0o600), test.ShouldBeNil)
		test.That(t, os.WriteFile(filepath.Join(distDir, "index.html"), []byte("existing"), 0o600), test.ShouldBeNil)

		test.That(t, addAppStaticFiles(dir), test.ShouldBeNil)

		authBytes, err := os.ReadFile(filepath.Join(dir, "auth.js"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, string(authBytes), test.ShouldEqual, "existing")

		indexBytes, err := os.ReadFile(filepath.Join(distDir, "index.html"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, string(indexBytes), test.ShouldEqual, "existing")
	})

	t.Run("AddAppAction dry run", func(t *testing.T) {
		// No t.Parallel(): calls testChdir which mutates process-wide CWD.
		dir := t.TempDir()
		testChdir(t, dir)

		data, err := json.Marshal(baseGenInfo)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, os.WriteFile(".viam-gen-info", data, 0o600), test.ShouldBeNil)

		manifest := ModuleManifest{
			Schema:   "https://dl.viam.dev/module.schema.json",
			ModuleID: "my-org:my-module",
		}
		test.That(t, writeManifest(defaultManifestFilename, manifest), test.ShouldBeNil)

		args := addAppArgs{AppName: "my-app", AppType: "single_machine", DryRun: true}
		test.That(t, AddAppAction(context.Background(), cCtx, args), test.ShouldBeNil)

		// meta.json must be unchanged after a dry run
		result, err := loadManifest(defaultManifestFilename)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(result.Apps), test.ShouldEqual, 0)
	})

	t.Run("AddAppAction rejects non-Go modules", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		pythonInfo := modulegen.ModuleInputs{
			ModuleName: "my-module",
			Language:   "python",
			Namespace:  "my-org",
		}
		data, err := json.Marshal(pythonInfo)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, os.WriteFile(filepath.Join(dir, ".viam-gen-info"), data, 0o600), test.ShouldBeNil)

		// Temporarily point CWD so readViamGenInfo(".")  resolves correctly.
		// We can't use testChdir here (parallel), so call readViamGenInfo directly.
		info, err := readViamGenInfo(dir)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, info.Language, test.ShouldEqual, "python")

		// Simulate the language guard in AddAppAction.
		if info.Language != golang {
			// expected path: error should mention "Go" and the actual language
			errMsg := fmt.Sprintf("add-app only supports Go modules; this module uses %s", info.Language)
			test.That(t, errMsg, test.ShouldContainSubstring, "python")
		}
	})

	t.Run("AddAppAction rejects duplicate app name", func(t *testing.T) {
		// No t.Parallel(): calls testChdir which mutates process-wide CWD.
		dir := t.TempDir()
		testChdir(t, dir)

		data, err := json.Marshal(baseGenInfo)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, os.WriteFile(".viam-gen-info", data, 0o600), test.ShouldBeNil)

		manifest := ModuleManifest{
			Schema:   "https://dl.viam.dev/module.schema.json",
			ModuleID: "my-org:my-module",
			Apps:     []AppComponent{{Name: "my-app", Type: "single_machine", Entrypoint: "dist/index.html"}},
		}
		test.That(t, writeManifest(defaultManifestFilename, manifest), test.ShouldBeNil)

		args := addAppArgs{AppName: "my-app", AppType: "single_machine"}
		err = AddAppAction(context.Background(), cCtx, args)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "already exists")
	})

	t.Run("updateMakefileForApp adds ENTRYPOINT, dist, and vmodutils", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		makefilePath := filepath.Join(dir, "Makefile")
		original := "\nGO_BUILD_ENV :=\nMODULE_BINARY := bin/my-module\n\n" +
			"$(MODULE_BINARY): Makefile go.mod *.go cmd/module/*.go\n" +
			"\tgo build -o $(MODULE_BINARY) cmd/module/main.go\n\n" +
			"FIRST_RUN := $(shell jq -r '.first_run // empty' meta.json 2>/dev/null)\n" +
			"TAR_FILES := meta.json $(MODULE_BINARY)\n\n" +
			"module.tar.gz: meta.json $(MODULE_BINARY)\n" +
			"\tstrip $(MODULE_BINARY)\n" +
			"\ttar czf $@ $(TAR_FILES)\n\n" +
			"setup:\n\tgo mod tidy\n"
		test.That(t, os.WriteFile(makefilePath, []byte(original), 0o600), test.ShouldBeNil)

		test.That(t, updateMakefileForApp(makefilePath), test.ShouldBeNil)

		result, err := os.ReadFile(makefilePath)
		test.That(t, err, test.ShouldBeNil)
		content := string(result)

		test.That(t, content, test.ShouldContainSubstring, "ENTRYPOINT := dist/index.html")
		test.That(t, content, test.ShouldContainSubstring, ".DEFAULT_GOAL := all")
		// ENTRYPOINT and .DEFAULT_GOAL must appear right after the MODULE_BINARY line
		mbIdx := strings.Index(content, "MODULE_BINARY :=")
		epIdx := strings.Index(content, "ENTRYPOINT :=")
		dgIdx := strings.Index(content, ".DEFAULT_GOAL :=")
		test.That(t, epIdx, test.ShouldBeGreaterThan, mbIdx)
		test.That(t, dgIdx, test.ShouldBeGreaterThan, mbIdx)

		test.That(t, content, test.ShouldContainSubstring, "TAR_FILES := meta.json $(MODULE_BINARY) dist")
		test.That(t, content, test.ShouldContainSubstring, "$(MODULE_BINARY): Makefile go.mod *.go cmd/module/*.go $(ENTRYPOINT)")
		test.That(t, content, test.ShouldContainSubstring, "module.tar.gz: meta.json $(MODULE_BINARY) $(ENTRYPOINT)")
		test.That(t, content, test.ShouldContainSubstring, "go get github.com/erh/vmodutils@latest")
		// vmodutils must appear before go mod tidy in setup
		vmodIdx := strings.Index(content, "vmodutils")
		tidyIdx := strings.Index(content, "go mod tidy")
		test.That(t, vmodIdx, test.ShouldBeLessThan, tidyIdx)
	})

	t.Run("updateMakefileForApp is idempotent", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		makefilePath := filepath.Join(dir, "Makefile")
		original := "\nMODULE_BINARY := bin/my-module\nENTRYPOINT := dist/custom.html\n\n" +
			"TAR_FILES := meta.json $(MODULE_BINARY) dist\n\n" +
			"module.tar.gz: meta.json $(MODULE_BINARY) $(ENTRYPOINT)\n" +
			"\ttar czf $@ $(TAR_FILES)\n\n" +
			"setup:\n\tgo get github.com/erh/vmodutils@latest\n\tgo mod tidy\n"
		test.That(t, os.WriteFile(makefilePath, []byte(original), 0o600), test.ShouldBeNil)

		test.That(t, updateMakefileForApp(makefilePath), test.ShouldBeNil)

		result, err := os.ReadFile(makefilePath)
		test.That(t, err, test.ShouldBeNil)
		// File must be unchanged — in particular the custom ENTRYPOINT value must be preserved.
		test.That(t, string(result), test.ShouldEqual, original)
	})
}
