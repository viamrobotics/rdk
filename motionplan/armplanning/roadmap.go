package armplanning

import (
	"container/heap"
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang/geo/r3"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// This file implements a lazy probabilistic roadmap: a reusable graph over the
// moving chain's configuration space whose edge validity is discovered per
// scene and remembered. The expensive part of planning - discovering how
// configuration-space regions connect, including the joint-family
// reconfigurations that random search spends most of its time on - is thereby
// paid once per robot+scene instead of once per plan.
//
// Structure (nodes and candidate edges) contains no collision information at
// all, so it is cached per (frame system, moving chain, goal frame) for the
// life of the process. Edges are proposed two ways:
//
//   - joint-space k-nearest neighbors: short, ordinary motions;
//   - workspace k-nearest neighbors: node pairs whose end effectors are close
//     while their configurations are far - exactly the joint-family bridges a
//     joint-space-only roadmap can never contain.
//
// Validity is evaluated on demand during the query's A* search via
// PlanSegmentContext.CheckPath (inheriting the distance-field and cover fast
// paths) and cached per scene fingerprint, including negative verdicts.

const (
	// roadmapNodes is the number of sampled configurations in the structure.
	roadmapNodes = 600
	// roadmapJointK is the joint-space nearest-neighbor edge count per node.
	roadmapJointK = 10
	// roadmapWorkspaceK is the workspace nearest-neighbor edge count per node.
	roadmapWorkspaceK = 4
	// roadmapQueryK is how many roadmap nodes the query's start and each goal
	// configuration get candidate connections to.
	roadmapQueryK = 15
	// roadmapEdgeBudget caps edge validations per query; an exhausted budget
	// fails the query and the planner falls back to search.
	roadmapEdgeBudget = 500
	// roadmapStructureSeed makes the sampled structure deterministic.
	roadmapStructureSeed = 424242
)

// roadmapRegistry caches structures per key for the life of the process.
var roadmapRegistry sync.Map // string -> *roadmap

type roadmap struct {
	frames []string // moving chain frames with DoF, sorted; defines flat layout
	dims   []int    // DoF per frame
	// goalFrame is the frame whose world position eePos records.
	goalFrame string

	// mu guards flat, neighbors, and eePos: queries hold it for read,
	// harvesting appends under write.
	mu   sync.RWMutex
	flat [][]float64
	// neighbors is the union of joint-space and workspace candidate edges.
	neighbors [][]int
	// eePos is the goalFrame's world position per node (non-chain frames at
	// the building query's start). Used to find goal-adjacent configurations
	// for IK seeding; entries that failed FK hold +Inf.
	eePos [][3]float64

	// built flips true once build has completed successfully; lock-free
	// readers (peekRoadmap) must check it before touching any other field.
	built atomic.Bool

	// key is the registry key; used to derive the disk-cache filename.
	key string
	// lastSaveNs throttles disk saves (unix nanos of the last save).
	lastSaveNs atomic.Int64

	// sceneVerdicts caches per-scene edge validity: key -> bool.
	sceneVerdicts sync.Map // roadmapVerdictKey -> bool

	// Dynamic-roadmap (DRM-style) state, all scene-independent and persisted:
	// edgeVox holds each structural edge's swept workspace volume as sorted
	// coarse voxel keys (computed lazily on first touch); cleanValid records
	// edges that were fully validated in a scene whose occupancy did not
	// intersect their swept volume - such a verdict transfers to any other
	// scene that also misses the volume (same constraint fingerprint), which
	// is what keeps a learned roadmap warm when obstacles move a little
	// (per Leven & Hutchinson's dynamic roadmaps).
	edgeVoxMu  sync.RWMutex
	edgeVox    map[uint64][]uint64
	cleanValid sync.Map // drmVerdictKey -> bool(true)
	// sceneOcc caches per-scene occupancy voxel sets (in-memory only).
	// sceneOccCount bounds it: a robot whose obstacles move sees a new scene
	// fingerprint every plan, and an unbounded per-scene cache is a leak.
	sceneOcc      sync.Map // uint64 -> map[uint64]struct{}
	sceneOccCount atomic.Int64

	// smoothedCache memoizes the final smoothed-and-expanded trajectory for a
	// (scene, raw roadmap path) pair. Smoothing plus close-obstacle expansion
	// is the dominant cost of a warm roadmap solve (~0.5s of a 0.75s plan on
	// a captured dual-arm scene) and is deterministic given the same scene
	// and the same raw waypoints, which is exactly what a replayed corridor
	// produces. Persisted with the roadmap.
	smoothedCache sync.Map // uint64 -> [][]float64 (linearized configs)
	// smoothedCount bounds smoothedCache growth.
	smoothedCount atomic.Int64

	buildOnce sync.Once
	buildErr  error
}

type drmVerdictKey struct {
	constraints uint64
	a, b        int32
}

type roadmapVerdictKey struct {
	scene uint64
	a, b  int32 // canonical: a < b; query endpoints use negative ids
}

// roadmapKeyFor identifies a reusable structure.
func roadmapKeyFor(psc *PlanSegmentContext, frames []string) string {
	var sb strings.Builder
	sb.WriteString(psc.pc.fs.Name())
	for _, f := range frames {
		sb.WriteByte('|')
		sb.WriteString(f)
	}
	for f := range psc.goal {
		sb.WriteByte('#')
		sb.WriteString(f)
	}
	return sb.String()
}

// getRoadmap returns the (possibly freshly built) structure for this segment
// context, or nil when the chain is unsuitable.
func getRoadmap(psc *PlanSegmentContext, logger logging.Logger) *roadmap {
	frames := movingFrameNamesWithDoF(psc)
	if len(frames) == 0 || len(psc.goal) != 1 {
		return nil
	}
	sort.Strings(frames)
	key := roadmapKeyFor(psc, frames)
	v, _ := roadmapRegistry.LoadOrStore(key, &roadmap{frames: frames, key: key})
	rm := v.(*roadmap)
	rm.buildOnce.Do(func() {
		if rm.loadFromDisk(logger) {
			rm.built.Store(true)
			return
		}
		rm.buildErr = rm.build(psc, logger)
	})
	if rm.buildErr != nil {
		return nil
	}
	return rm
}

// peekRoadmap returns the already-built structure for this segment context,
// or nil. Unlike getRoadmap it never builds, so cheap callers (IK seeding)
// don't pay the build cost for plans that may never need the roadmap.
func peekRoadmap(psc *PlanSegmentContext) *roadmap {
	frames := movingFrameNamesWithDoF(psc)
	if len(frames) == 0 || len(psc.goal) != 1 {
		return nil
	}
	sort.Strings(frames)
	v, ok := roadmapRegistry.Load(roadmapKeyFor(psc, frames))
	if !ok {
		return nil
	}
	rm := v.(*roadmap)
	if !rm.built.Load() {
		return nil
	}
	return rm
}

// roadmapGoalSeeds returns up to maxSeeds roadmap configurations whose end
// effector already sits near this goal's world position - in practice, goal
// configurations harvested from earlier successful plans in this process.
// They are fed to IK as seeds: nlopt's per-seed draws sometimes miss the
// joint family closest to the start entirely, and every downstream strategy
// then pays a reconfiguration detour to whatever family it did find. Seeding
// with remembered goal-adjacent configurations makes the good family reliably
// present, so the solution ranking (by cost from start) can prefer it.
func roadmapGoalSeeds(psc *PlanSegmentContext, maxSeeds int) []*referenceframe.LinearInputs {
	rm := peekRoadmap(psc)
	if rm == nil {
		return nil
	}
	var goalFrame string
	goalPt := [3]float64{}
	for f, p := range psc.goal {
		goalFrame = f
		pt := p.Pose().Point()
		goalPt = [3]float64{pt.X, pt.Y, pt.Z}
	}
	if goalFrame != rm.goalFrame {
		return nil
	}
	// Generous radius: harvested goal configurations match the goal almost
	// exactly; uniform samples essentially never land this close. The seeds
	// are advisory (IK still solves to the actual goal), so a near-miss from
	// a slightly moved goal is still a useful seed.
	const goalSeedRadiusMM = 100.0
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	type cand struct {
		d float64
		i int
	}
	cands := make([]cand, 0, 8)
	for i, p := range rm.eePos {
		dx, dy, dz := p[0]-goalPt[0], p[1]-goalPt[1], p[2]-goalPt[2]
		if d := dx*dx + dy*dy + dz*dz; d < goalSeedRadiusMM*goalSeedRadiusMM {
			cands = append(cands, cand{d, i})
		}
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].d < cands[j].d })
	out := make([]*referenceframe.LinearInputs, 0, maxSeeds)
	chosen := make([][]float64, 0, maxSeeds)
	for _, c := range cands {
		f := rm.flat[c.i]
		// Keep only one representative per joint family.
		distinct := true
		for _, cf := range chosen {
			if flatL2Sq(cf, f) < 1.0 {
				distinct = false
				break
			}
		}
		if !distinct {
			continue
		}
		chosen = append(chosen, f)
		out = append(out, rm.compose(psc.start, f))
		if len(out) >= maxSeeds {
			break
		}
	}
	return out
}

