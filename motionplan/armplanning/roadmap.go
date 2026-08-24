package armplanning

import (
	"container/heap"
	"context"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
	"sync"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
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

	// mu guards flat and neighbors: queries hold it for read, harvesting
	// appends under write.
	mu   sync.RWMutex
	flat [][]float64
	// neighbors is the union of joint-space and workspace candidate edges.
	neighbors [][]int

	// sceneVerdicts caches per-scene edge validity: key -> bool.
	sceneVerdicts sync.Map // roadmapVerdictKey -> bool

	buildOnce sync.Once
	buildErr  error
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
	v, _ := roadmapRegistry.LoadOrStore(key, &roadmap{frames: frames})
	rm := v.(*roadmap)
	rm.buildOnce.Do(func() {
		rm.buildErr = rm.build(psc, logger)
	})
	if rm.buildErr != nil {
		return nil
	}
	return rm
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
		}
		if budget <= 0 {
			return false
		}
		budget--
		validated++
		ok := psc.CheckPath(ctx, cfgOf(a), cfgOf(b), true, nil) == nil
		if cacheable {
			rm.sceneVerdicts.Store(key, ok)
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
// structure: the obstacle set and the non-chain part of the configuration.
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
func (rm *roadmap) harvest(scene uint64, path []*referenceframe.LinearInputs, logger logging.Logger) {
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
	rm.harvest(pm.roadmapSceneKey(psc, rm), steps, logger)
}
