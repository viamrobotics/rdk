package armplanning

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// TestDiagSceneKeyChurn replays the captured order's 98 plan requests and
// reports how the roadmap scene key and the SDF static-set hashes actually
// behave across them. Diagnostic only; skipped unless ORDER_EXPORT_DIR is set.
func TestDiagSceneKeyChurn(t *testing.T) {
	dir := os.Getenv("ORDER_EXPORT_DIR")
	if dir == "" {
		t.Skip("set ORDER_EXPORT_DIR to the captured order's data directory")
	}
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && filepath.Ext(path) == ".json" {
			files = append(files, path)
		}
		return nil
	})
	test.That(t, err, test.ShouldBeNil)
	sort.Slice(files, func(i, j int) bool { return filepath.Base(files[i]) < filepath.Base(files[j]) })
	test.That(t, len(files), test.ShouldBeGreaterThan, 0)

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
			fmt.Printf("chain frames: %v; goal frames: %d\n", frames, len(psc.goal))
		}

		// Mirror what NewPlanSegmentContext feeds the constraint checker.
		fsg, err := referenceframe.FrameSystemGeometries(pm.pc.fs, start.ToFrameSystemInputs())
		test.That(t, err, test.ShouldBeNil)
		movingG, staticG, _ := psc.motionChains.geometries(pm.pc.fs, fsg)
		_ = movingG
		sh := diagStaticSetHash(staticG)
		staticHashes[sh] = append(staticHashes[sh], i)
		if i > 0 && sh != prevStatic {
			staticChanges++
		}
		prevStatic = sh

		if req.ObstaclesInWorldFrame != nil {
			worldHashes[diagStaticSetHash(req.ObstaclesInWorldFrame.Geometries())] = true
		}
	}

	fmt.Printf("distinct roadmap scene keys: %d\n", len(sceneKeys))
	for k, idxs := range sceneKeys {
		fmt.Printf("  key %016x -> %d plans %v\n", k, len(idxs), truncInts(idxs, 12))
	}
	fmt.Printf("distinct static-robot SDF hashes: %d (consecutive changes: %d)\n", len(staticHashes), staticChanges)
	for k, idxs := range staticHashes {
		fmt.Printf("  static %016x -> %d plans %v\n", k, len(idxs), truncInts(idxs, 12))
	}
	fmt.Printf("distinct world-obstacle SDF hashes: %d\n", len(worldHashes))

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
		fmt.Printf("scene key %016x: %d plans across %d distinct environments\n", k, len(idxs), len(envs))
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
	fmt.Printf("keySeq: %v\n", keySeq)
	fmt.Printf("envSeq: %v\n", envSeq)
}

func truncInts(v []int, n int) []int {
	if len(v) <= n {
		return v
	}
	return v[:n]
}

// diagStaticSetHash replicates motionplan.staticSetHash (unexported there).
func diagStaticSetHash(geoms []spatialmath.Geometry) uint64 {
	const fnvPrime = 0x100000001b3
	total := uint64(len(geoms))
	for _, g := range geoms {
		h := uint64(0xcbf29ce484222325)
		mix := func(v uint64) {
			h ^= v
			h *= fnvPrime
		}
		for _, ch := range g.Label() {
			mix(uint64(ch))
		}
		pt := g.Pose().Point()
		q := g.Pose().Orientation().Quaternion()
		for _, f := range [7]float64{pt.X, pt.Y, pt.Z, q.Real, q.Imag, q.Jmag, q.Kmag} {
			mix(math.Float64bits(f))
		}
		mix(uint64(g.Hash()))
		total += h
	}
	return total
}