// build samples the structure. No collision checking happens here.
func (rm *roadmap) build(psc *PlanSegmentContext, logger logging.Logger) error {
	fs := psc.pc.fs
	totalDoF := 0
	rm.dims = make([]int, len(rm.frames))
	type frameLimits struct {
		min, max []float64
	}
	limits := make([]frameLimits, len(rm.frames))
	for i, name := range rm.frames {
		f := fs.Frame(name)
		if f == nil {
			return fmt.Errorf("roadmap: missing frame %s", name)
		}
		dof := f.DoF()
		rm.dims[i] = len(dof)
		totalDoF += len(dof)
		fl := frameLimits{min: make([]float64, len(dof)), max: make([]float64, len(dof))}
		for j, l := range dof {
			lo, hi := l.Min, l.Max
			if math.IsInf(lo, -1) || lo < -999 {
				lo = -999
			}
			if math.IsInf(hi, 1) || hi > 999 {
				hi = 999
			}
			fl.min[j], fl.max[j] = lo, hi
		}
		limits[i] = fl
	}
	if totalDoF == 0 {
		return fmt.Errorf("roadmap: chain has no degrees of freedom")
	}

	//nolint:gosec
	rnd := rand.New(rand.NewSource(roadmapStructureSeed))
	rm.flat = make([][]float64, roadmapNodes)
	for n := 0; n < roadmapNodes; n++ {
		cfg := make([]float64, 0, totalDoF)
		for i := range rm.frames {
			for j := 0; j < rm.dims[i]; j++ {
				lo, hi := limits[i].min[j], limits[i].max[j]
				cfg = append(cfg, lo+rnd.Float64()*(hi-lo))
			}
		}
		rm.flat[n] = cfg
	}

	// Workspace positions for the bridge edges: end-effector pose of each
	// node, other frames at the query start (constant offsets cancel in
	// nearest-neighbor comparisons).
	var goalFrame string
	for f := range psc.goal {
		goalFrame = f
	}
	positions := make([][3]float64, roadmapNodes)
	for n := range rm.flat {
		cfg := rm.compose(psc.start, rm.flat[n])
		fk := fs.NewFrameToWorld(cfg)
		_, pt, err := fk.PoseQT(goalFrame)
		if err != nil {
			return err
		}
		positions[n] = [3]float64{pt.X, pt.Y, pt.Z}
	}
	rm.goalFrame = goalFrame
	rm.eePos = positions

	// Candidate edges: joint kNN plus workspace kNN.
	rm.neighbors = make([][]int, roadmapNodes)
	type distIdx struct {
		d float64
		i int
	}
	addEdge := func(a, b int) {
		if a == b {
			return
		}
		for _, e := range rm.neighbors[a] {
			if e == b {
				return
			}
		}
		rm.neighbors[a] = append(rm.neighbors[a], b)
		rm.neighbors[b] = append(rm.neighbors[b], a)
	}
	scratch := make([]distIdx, roadmapNodes)
	for a := 0; a < roadmapNodes; a++ {
		for b := 0; b < roadmapNodes; b++ {
			scratch[b] = distIdx{flatL2Sq(rm.flat[a], rm.flat[b]), b}
		}
		sort.Slice(scratch, func(i, j int) bool { return scratch[i].d < scratch[j].d })
		for i := 1; i <= roadmapJointK && i < len(scratch); i++ {
			addEdge(a, scratch[i].i)
		}
		for b := 0; b < roadmapNodes; b++ {
			dx := positions[a][0] - positions[b][0]
			dy := positions[a][1] - positions[b][1]
			dz := positions[a][2] - positions[b][2]
			scratch[b] = distIdx{dx*dx + dy*dy + dz*dz, b}
		}
		sort.Slice(scratch, func(i, j int) bool { return scratch[i].d < scratch[j].d })
		for i := 1; i <= roadmapWorkspaceK && i < len(scratch); i++ {
			addEdge(a, scratch[i].i)
		}
	}

	edgeCount := 0
	for _, nb := range rm.neighbors {
		edgeCount += len(nb)
	}
	logger.Debugf("roadmap built: %d nodes, %d DoF, %d edges", roadmapNodes, totalDoF, edgeCount/2)
	rm.built.Store(true)
	return nil
}

