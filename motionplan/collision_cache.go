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

// sdfRegistry caches voxel distance fields process-wide. A field depends only
// on the static scene (keyed by the shape-aware staticSetHash), never on the
// plan, and costs ~35-70ms plus a multi-MB grid allocation to build - caching
// it per plan made every replan of an otherwise-trivial scene pay that in
// full. Small LRU: each field can be tens of MB, and a scene whose obstacles
// move between plans mints a new key every time.
type sdfRegistry struct {
	mu      sync.Mutex
	tick    uint64
	entries map[uint64]*sdfRegistryEntry
}

type sdfRegistryEntry struct {
	sdf      *spatialmath.VoxelSDF // nil records a scene the SDF cannot represent
	lastUsed uint64
}

const sdfRegistryCap = 8

var globalSDFRegistry = sdfRegistry{entries: map[uint64]*sdfRegistryEntry{}}

func (r *sdfRegistry) lookup(key uint64) (*spatialmath.VoxelSDF, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.entries[key]
	if !ok {
		return nil, false
	}
	r.tick++
	e.lastUsed = r.tick
	return e.sdf, true
}

func (r *sdfRegistry) insert(key uint64, sdf *spatialmath.VoxelSDF) *spatialmath.VoxelSDF {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e, ok := r.entries[key]; ok {
		// Another plan built the same field concurrently; keep the incumbent.
		return e.sdf
	}
	r.tick++
	r.entries[key] = &sdfRegistryEntry{sdf: sdf, lastUsed: r.tick}
	for len(r.entries) > sdfRegistryCap {
		var oldestKey uint64
		oldest := uint64(math.MaxUint64)
		for k, e := range r.entries {
			if e.lastUsed < oldest {
				oldest, oldestKey = e.lastUsed, k
			}
		}
		delete(r.entries, oldestKey)
	}
	return sdf
}

// SDFFor returns the voxel distance field for the given static geometry set,
// building it on first request anywhere in the process and sharing it across
// plans of the same scene. sdfResolutionMM balances build time (~70ms for a
// dual-arm scene at 10mm) against the conservative margin subtracted from
// every query (half the voxel diagonal, ~8.7mm at 10mm).
func (c *CollisionCache) SDFFor(static []spatialmath.Geometry) *spatialmath.VoxelSDF {
	if c == nil || len(static) == 0 {
		return nil
	}
	key := staticSetHash(static)
	if sdf, ok := globalSDFRegistry.lookup(key); ok {
		return sdf
	}
	return globalSDFRegistry.insert(key, spatialmath.NewVoxelSDF(static, sdfResolutionMM))
}

const sdfResolutionMM = 10.0

// staticSetHash fingerprints a static geometry set by label, pose, and shape
// (via Geometry.Hash, which folds in type-specific dimensions - process-wide
// sharing must not serve a stale field to a same-named, same-posed geometry
// whose size changed). The per-geometry hashes are combined commutatively
// because callers assemble the set from map iteration - the same scene must
// hash identically regardless of geometry order, or every caller would
// rebuild the field.
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
		mix(uint64(g.Hash()))
		total += h
	}
	return total
}
