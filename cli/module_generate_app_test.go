package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"go.viam.com/test"
)

func TestAppTemplateCompiles(t *testing.T) {
	testData := appTemplateData{
		ModuleName:      "testapp",
		ModuleLowercase: "testapp",
		AppName:         "testapp",
		AppType:         "single_machine",
		Namespace:       "testorg",
		Visibility:      "private",
	}

	cCtx := newTestContext(t, map[string]any{"local": true})
	gArgs, _ := getGlobalArgs(cCtx)
	globalArgs := *gArgs

	testDir := t.TempDir()
	testChdir(t, testDir)
	appPath := filepath.Join(testDir, testData.ModuleName)

	// Generate the app
	err := setupDirectories(cCtx, testData.ModuleName, globalArgs)
	test.That(t, err, test.ShouldBeNil)

	err = copyLanguageTemplate(cCtx, "app", testData.ModuleName, globalArgs)
	test.That(t, err, test.ShouldBeNil)

	err = renderAppTemplate(cCtx, testData.ModuleName, testData, globalArgs)
	test.That(t, err, test.ShouldBeNil)

	// Add a replace directive to use the local rdk so we test against the current interface
	_, thisFile, _, ok := runtime.Caller(0)
	test.That(t, ok, test.ShouldBeTrue)
	rdkRoot := filepath.Dir(filepath.Dir(thisFile))
	goModPath := filepath.Join(appPath, "go.mod")
	goMod, err := os.ReadFile(goModPath)
	test.That(t, err, test.ShouldBeNil)
	goMod = append(goMod, []byte(fmt.Sprintf("\nreplace go.viam.com/rdk => %s\n", rdkRoot))...)
	err = os.WriteFile(goModPath, goMod, 0o644)
	test.That(t, err, test.ShouldBeNil)

	goGet := exec.Command("go", "get", "github.com/erh/vmodutils@latest")
	goGet.Dir = appPath
	goGetOut, err := goGet.CombinedOutput()
	if err != nil {
		t.Fatalf("go get vmodutils failed: %v\n%s", err, goGetOut)
	}

	// Run go mod tidy to resolve dependencies
	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = appPath
	tidyOut, err := tidy.CombinedOutput()
	if err != nil {
		t.Fatalf("go mod tidy failed: %v\n%s", err, tidyOut)
	}

	// Verify the generated module.go compiles against current rdk
	build := exec.Command("go", "build", "./...")
	build.Dir = appPath
	buildOut, err := build.CombinedOutput()
	if err != nil {
		t.Fatalf("generated app module does not compile: %v\n%s", err, buildOut)
	}
}