// compose overlays a flat chain configuration onto the base configuration.
func (rm *roadmap) compose(base *referenceframe.LinearInputs, flat []float64) *referenceframe.LinearInputs {
	out := base.Copy()
	idx := 0
	for i, name := range rm.frames {
		vals := make([]referenceframe.Input, rm.dims[i])
		for j := range vals {
			vals[j] = flat[idx]
			idx++
		}
		out.Put(name, vals)
	}
	return out
}

// extract pulls the chain frames' values out of a full configuration.
func (rm *roadmap) extract(cfg *referenceframe.LinearInputs) []float64 {
	out := make([]float64, 0, 16)
	for i, name := range rm.frames {
		vals := cfg.Get(name)
		if len(vals) != rm.dims[i] {
			return nil
		}
		out = append(out, vals...)
	}
	return out
}

func flatL2Sq(a, b []float64) float64 {
	t := 0.0
	for i := range a {
		d := a[i] - b[i]
		t += d * d
	}
	return t
}

// DRM voxel grid: fixed world-aligned grid so voxel keys are stable across
// processes and scenes.
const drmVoxelMM = 50.0

// drmEdgeSampleL2 is the joint-space interval between swept-volume samples.
// Finer sampling costs build time; the end-to-end validation in smoothPath
// backstops the (rare) case where motion between samples clips a voxel the
// sweep missed.
const drmEdgeSampleL2 = 0.1

func drmVoxelKey(x, y, z int32) uint64 {
	return uint64(uint16(x))<<32 | uint64(uint16(y))<<16 | uint64(uint16(z))
}

// drmRasterizeSphere marks all voxels overlapping the sphere's AABB.
func drmRasterizeSphere(c r3.Vector, r float64, out map[uint64]struct{}) {
	x0, x1 := int32(math.Floor((c.X-r)/drmVoxelMM)), int32(math.Floor((c.X+r)/drmVoxelMM))
	y0, y1 := int32(math.Floor((c.Y-r)/drmVoxelMM)), int32(math.Floor((c.Y+r)/drmVoxelMM))
	z0, z1 := int32(math.Floor((c.Z-r)/drmVoxelMM)), int32(math.Floor((c.Z+r)/drmVoxelMM))
	for x := x0; x <= x1; x++ {
		for y := y0; y <= y1; y++ {
			for z := z0; z <= z1; z++ {
				out[drmVoxelKey(x, y, z)] = struct{}{}
			}
		}
	}
}

func drmEdgeKey(a, b int) uint64 {
	if a > b {
		a, b = b, a
	}
	return uint64(a)<<32 | uint64(b)
}

