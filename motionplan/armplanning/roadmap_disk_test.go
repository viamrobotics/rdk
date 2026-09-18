package armplanning

import (
	"os"
	"path/filepath"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
)

func TestRoadmapDiskRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MOTION_ROADMAP_CACHE_DIR", dir)
	logger := logging.NewTestLogger(t)

	rm := &roadmap{
		key:       "test-key",
		frames:    []string{"arm"},
		dims:      []int{2},
		goalFrame: "arm",
		flat:      [][]float64{{0, 0}, {1, 1}, {2, 0}},
		neighbors: [][]int{{1}, {0, 2}, {1}},
		eePos:     [][3]float64{{0, 0, 0}, {10, 0, 0}, {20, 0, 0}},
	}
	rm.sceneVerdicts.Store(roadmapVerdictKey{scene: 42, a: 0, b: 1}, true)
	rm.sceneVerdicts.Store(roadmapVerdictKey{scene: 42, a: 1, b: 2}, false)
	rm.saveToDiskThrottled(logger)

	files, err := filepath.Glob(filepath.Join(dir, "roadmap-*.json"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(files), test.ShouldEqual, 1)

	loaded := &roadmap{key: "test-key", frames: []string{"arm"}}
	test.That(t, loaded.loadFromDisk(logger), test.ShouldBeTrue)
	test.That(t, loaded.flat, test.ShouldResemble, rm.flat)
	test.That(t, loaded.neighbors, test.ShouldResemble, rm.neighbors)
	test.That(t, loaded.eePos, test.ShouldResemble, rm.eePos)
	test.That(t, loaded.goalFrame, test.ShouldEqual, "arm")
	v, ok := loaded.sceneVerdicts.Load(roadmapVerdictKey{scene: 42, a: 0, b: 1})
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, v.(bool), test.ShouldBeTrue)
	v, ok = loaded.sceneVerdicts.Load(roadmapVerdictKey{scene: 42, a: 1, b: 2})
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, v.(bool), test.ShouldBeFalse)

	// A different frame layout must refuse the cache.
	mismatch := &roadmap{key: "test-key", frames: []string{"arm", "arm2"}}
	test.That(t, mismatch.loadFromDisk(logger), test.ShouldBeFalse)

	// Corrupt file must refuse and not error.
	test.That(t, os.WriteFile(files[0], []byte("{bad"), 0o644), test.ShouldBeNil)
	test.That(t, (&roadmap{key: "test-key", frames: []string{"arm"}}).loadFromDisk(logger), test.ShouldBeFalse)
}
