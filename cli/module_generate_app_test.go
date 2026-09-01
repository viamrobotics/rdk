package cli

import (
	"fmt"
	"os"
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
		ConfigName:      "Config",
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

	// The generated module resolves its dependencies from scratch, so every step below
	// reaches the module proxy and checksum database. runGoWithRetry absorbs the
	// intermittent stream resets those endpoints return.
	goGetOut, err := runGoWithRetry(appPath, "get", "github.com/erh/vmodutils@latest")
	if err != nil {
		t.Fatalf("go get vmodutils failed: %v\n%s", err, goGetOut)
	}

	// Run go mod tidy to resolve dependencies
	tidyOut, err := runGoWithRetry(appPath, "mod", "tidy")
	if err != nil {
		t.Fatalf("go mod tidy failed: %v\n%s", err, tidyOut)
	}

	// Verify the generated module.go compiles against current rdk
	buildOut, err := runGoWithRetry(appPath, "build", "./...")
	if err != nil {
		t.Fatalf("generated app module does not compile: %v\n%s", err, buildOut)
	}
}