// edgeSweptVoxels returns (computing and persisting on first touch) the
// sorted voxel keys covering the chain's swept volume along edge a-b.
func (rm *roadmap) edgeSweptVoxels(psc *PlanSegmentContext, a, b int) []uint64 {
	ek := drmEdgeKey(a, b)
	rm.edgeVoxMu.RLock()
	v, ok := rm.edgeVox[ek]
	rm.edgeVoxMu.RUnlock()
	if ok {
		return v
	}
	cfgA := rm.compose(psc.start, rm.flat[a])
	cfgB := rm.compose(psc.start, rm.flat[b])
	steps := int(math.Ceil(flatL2(rm.flat[a], rm.flat[b])/drmEdgeSampleL2)) + 1
	if steps < 2 {
		steps = 2
	}
	chain := map[string]bool{}
	for _, f := range rm.frames {
		chain[f] = true
	}
	set := map[uint64]struct{}{}
	for i := 0; i <= steps; i++ {
		cfg, err := referenceframe.InterpolateFS(psc.pc.fs, cfgA, cfgB, float64(i)/float64(steps))
		if err != nil {
			return nil
		}
		geoms, err := referenceframe.FrameSystemGeometriesForFrames(psc.pc.fs, cfg, chain)
		if err != nil {
			return nil
		}
		for _, gif := range geoms {
			for _, g := range gif.Geometries() {
				for _, sb := range spatialmath.SphereCover(g) {
					drmRasterizeSphere(sb.Center, sb.R+drmVoxelMM/2, set)
				}
			}
		}
	}
	out := make([]uint64, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	rm.edgeVoxMu.Lock()
	if rm.edgeVox == nil {
		rm.edgeVox = map[uint64][]uint64{}
	}
	rm.edgeVox[ek] = out
	rm.edgeVoxMu.Unlock()
	return out
}

func flatL2(a, b []float64) float64 { return math.Sqrt(flatL2Sq(a, b)) }

// sceneOccupancy returns (cached per scene) the occupancy voxel set of
// everything the chain can collide with: world obstacles plus the rest of the
// robot at the query's start configuration, inflated by the collision buffer.
func (rm *roadmap) sceneOccupancy(pm *planManager, psc *PlanSegmentContext, sceneKey uint64) map[uint64]struct{} {
	if v, ok := rm.sceneOcc.Load(sceneKey); ok {
		return v.(map[uint64]struct{})
	}
	occ := map[uint64]struct{}{}
	inflate := drmVoxelMM/2 + pm.request.PlannerOptions.CollisionBufferMM
	if pm.request.ObstaclesInWorldFrame != nil {
		for _, g := range pm.request.ObstaclesInWorldFrame.Geometries() {
			for _, sb := range spatialmath.SphereCover(g) {
				drmRasterizeSphere(sb.Center, sb.R+inflate, occ)
			}
		}
	}
	chain := map[string]bool{}
	for _, f := range rm.frames {
		chain[f] = true
	}
	nonChain := map[string]bool{}
	for _, name := range psc.pc.fs.FrameNames() {
		if !chain[name] {
			nonChain[name] = true
		}
	}
	if geoms, err := referenceframe.FrameSystemGeometriesForFrames(psc.pc.fs, psc.start, nonChain); err == nil {
		for _, gif := range geoms {
			for _, g := range gif.Geometries() {
				for _, sb := range spatialmath.SphereCover(g) {
					drmRasterizeSphere(sb.Center, sb.R+inflate, occ)
				}
			}
		}
	}
	const sceneOccCap = 32
	if rm.sceneOccCount.Load() >= sceneOccCap {
		// Cache full: serve the computed set without retaining it.
		return occ
	}
	actual, loaded := rm.sceneOcc.LoadOrStore(sceneKey, occ)
	if !loaded {
		rm.sceneOccCount.Add(1)
	}
	return actual.(map[uint64]struct{})
}

func drmOverlaps(occ map[uint64]struct{}, voxels []uint64) bool {
	for _, v := range voxels {
		if _, hit := occ[v]; hit {
			return true
		}
	}
	return false
}

// constraintsFingerprint hashes the request's constraints; DRM verdict
// transfer is only valid between scenes planned under identical constraints.
func constraintsFingerprint(c *motionplan.Constraints) uint64 {
	data, err := json.Marshal(c)
	if err != nil {
		return 0
	}
	h := fnv.New64a()
	h.Write(data)
	return h.Sum64()
}

// tryRoadmap answers the query start -> any goal root through the roadmap
// with lazily-validated edges. Returns the waypoint list (full
// configurations, start included) or nil.
func (pm *planManager) tryRoadmap(
	ctx context.Context,
	psc *PlanSegmentContext,
	goalRoots []*node,
	logger logging.Logger,
) []*referenceframe.LinearInputs {
	rm := getRoadmap(psc, logger)
	if rm == nil || len(goalRoots) == 0 {
		return nil
	}
	// Every goal root costs roadmapQueryK candidate connections in the
	// multi-goal A*; past the several cheapest, extra roots only burn edge
	// budget.
	const roadmapMaxGoals = 8
	if len(goalRoots) > roadmapMaxGoals {
		sort.Slice(goalRoots, func(i, j int) bool { return goalRoots[i].cost < goalRoots[j].cost })
		goalRoots = goalRoots[:roadmapMaxGoals]
	}
	sceneKey := pm.roadmapSceneKey(psc, rm)
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	startFlat := rm.extract(psc.start)
	if startFlat == nil {
		return nil
	}
	goals := make([][]float64, 0, len(goalRoots))
	goalCfgs := make([]*referenceframe.LinearInputs, 0, len(goalRoots))
	for _, g := range goalRoots {
		if f := rm.extract(g.inputs); f != nil {
			goals = append(goals, f)
			goalCfgs = append(goalCfgs, g.inputs)
		}
	}
	if len(goals) == 0 {
		return nil
	}

	// DRM state for this query: scene occupancy and the constraint
	// fingerprint under which clean verdicts may transfer.
	occ := rm.sceneOccupancy(pm, psc, sceneKey)
	constraintKey := constraintsFingerprint(pm.request.Constraints)

	// Query graph ids: 0..N-1 roadmap, N = start, N+1+i = goal i.
	n := len(rm.flat)
	startID := n
	goalID := func(i int) int { return n + 1 + i }
	isGoal := func(id int) bool { return id > startID }

	flatOf := func(id int) []float64 {
		switch {
		case id < n:
			return rm.flat[id]
		case id == startID:
			return startFlat
		default:
			return goals[id-startID-1]
		}
	}
	cfgOf := func(id int) *referenceframe.LinearInputs {
		switch {
		case id < n:
			return rm.compose(psc.start, rm.flat[id])
		case id == startID:
			return psc.start
		default:
			return goalCfgs[id-startID-1]
		}
	}

	// Candidate neighbors of the query endpoints.
	queryNeighbors := func(flat []float64) []int {
		type distIdx struct {
			d float64
			i int
		}
		best := make([]distIdx, 0, n)
		for i := 0; i < n; i++ {
			best = append(best, distIdx{flatL2Sq(flat, rm.flat[i]), i})
		}
		sort.Slice(best, func(i, j int) bool { return best[i].d < best[j].d })
		out := make([]int, 0, roadmapQueryK)
		for i := 0; i < roadmapQueryK && i < len(best); i++ {
			out = append(out, best[i].i)
		}
		return out
	}
	startNbrs := queryNeighbors(startFlat)
	goalNbrs := make(map[int][]int, len(goals)) // roadmap node -> goal ids it connects to
	for gi, g := range goals {
		for _, nb := range queryNeighbors(g) {
			goalNbrs[nb] = append(goalNbrs[nb], goalID(gi))
		}
	}

	neighborsOf := func(id int) []int {
		switch {
		case id == startID:
			return startNbrs
		case isGoal(id):
			return nil // goals are terminal
		default:
			nbrs := rm.neighbors[id]
			if extra, ok := goalNbrs[id]; ok {
				out := make([]int, 0, len(nbrs)+len(extra)+1)
				out = append(out, nbrs...)
				out = append(out, extra...)
				return out
			}
			return nbrs
		}
	}

	heuristic := func(id int) float64 {
		f := flatOf(id)
		best := math.Inf(1)
		for _, g := range goals {
			best = math.Min(best, flatL2Sq(f, g))
		}
		return math.Sqrt(best)
	}

	budget := roadmapEdgeBudget
	validated := 0
	checkEdge := func(a, b int) bool {
		ka, kb := int32(a), int32(b)
		// Query endpoints get negative ids in the cache key so different
		// queries never collide with structural edges.
		if a >= n {
			ka = -int32(a - n + 1)
		}
		if b >= n {
			kb = -int32(b - n + 1)
		}
		if ka > kb {
			ka, kb = kb, ka
		}
		key := roadmapVerdictKey{scene: sceneKey, a: ka, b: kb}
		// Query-endpoint verdicts are only cacheable for the start/goals of
		// this exact query; structural edges (both ids >= 0) cache globally.
		cacheable := ka >= 0
		if cacheable {
			if v, ok := rm.sceneVerdicts.Load(key); ok {
				return v.(bool)
			}
			// DRM transfer: this edge was fully validated under the same
			// constraints in a scene whose occupancy missed its swept volume;
			// if this scene's occupancy misses it too, the verdict carries
			// over without a collision check.
			if _, clean := rm.cleanValid.Load(drmVerdictKey{constraints: constraintKey, a: ka, b: kb}); clean {
				if vox := rm.edgeSweptVoxels(psc, a, b); vox != nil && !drmOverlaps(occ, vox) {
					rm.sceneVerdicts.Store(key, true)
					return true
				}
			}
		}
		if budget <= 0 {
			return false
		}
		budget--
		validated++
		ok := psc.CheckPath(ctx, cfgOf(a), cfgOf(b), true, nil) == nil
		if cacheable {
			rm.sceneVerdicts.Store(key, ok)
			if ok {
				// Record a clean verdict when nothing in this scene was near
				// the edge's swept volume: validity then owed nothing to this
				// scene's obstacles and is transferable.
				if vox := rm.edgeSweptVoxels(psc, a, b); vox != nil && !drmOverlaps(occ, vox) {
					rm.cleanValid.Store(drmVerdictKey{constraints: constraintKey, a: ka, b: kb}, true)
				}
			}
		}
		return ok
	}

	// A* with lazy edge validation on expansion.
	pq := &roadmapPQ{}
	heap.Init(pq)
	gScore := map[int]float64{startID: 0}
	cameFrom := map[int]int{}
	closed := map[int]bool{}
	heap.Push(pq, roadmapQItem{startID, heuristic(startID)})

	for pq.Len() > 0 && ctx.Err() == nil && budget > 0 {
		cur := heap.Pop(pq).(roadmapQItem)
		if closed[cur.id] {
			continue
		}
		closed[cur.id] = true
		if isGoal(cur.id) {
			// Reconstruct.
			ids := []int{cur.id}
			for ids[len(ids)-1] != startID {
				ids = append(ids, cameFrom[ids[len(ids)-1]])
			}
			path := make([]*referenceframe.LinearInputs, 0, len(ids))
			for i := len(ids) - 1; i >= 0; i-- {
				path = append(path, cfgOf(ids[i]))
			}
			logger.Debugf("roadmap connected: %d waypoints, %d edges validated", len(path), validated)
			return path
		}
		for _, nb := range neighborsOf(cur.id) {
			if closed[nb] {
				continue
			}
			if !checkEdge(cur.id, nb) {
				continue
			}
			tentative := gScore[cur.id] + math.Sqrt(flatL2Sq(flatOf(cur.id), flatOf(nb)))
			if old, ok := gScore[nb]; ok && tentative >= old {
				continue
			}
			gScore[nb] = tentative
			cameFrom[nb] = cur.id
			heap.Push(pq, roadmapQItem{nb, tentative + heuristic(nb)})
		}
	}
	logger.Debugf("roadmap query failed: %d edges validated, budget left %d", validated, budget)
	return nil
}

// roadmapSceneKey fingerprints everything edge validity depends on beyond the
// structure: the obstacle set, the non-chain part of the configuration, and
// the static robot geometry's world poses. The geometry poses must be in the
// key because the non-chain inputs alone are blind to environment geometry
// that lives in the frame system (a door whose fixed transform is updated
// between plans, a tracked fixture): the inputs never change while the
// geometry moves, so edge verdicts, cached occupancy, and cached smoothed
// trajectories would be shared across physically different scenes.
func (pm *planManager) roadmapSceneKey(psc *PlanSegmentContext, rm *roadmap) uint64 {
	const fnvPrime = 0x100000001b3
	h := uint64(0xcbf29ce484222325)
	mix := func(v uint64) {
		h ^= v
		h *= fnvPrime
	}
	inChain := map[string]bool{}
	for _, f := range rm.frames {
		inChain[f] = true
	}
	for name, vals := range psc.start.Items() {
		if inChain[name] {
			continue
		}
		for _, ch := range name {
			mix(uint64(ch))
		}
		for _, v := range vals {
			mix(math.Float64bits(v))
		}
	}
	mix(psc.staticGeomHash)
	if pm.request.ObstaclesInWorldFrame != nil {
		for _, g := range pm.request.ObstaclesInWorldFrame.Geometries() {
			for _, ch := range g.Label() {
				mix(uint64(ch))
			}
			pt := g.Pose().Point()
			mix(math.Float64bits(pt.X))
			mix(math.Float64bits(pt.Y))
			mix(math.Float64bits(pt.Z))
		}
	}
	return h
}

// roadmapQItem is one A* frontier entry.
type roadmapQItem struct {
	id int
	f  float64
}

// roadmapPQ is a minimal min-heap on f-score for the A* frontier.
type roadmapPQ []roadmapQItem

func (pq roadmapPQ) Len() int           { return len(pq) }
func (pq roadmapPQ) Less(i, j int) bool { return pq[i].f < pq[j].f }
func (pq roadmapPQ) Swap(i, j int)      { pq[i], pq[j] = pq[j], pq[i] }
func (pq *roadmapPQ) Push(x any)        { *pq = append(*pq, x.(roadmapQItem)) }
func (pq *roadmapPQ) Pop() any {
	old := *pq
	n := len(old)
	x := old[n-1]
	*pq = old[:n-1]
	return x
}

// harvestKNN is how many candidate edges each harvested waypoint gets into
// the existing structure (validity lazy, like any candidate edge).
const harvestKNN = 6

// harvest inserts a successful plan's waypoints into the roadmap: nodes are
// permanent (they describe robot capability, not the scene), consecutive
// edges are recorded as valid for the scene they were planned in, and each
// node is candidate-linked to its nearest existing neighbors. Every solved
// plan thereby teaches the roadmap a working corridor - including the
// joint-family reconfigurations a fresh search pays hundreds of iterations to
// rediscover.
func (rm *roadmap) harvest(psc *PlanSegmentContext, scene uint64, path []*referenceframe.LinearInputs, logger logging.Logger) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	type distIdx struct {
		d float64
		i int
	}
	prev := -1
	added := 0
	for _, cfg := range path {
		flat := rm.extract(cfg)
		if flat == nil {
			prev = -1
			continue
		}
		if prev >= 0 && flatL2Sq(rm.flat[prev], flat) < 1e-12 {
			continue
		}
		// Reuse an existing node when the waypoint is (nearly) already there,
		// so repeated plans don't grow the roadmap without bound.
		id := -1
		best := 1e-4 // squared; ~0.01 rad
		for i, f := range rm.flat {
			if d := flatL2Sq(f, flat); d < best {
				best, id = d, i
			}
		}
		if id < 0 {
			id = len(rm.flat)
			rm.flat = append(rm.flat, flat)
			rm.neighbors = append(rm.neighbors, nil)
			pos := [3]float64{math.Inf(1), math.Inf(1), math.Inf(1)}
			if _, pt, err := psc.pc.fs.NewFrameToWorld(cfg).PoseQT(rm.goalFrame); err == nil {
				pos = [3]float64{pt.X, pt.Y, pt.Z}
			}
			rm.eePos = append(rm.eePos, pos)
			added++
			// Candidate-link into the structure.
			cands := make([]distIdx, 0, id)
			for i := 0; i < id; i++ {
				cands = append(cands, distIdx{flatL2Sq(rm.flat[i], flat), i})
			}
			sort.Slice(cands, func(i, j int) bool { return cands[i].d < cands[j].d })
			for i := 0; i < harvestKNN && i < len(cands); i++ {
				nb := cands[i].i
				rm.neighbors[id] = append(rm.neighbors[id], nb)
				rm.neighbors[nb] = append(rm.neighbors[nb], id)
			}
		}
		if prev >= 0 && prev != id {
			rm.neighbors[prev] = append(rm.neighbors[prev], id)
			rm.neighbors[id] = append(rm.neighbors[id], prev)
			a, b := int32(prev), int32(id)
			if a > b {
				a, b = b, a
			}
			// The hop was validated by the plan's full-resolution smoothing
			// sweep for this scene.
			rm.sceneVerdicts.Store(roadmapVerdictKey{scene: scene, a: a, b: b}, true)
		}
		prev = id
	}
	if added > 0 {
		logger.Debugf("roadmap harvested %d nodes (total %d)", added, len(rm.flat))
	}
}

