package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"go.viam.com/test"
)

func TestPythonSDKModuleFallsBackWhenUnderscoresAreDropped(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is not installed")
	}
	root := t.TempDir()
	writePy := func(rel, body string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(rel))
		test.That(t, os.MkdirAll(filepath.Dir(path), 0o750), test.ShouldBeNil)
		test.That(t, os.WriteFile(path, []byte(body), 0o644), test.ShouldBeNil)
	}
	for _, pkg := range []string{
		"viam",
		"viam/services",
		"viam/services/worldstatestore",
		"viam/components",
		"viam/components/movement_sensor",
		"viam/components/movementsensor",
		"viam/services/broken_service",
	} {
		writePy(pkg+"/__init__.py", "")
	}
	writePy("viam/services/worldstatestore/worldstatestore.py", "MARKER = 'stripped'\n")
	writePy("viam/components/movement_sensor/movement_sensor.py", "MARKER = 'underscored'\n")
	writePy("viam/components/movementsensor/movementsensor.py", "MARKER = 'stripped-component'\n")
	writePy("viam/services/broken_service/broken_service.py", "import does_not_exist_dep\nMARKER = 'broken'\n")

	script := filepath.Join("module_generate", "scripts", "generate_stubs.py")
	cmd := exec.CommandContext(t.Context(), "python3", "-c", `
import importlib.util, sys
spec = importlib.util.spec_from_file_location("generate_stubs", sys.argv[1])
mod = importlib.util.module_from_spec(spec)
spec.loader.exec_module(mod)
sys.path.insert(0, sys.argv[2])

world = mod.load_sdk_module("service", "world_state_store")
assert world.MARKER == "stripped", world.__name__
assert world.__name__ == "viam.services.worldstatestore.worldstatestore"

sensor = mod.load_sdk_module("component", "movement_sensor")
assert sensor.MARKER == "underscored", sensor.__name__

try:
    mod.load_sdk_module("service", "not_a_service")
    raise SystemExit("missing subtype should fail")
except ModuleNotFoundError:
    pass

try:
    mod.load_sdk_module("service", "broken_service")
    raise SystemExit("missing dependency should fail")
except ModuleNotFoundError as exc:
    assert exc.name == "does_not_exist_dep", exc.name
`, script, root)
	out, err := cmd.CombinedOutput()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, string(out), test.ShouldEqual, "")
}
