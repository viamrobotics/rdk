package motionplan

import (
	"math"
	"sync"
	"sync/atomic"

	"go.viam.com/rdk/spatialmath"
)

// CollisionCache holds planner-level temporal-coherence state for collision
// queries within a single planning request. Owned by planContext and threaded
// down through the constraint checker. Holds two pieces of state:
//
//  1. Geometry-pair "last-violated" hints — one slot per constraint type. When
//     a constraint check finds (geomA, geomB) in collision, it stores that pair
//     so the next call to the same constraint tries that pair first.
//  2. Edge-result memoization for checkPath — RRT-Connect rewire and path
//     smoothing re-check the same interpolated edges repeatedly; the verdict
//     for collision-free edges is cached here.
//
// Per-mesh witness caches (the inner-loop temporal-coherence short-circuit)
// live on spatialmath.Mesh.state, not here. Threading a cache through the
// motionplan call chain was measurably slower than direct field access on
// the mesh.
type CollisionCache struct {
	// obstaclePairHint, selfPairHint, robotPairHint each cache the
	// most-recently-violated geometry-label pair for one constraint type.
	// Lock-free reads via atomic; stale reads are harmless.
	obstaclePairHint atomic.Pointer[[2]string]
	selfPairHint     atomic.Pointer[[2]string]
	robotPairHint    atomic.Pointer[[2]string]

	// edgeResults memoizes the outcome of CheckStateConstraintsAcrossSegmentFS
	// for an interpolated edge. Key is the canonical {hashA, hashB} pair —
	// uint64 fits inside sync.Map's interface{} slot without allocation.
	edgeResults sync.Map // edgeResultKey -> edgeResultValue

	// sdfs caches voxel distance fields per static geometry set (see SDFFor).
	sdfs sync.Map // uint64 -> *spatialmath.VoxelSDF
}

// NewCollisionCache constructs an empty cache. Safe for concurrent use.
func NewCollisionCache() *CollisionCache {
	return &CollisionCache{}
}

// edgeResultKey identifies an interpolated edge by hashed-config endpoints.
// Symmetric (edges are bidirectional) — sorting the hashes canonicalizes the key.
type edgeResultKey struct {
	a, b uint64
}

// edgeResultValue records whether an edge was found collision-free. Only clear
// results are cached — failed-edge results would need to be keyed by the buffer
// and resolution used at the time, which varies across callers.
type edgeResultValue struct {
	isClear bool
}

// LookupEdgeResult returns whether the edge between two configurations has been
// previously verified collision-free. Returns (false, false) for "no cached result".
// Caller hashes must be deterministic for the same inputs across calls.
func (c *CollisionCache) LookupEdgeResult(hashA, hashB uint64) (isClear, ok bool) {
	if c == nil {
		return false, false
	}
	if hashA > hashB {
		hashA, hashB = hashB, hashA
	}
	v, ok := c.edgeResults.Load(edgeResultKey{a: hashA, b: hashB})
	if !ok {
		return false, false
	}
	return v.(edgeResultValue).isClear, true
}

// StoreEdgeResult records that an edge was found collision-free.
func (c *CollisionCache) StoreEdgeResult(hashA, hashB uint64, isClear bool) {
	if c == nil {
		return
	}
	if hashA > hashB {
		hashA, hashB = hashB, hashA
	}
	c.edgeResults.Store(edgeResultKey{a: hashA, b: hashB}, edgeResultValue{isClear: isClear})
}

// SDFFor returns the voxel distance field for the given static geometry set,
// building it on first request. Keyed by a hash of the geometries' labels and
// poses so all PlanSegmentContexts of a plan (which share their static scene)
// share one field. sdfResolutionMM balances build time (~70ms for a dual-arm
// scene at 10mm) against the conservative margin subtracted from every query
// (half the voxel diagonal, ~8.7mm at 10mm).
func (c *CollisionCache) SDFFor(static []spatialmath.Geometry) *spatialmath.VoxelSDF {
	if c == nil || len(static) == 0 {
		return nil
	}
	key := staticSetHash(static)
	if v, ok := c.sdfs.Load(key); ok {
		return v.(*spatialmath.VoxelSDF)
	}
	sdf := spatialmath.NewVoxelSDF(static, sdfResolutionMM)
	if sdf == nil {
		return nil
	}
	actual, _ := c.sdfs.LoadOrStore(key, sdf)
	return actual.(*spatialmath.VoxelSDF)
}

const sdfResolutionMM = 10.0

// staticSetHash fingerprints a static geometry set by label and pose. The
// per-geometry hashes are combined commutatively because callers assemble the
// set from map iteration - the same scene must hash identically regardless of
// geometry order, or every segment context would rebuild the field.
func staticSetHash(geoms []spatialmath.Geometry) uint64 {
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
		total += h
	}
	return total
}