// harvestPlan records a successful plan into the roadmap, if one exists for
// this segment context.
func (pm *planManager) harvestPlan(psc *PlanSegmentContext, steps []*referenceframe.LinearInputs, logger logging.Logger) {
	rm := getRoadmap(psc, logger)
	if rm == nil || len(steps) < 2 {
		return
	}
	// Backstop against feedback loops: a plan that smoothed poorly would
	// teach the roadmap a long, awkward corridor that makes every later
	// query's raw path longer still.
	if len(steps) > 48 {
		logger.Debugf("skipping harvest of %d-waypoint path", len(steps))
		return
	}
	rm.harvest(psc, pm.roadmapSceneKey(psc, rm), steps, logger)
	rm.saveToDiskThrottled(logger)
}

// roadmapSmoothedCacheCap bounds how many smoothed trajectories one roadmap
// retains; corridors beyond that simply re-smooth.
const roadmapSmoothedCacheCap = 64

// smoothedPathKey hashes a scene fingerprint plus the raw path's configs.
func smoothedPathKey(scene uint64, path []*referenceframe.LinearInputs) uint64 {
	h := fnv.New64a()
	var buf [8]byte
	put := func(v uint64) {
		for i := 0; i < 8; i++ {
			buf[i] = byte(v >> (8 * i))
		}
		h.Write(buf[:])
	}
	put(scene)
	for _, cfg := range path {
		for _, f := range cfg.GetLinearizedInputs() {
			put(math.Float64bits(f))
		}
	}
	return h.Sum64()
}

