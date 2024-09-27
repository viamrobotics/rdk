package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"go.viam.com/test"
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
		EnableCloudBuild: false,
		InitializeGit:    false,
		GeneratorVersion: "0.1.0",
		GeneratedOn:      time.Now().UTC(),

		ModulePascal:          "MyModule",
		API:                   "rdk:component:arm",
		ResourceSubtypePascal: "Arm",
		ModelPascal:           "MyModel",
		ModelTriple:           "my-org:my-module:my-model",
	}

	cCtx := newTestContext(t, map[string]any{"local": true})

	testDir := t.TempDir()
	testChdir(t, testDir)

	t.Run("test prompt user", func(t *testing.T) {
		module, form := promptUser()
		tm := teatest.NewTestModel(t, form)

		//input module name
		teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
			return bytes.Contains(out, []byte("alphanumeric characters"))
		}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(time.Millisecond*10))
		tm.Type("my-module")
		tm.Send(tea.KeyMsg{
			Type: tea.KeyTab,
		})
		//language
		teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
			return bytes.Contains(out, []byte("language"))
		}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(time.Millisecond*10))
		tm.Send(tea.KeyMsg{
			Type: tea.KeyEnter,
		})
		//visibility
		teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
			return bytes.Contains(out, []byte("Visibility"))
		}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(time.Millisecond*10))

		tm.Send(tea.KeyMsg{
			Type: tea.KeyEnter,
		})

		//namespace/orgid
		teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
			return bytes.Contains(out, []byte(" ID"))
		}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(time.Millisecond*10))

		tm.Type("my-org")
		tm.Send(tea.KeyMsg{
			Type: tea.KeyEnter,
		})

		//choose resource
		teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
			return bytes.Contains(out, []byte(" resource"))
		}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(time.Millisecond*10))

		tm.Send(tea.KeyMsg{
			Type: tea.KeyEnter,
		})

		//input model name
		teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
			return bytes.Contains(out, []byte("model name"))
		}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(time.Millisecond*10))
		tm.Type("my-model")
		tm.Send(tea.KeyMsg{
			Type: tea.KeyEnter,
		})

		//cloud build
		teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
			return bytes.Contains(out, []byte("cloud build"))
		}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(time.Millisecond*10))
		tm.Send(tea.KeyMsg{
			Type: tea.KeyEnter,
		})

		//git repo
		teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
			return bytes.Contains(out, []byte(" git repository"))
		}, teatest.WithDuration(5*time.Second), teatest.WithCheckInterval(time.Millisecond*10))
		tm.Send(tea.KeyMsg{
			Type: tea.KeyEnter,
		})

		err := tm.Quit()
		test.That(t, err, test.ShouldBeNil)

		fillAdditionalInfo(module)
		expectedPythonTestModule.GeneratedOn = module.GeneratedOn
		test.That(t, module.ModuleName, test.ShouldEqual, "my-module")
		test.That(t, module.Namespace, test.ShouldEqual, "my-org")
		test.That(t, module.ModelName, test.ShouldEqual, "my-model")
		test.That(t, *module, test.ShouldResemble, expectedPythonTestModule)
		time.Sleep(time.Second)
		syscall.Kill(os.Getpid(), syscall.SIGINT)

	})

	t.Run("test setting up module directory", func(t *testing.T) {
		_, err := os.Stat(filepath.Join(testDir, expectedPythonTestModule.ModuleName))
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, setupDirectories(cCtx, expectedPythonTestModule.ModuleName), test.ShouldBeNil)
		_, err = os.Stat(filepath.Join(testDir, expectedPythonTestModule.ModuleName))
		test.That(t, err, test.ShouldBeNil)

	})

	t.Run("test render common files", func(t *testing.T) {
		err := renderCommonFiles(cCtx, &expectedPythonTestModule)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("test copy python template", func(t *testing.T) {
		err := copyLanguageTemplate(cCtx, "python", expectedPythonTestModule.ModuleName)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("test render template", func(t *testing.T) {
		err := renderTemplate(cCtx, &expectedPythonTestModule)
		test.That(t, err, test.ShouldBeNil)
	})

	// t.Run("test generate stubs", func(t *testing.T) {
	// 	err := generateStubs(cCtx, &expectedPythonTestModule)
	// 	test.That(t, err, test.ShouldBeNil)
	// })

}
