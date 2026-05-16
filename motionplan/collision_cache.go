package motionplan

import (
	"sync"
	"sync/atomic"
)

// CollisionCache holds planner-level temporal-coherence state for collision
// queries within a single planning request. Owned by planContext and threaded
// down through the constraint checker. Holds two pieces of state that benefit
// from living above the geometry library:
//
//  1. Geometry-pair "last-violated" hints — one slot per constraint type. When
//     a constraint check finds (geomA, geomB) in collision, it stores that pair
//     so the next call to the same constraint tries that pair first.
//  2. Edge-result memoization for checkPath — RRT-Connect rewire and path
//     smoothing re-check the same interpolated edges repeatedly; the verdict
//     for collision-free edges is cached here.
//
// Per-triangle witness caching deliberately lives one layer below, on the
// *spatialmath.Mesh itself: an external interface-mediated cache costs ~30 ns
// of dispatch+lookup overhead per call, which adds up across the N·M pair
// checks per planning state.
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