// cachedSmoothed returns the memoized final trajectory for this scene+path,
// or nil.
func (rm *roadmap) cachedSmoothed(
	psc *PlanSegmentContext, scene uint64, path []*referenceframe.LinearInputs,
) []*referenceframe.LinearInputs {
	v, ok := rm.smoothedCache.Load(smoothedPathKey(scene, path))
	if !ok {
		return nil
	}
	flat := v.([][]float64)
	out := make([]*referenceframe.LinearInputs, 0, len(flat))
	for _, f := range flat {
		cfg, err := psc.pc.lis.FloatsToInputs(f)
		if err != nil {
			return nil
		}
		out = append(out, cfg)
	}
	return out
}

// storeSmoothed memoizes a final trajectory for this scene+path. At capacity
// an arbitrary entry is evicted: a scene-changing robot would otherwise fill
// the cache with stale scenes once and never cache again.
func (rm *roadmap) storeSmoothed(scene uint64, path, smoothed []*referenceframe.LinearInputs) {
	if rm.smoothedCount.Load() >= roadmapSmoothedCacheCap {
		rm.smoothedCache.Range(func(k, _ any) bool {
			rm.smoothedCache.Delete(k)
			rm.smoothedCount.Add(-1)
			return false
		})
	}
	flat := make([][]float64, 0, len(smoothed))
	for _, cfg := range smoothed {
		src := cfg.GetLinearizedInputs()
		f := make([]float64, len(src))
		copy(f, src)
		flat = append(flat, f)
	}
	if _, loaded := rm.smoothedCache.LoadOrStore(smoothedPathKey(scene, path), flat); !loaded {
		rm.smoothedCount.Add(1)
	}
}

