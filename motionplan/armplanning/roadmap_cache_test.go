package armplanning

import (
	"testing"

	"go.viam.com/test"
)

func TestClearRoadmapCache(t *testing.T) {
	rm := &roadmap{key: "clear-test-key", frames: []string{"arm"}}
	rm.built.Store(true)
	roadmapRegistry.Store(rm.key, rm)

	ClearRoadmapCache()

	_, ok := roadmapRegistry.Load(rm.key)
	test.That(t, ok, test.ShouldBeFalse)
}
