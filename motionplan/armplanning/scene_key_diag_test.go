package armplanning

import (
	"context"
	"path/filepath"
	"sort"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// orderReplayCorpus is the artifact directory holding a captured order's plan
// requests: one full espresso-drink order of 98 plans, filenames prefixed with
// their position in the order.
const orderReplayCorpus = "motionplan/order-replay/cappuccina-5fb95a4c"

// diagCorpusFiles fetches the captured order's plan payloads from the artifact
// store and returns them in recorded order (Glob sorts, and the fixed-width
// index prefix makes lexical order the recorded order). The corpus is ~100MB,
// so a machine that cannot reach the artifact store skips rather than fails -
// that keeps plain `go test` usable offline.
func diagCorpusFiles(t *testing.T) []string {
	t.Helper()
	dir, err := artifact.Path(orderReplayCorpus)
	if err != nil {
		t.Skipf("corpus %q unavailable in the artifact store: %v", orderReplayCorpus, err)
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(files), test.ShouldBeGreaterThan, 0)
	return files
}

// TestDiagSceneKeyChurn replays the captured order's 98 plan requests and
// reports how the roadmap scene key and the SDF static-set hashes actually
// behave across them.
func TestDiagSceneKeyChurn(t *testing.T) {
	files := diagCorpusFiles(t)

	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	sceneKeys := map[uint64][]int{}
	staticHashes := map[uint64][]int{}
	worldHashes := map[uint64]bool{}
	prevStatic := uint64(0)
	staticChanges := 0

	for i, f := range files {
		req, _, err := ReadRequestAndResponseFromFile(f)
		test.That(t, err, test.ShouldBeNil)
		pm, err := newPlanManager(ctx, logger, req, &PlanMeta{})
		test.That(t, err, test.ShouldBeNil)

		start := req.StartState.LinearConfiguration()
		goalPoses, err := req.Goals[0].ComputePoses(ctx, req.FrameSystem)
		test.That(t, err, test.ShouldBeNil)
		psc, err := NewPlanSegmentContext(ctx, pm.pc, start, goalPoses)
		test.That(t, err, test.ShouldBeNil)

		frames := movingFrameNamesWithDoF(psc)
		sort.Strings(frames)
		rm := &roadmap{frames: frames}
		key := pm.roadmapSceneKey(psc, rm)
		sceneKeys[key] = append(sceneKeys[key], i)

		if i == 0 {
			t.Logf("chain frames: %v; goal frames: %d", frames, len(psc.goal))
		}

		// Mirror what NewPlanSegmentContext feeds the constraint checker.
		fsg, err := referenceframe.FrameSystemGeometries(pm.pc.fs, start.ToFrameSystemInputs())
		test.That(t, err, test.ShouldBeNil)
		movingG, staticG, _ := psc.motionChains.geometries(pm.pc.fs, fsg)
		_ = movingG
		sh := spatialmath.GeometrySetHash(staticG)
		staticHashes[sh] = append(staticHashes[sh], i)
		if i > 0 && sh != prevStatic {
			staticChanges++
		}
		prevStatic = sh

		if req.ObstaclesInWorldFrame != nil {
			worldHashes[spatialmath.GeometrySetHash(req.ObstaclesInWorldFrame.Geometries())] = true
		}
	}

	t.Logf("distinct roadmap scene keys: %d", len(sceneKeys))
	for k, idxs := range sceneKeys {
		t.Logf("  key %016x -> %d plans %v", k, len(idxs), truncInts(idxs, 12))
	}
	t.Logf("distinct static-robot SDF hashes: %d (consecutive changes: %d)", len(staticHashes), staticChanges)
	for k, idxs := range staticHashes {
		t.Logf("  static %016x -> %d plans %v", k, len(idxs), truncInts(idxs, 12))
	}
	t.Logf("distinct world-obstacle SDF hashes: %d", len(worldHashes))

	// Cross-tab: how many distinct static-geometry environments hide behind
	// each roadmap scene key. >1 means edge verdicts and cached smoothed
	// trajectories are shared across physically different scenes.
	staticOf := make([]uint64, len(files))
	for sh, idxs := range staticHashes {
		for _, i := range idxs {
			staticOf[i] = sh
		}
	}
	for k, idxs := range sceneKeys {
		envs := map[uint64]bool{}
		for _, i := range idxs {
			envs[staticOf[i]] = true
		}
		t.Logf("scene key %016x: %d plans across %d distinct environments", k, len(idxs), len(envs))
	}

	// Compact per-plan assignment for visualization: scene-key ordinal and
	// environment ordinal per plan, in chronological order.
	keyOrd := map[uint64]int{}
	keyOf := make([]uint64, len(files))
	for k, idxs := range sceneKeys {
		for _, i := range idxs {
			keyOf[i] = k
		}
	}
	envOrd := map[uint64]int{}
	keySeq := make([]int, len(files))
	envSeq := make([]int, len(files))
	for i := range files {
		if _, ok := keyOrd[keyOf[i]]; !ok {
			keyOrd[keyOf[i]] = len(keyOrd)
		}
		if _, ok := envOrd[staticOf[i]]; !ok {
			envOrd[staticOf[i]] = len(envOrd)
		}
		keySeq[i] = keyOrd[keyOf[i]]
		envSeq[i] = envOrd[staticOf[i]]
	}
	t.Logf("keySeq: %v", keySeq)
	t.Logf("envSeq: %v", envSeq)
}

func truncInts(v []int, n int) []int {
	if len(v) <= n {
		return v
	}
	return v[:n]
}