// Roadmap disk persistence. The roadmap's value compounds with experience:
// harvested corridors and per-scene edge verdicts are what turn a 45-120s
// cold search into a sub-second warm replay. A process restart threw all of
// that away. When MOTION_ROADMAP_CACHE_DIR is set (cmd-plan sets a default;
// the library leaves it opt-in), each roadmap is saved there after harvests
// and loaded instead of built on the next process start. Safety: the
// filename is derived from the registry key (frame system + chain + goal
// frame), the file carries the frames/dims for an identity check, and edge
// verdicts stay keyed by the scene fingerprint, so a changed scene simply
// misses rather than misapplies. A corrupt or mismatched file is ignored and
// the roadmap is rebuilt.

type roadmapDiskVerdict struct {
	Scene uint64 `json:"s"`
	A     int32  `json:"a"`
	B     int32  `json:"b"`
	Clear bool   `json:"c"`
}

type roadmapDisk struct {
	Frames    []string             `json:"frames"`
	Dims      []int                `json:"dims"`
	GoalFrame string               `json:"goal_frame"`
	Flat      [][]float64          `json:"flat"`
	Neighbors [][]int              `json:"neighbors"`
	EePos     [][3]float64         `json:"ee_pos"`
	Verdicts  []roadmapDiskVerdict `json:"verdicts"`
	// Smoothed maps smoothedPathKey (as decimal string; JSON keys must be
	// strings) to the final trajectory's linearized configs.
	Smoothed map[string][][]float64 `json:"smoothed,omitempty"`
	// EdgeVox maps drmEdgeKey (decimal string) to sorted swept-volume voxels.
	EdgeVox map[string][]uint64 `json:"edge_vox,omitempty"`
	// CleanValid lists constraint-fingerprinted transferable edge verdicts.
	CleanValid []roadmapDiskClean `json:"clean_valid,omitempty"`
}

type roadmapDiskClean struct {
	Constraints uint64 `json:"k"`
	A           int32  `json:"a"`
	B           int32  `json:"b"`
}

// roadmapSaveMinInterval bounds how often one roadmap writes its cache file.
const roadmapSaveMinInterval = 5 * time.Second

func roadmapCachePath(key string) string {
	dir := os.Getenv("MOTION_ROADMAP_CACHE_DIR")
	if dir == "" {
		return ""
	}
	h := fnv.New64a()
	h.Write([]byte(key))
	return filepath.Join(dir, fmt.Sprintf("roadmap-%016x.json", h.Sum64()))
}

func (rm *roadmap) loadFromDisk(logger logging.Logger) bool {
	path := roadmapCachePath(rm.key)
	if path == "" {
		return false
	}
	data, err := os.ReadFile(path) //nolint:gosec // path derives from the operator-set cache-dir env var plus a hash
	if err != nil {
		return false
	}
	var d roadmapDisk
	if err := json.Unmarshal(data, &d); err != nil {
		logger.Debugf("roadmap cache %s unreadable, rebuilding: %v", path, err)
		return false
	}
	if len(d.Frames) != len(rm.frames) || len(d.Flat) == 0 || len(d.Flat) != len(d.Neighbors) || len(d.Flat) != len(d.EePos) {
		return false
	}
	for i, f := range rm.frames {
		if d.Frames[i] != f {
			return false
		}
	}
	// A corrupt or hand-edited file must degrade to a rebuild, not a panic:
	// validate the graph's internal consistency before adopting it.
	totalDoF := 0
	for _, dof := range d.Dims {
		if dof < 0 {
			return false
		}
		totalDoF += dof
	}
	for _, row := range d.Flat {
		if len(row) != totalDoF {
			return false
		}
	}
	for _, nbs := range d.Neighbors {
		for _, nb := range nbs {
			if nb < 0 || nb >= len(d.Flat) {
				return false
			}
		}
	}
	rm.dims = d.Dims
	rm.goalFrame = d.GoalFrame
	rm.flat = d.Flat
	rm.neighbors = d.Neighbors
	rm.eePos = d.EePos
	for _, v := range d.Verdicts {
		rm.sceneVerdicts.Store(roadmapVerdictKey{scene: v.Scene, a: v.A, b: v.B}, v.Clear)
	}
	for k, traj := range d.Smoothed {
		var key uint64
		if _, err := fmt.Sscanf(k, "%d", &key); err == nil {
			rm.smoothedCache.Store(key, traj)
			rm.smoothedCount.Add(1)
		}
	}
	if len(d.EdgeVox) > 0 {
		rm.edgeVox = make(map[uint64][]uint64, len(d.EdgeVox))
		for k, vox := range d.EdgeVox {
			var key uint64
			if _, err := fmt.Sscanf(k, "%d", &key); err == nil {
				rm.edgeVox[key] = vox
			}
		}
	}
	for _, cv := range d.CleanValid {
		rm.cleanValid.Store(drmVerdictKey{constraints: cv.Constraints, a: cv.A, b: cv.B}, true)
	}
	logger.Debugf("roadmap loaded from %s: %d nodes, %d verdicts", path, len(rm.flat), len(d.Verdicts))
	return true
}

func (rm *roadmap) saveToDiskThrottled(logger logging.Logger) {
	path := roadmapCachePath(rm.key)
	if path == "" {
		return
	}
	now := time.Now().UnixNano()
	last := rm.lastSaveNs.Load()
	if last != 0 && time.Duration(now-last) < roadmapSaveMinInterval {
		return
	}
	if !rm.lastSaveNs.CompareAndSwap(last, now) {
		return
	}
	d := roadmapDisk{GoalFrame: rm.goalFrame}
	rm.mu.RLock()
	d.Frames = append([]string{}, rm.frames...)
	d.Dims = append([]int{}, rm.dims...)
	// flat and eePos rows are never mutated after insertion, so snapshotting
	// the outer slices is safe; neighbors rows are appended to in place by
	// harvest, so they must be deep-copied under the lock.
	d.Flat = rm.flat[:len(rm.flat):len(rm.flat)]
	d.EePos = rm.eePos[:len(rm.eePos):len(rm.eePos)]
	d.Neighbors = make([][]int, len(rm.neighbors))
	for i, nb := range rm.neighbors {
		d.Neighbors[i] = append([]int{}, nb...)
	}
	rm.mu.RUnlock()
	// Verdicts accumulate one entry per (scene, edge) for the life of the
	// robot; cap what the file carries. The set is a cache - arbitrary
	// truncation only costs re-validation - and cleanValid (bounded by edge
	// count) carries the transferable knowledge regardless.
	const maxPersistedVerdicts = 20000
	rm.sceneVerdicts.Range(func(k, v any) bool {
		kk := k.(roadmapVerdictKey)
		d.Verdicts = append(d.Verdicts, roadmapDiskVerdict{Scene: kk.scene, A: kk.a, B: kk.b, Clear: v.(bool)})
		return len(d.Verdicts) < maxPersistedVerdicts
	})
	d.Smoothed = map[string][][]float64{}
	rm.smoothedCache.Range(func(k, v any) bool {
		d.Smoothed[fmt.Sprintf("%d", k.(uint64))] = v.([][]float64)
		return true
	})
	rm.edgeVoxMu.RLock()
	if len(rm.edgeVox) > 0 {
		d.EdgeVox = make(map[string][]uint64, len(rm.edgeVox))
		for k, vox := range rm.edgeVox {
			d.EdgeVox[fmt.Sprintf("%d", k)] = vox
		}
	}
	rm.edgeVoxMu.RUnlock()
	rm.cleanValid.Range(func(k, v any) bool {
		kk := k.(drmVerdictKey)
		d.CleanValid = append(d.CleanValid, roadmapDiskClean{Constraints: kk.constraints, A: kk.a, B: kk.b})
		return true
	})
	data, err := json.Marshal(&d)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		return
	}
	logger.Debugf("roadmap saved to %s: %d nodes, %d verdicts", path, len(d.Flat), len(d.Verdicts))
}
