package motionplan

import (
	"fmt"
	"math"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang/geo/r3"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// slowCollisionThreshold logs any single CollidesWith call that exceeds this.
// Debug aid for tracking down which mesh pairs dominate planning time.
const slowCollisionThreshold = 500 * time.Microsecond

// relativePoseHash bins both geometries' world poses into a coarse integer key
// so we can detect approximate repeats in the slow-check log. 1mm / 0.001-rad
// granularity is fine for "are we re-doing the same check" diagnostics.
func relativePoseHash(a, b spatialmath.Geometry) uint64 {
	mix := func(h uint64, p spatialmath.Pose) uint64 {
		pt := p.Point()
		o := p.Orientation().Quaternion()
		vals := [7]float64{pt.X, pt.Y, pt.Z, o.Real, o.Imag, o.Jmag, o.Kmag}
		for _, v := range vals {
			h ^= uint64(int64(v*1000)) + 0x9e3779b97f4a7c15 + (h << 6) + (h >> 2)
		}
		return h
	}
	return mix(mix(0xcbf29ce484222325, a.Pose()), b.Pose())
}

const unnamedCollisionGeometryPrefix = "unnamedCollisionGeometry_"

// Collision is a pair of strings corresponding to names of Geometry objects in collision.
type Collision struct {
	name1, name2 string
}

// collisionsEqual compares two Collisions and returns if they are equal (names can be in either order).
func collisionsEqual(c1, c2 Collision) bool {
	return (c1.name1 == c2.name1 && c1.name2 == c2.name2) || (c1.name1 == c2.name2 && c1.name2 == c2.name1)
}

func collisionSpecifications(
	pbConstraint []CollisionSpecification,
	frameSystemGeometries map[string]*referenceframe.GeometriesInFrame,
	frameNames, validGeoms map[string]bool,
) (allowedCollisions []Collision, err error) {
	// Get names of all geometries in frame system
	for frameName, geomsInFrame := range frameSystemGeometries {
		if _, ok := validGeoms[frameName]; ok {
			return nil, referenceframe.NewDuplicateGeometryNameError(frameName)
		}
		validGeoms[frameName] = true
		for _, geom := range geomsInFrame.Geometries() {
			geomName := geom.Label()

			// Ensure we're not double-adding components which only have one geometry, named identically to the component.
			if (frameName != "" && geomName == frameName) || geomName == "" {
				continue
			}
			if _, ok := validGeoms[geomName]; ok {
				return nil, referenceframe.NewDuplicateGeometryNameError(geomName)
			}
			validGeoms[geomName] = true
		}
	}

	// This allows the user to specify an entire component with sub-geometries, e.g. "myUR5arm", and the specification will apply to all
	// sub-pieces, e.g. myUR5arm:upper_arm_link, myUR5arm:base_link, etc. Individual sub-pieces may also be so addressed.
	var allowNameToSubGeoms func(cName string) ([]string, error) // Pre-define to allow recursive call
	allowNameToSubGeoms = func(cName string) ([]string, error) {
		subNames := []string{}

		// Check if an entire component is specified
		if _, ok := frameNames[cName]; ok {
			// If this is an entire component, it likely has an origin frame. Collect any origin geometries as well if so.
			// These will be the geometries that a user specified for this component in their RDK config, or via `Transforms()`
			originGeoms, err := allowNameToSubGeoms(cName + "_origin")
			if err == nil && len(originGeoms) > 0 {
				subNames = append(subNames, originGeoms...)
			}
		}

		// Check if key specified has more than one geometry associated with it. If so, gather the names of all sub-geometries.
		if geomsInFrame, ok := frameSystemGeometries[cName]; ok {
			for _, subGeom := range geomsInFrame.Geometries() {
				subNames = append(subNames, subGeom.Label())
			}
		}
		// Check if it's a single sub-component
		if validGeoms[cName] {
			subNames = append(subNames, cName)
		}
		if len(subNames) > 0 {
			return subNames, nil
		}

		// generate the list of available names to return in error message
		availNames := make([]string, 0, len(validGeoms))
		for name := range validGeoms {
			availNames = append(availNames, name)
		}

		return nil, fmt.Errorf("geometry specification allow name %s does not match any known geometries. Available: %v", cName, availNames)
	}

	// Create the structures that specify the allowed collisions
	for _, collisionSpec := range pbConstraint {
		for _, allowPair := range collisionSpec.Allows {
			allow1 := allowPair.Frame1
			allow2 := allowPair.Frame2
			allowNames1, err := allowNameToSubGeoms(allow1)
			if err != nil {
				return nil, err
			}
			allowNames2, err := allowNameToSubGeoms(allow2)
			if err != nil {
				return nil, err
			}
			for _, allowName1 := range allowNames1 {
				for _, allowName2 := range allowNames2 {
					allowedCollisions = append(allowedCollisions, Collision{name1: allowName1, name2: allowName2})
				}
			}
		}
	}
	return allowedCollisions, nil
}

// checkCollisionsHinted checks whether any geometries in one set collide with any geometries in another,
// ignoring allowed collisions. It will return a lower-bound estimate of the closest distance between non-colliding geometries.
// If collectAllCollisions is false it will return early after the first collision found. Otherwise it will return all found collisions.
// When hint is non-nil and the previously-violated pair still exists, that pair is checked first; on a new collision the hint
// is atomically updated. Per-mesh witness caching for the inner-loop short-circuit lives on spatialmath.Mesh itself; no plumbing
// needed here.
func checkCollisionsHinted(
	gg, other []spatialmath.Geometry,
	otherPre []namedGeom,
	allowed map[[2]string]bool,
	collisionBufferMM float64,
	collectAllCollisions bool,
	hint *atomic.Pointer[[2]string],
	logger logging.Logger,
) ([]Collision, float64, error) {
	// Self-collision callers pass the identical slice for both sets; that case
	// iterates the strict upper triangle so each unordered pair is visited
	// exactly once. Distinct slices are treated as disjoint sets (true for
	// every production caller); a name present in both merely costs a
	// redundant symmetric check, never a wrong answer.
	sameSet := len(gg) == len(other) && len(gg) > 0 && &gg[0] == &other[0]
	// For large allow lists, one name-set build beats scanning the list per
	// geometry; for small ones the scan is cheaper than the map.
	var allowNames map[string]bool
	if len(allowed) > 16 {
		allowNames = make(map[string]bool, len(allowed)*2)
		for k := range allowed {
			allowNames[k[0]] = true
			allowNames[k[1]] = true
		}
	}
	ggN := nameGeoms(gg, 0, allowed, allowNames)
	defer putNamedGeoms(ggN)
	otherN := ggN
	switch {
	case otherPre != nil:
		// Prebuilt set (the checker's static geometries): names, allow flags,
		// and bounding spheres computed once at checker construction instead
		// of once per state check.
		sameSet = false
		otherN = otherPre
	case !sameSet:
		otherN = nameGeoms(other, len(gg), allowed, allowNames)
		defer putNamedGeoms(otherN)
	}

	collisions := []Collision{}
	minDistance := math.Inf(1)

	recordCollision := func(xName, yName string) bool {
		n1, n2 := xName, yName
		if n1 > n2 {
			n1, n2 = n2, n1
		}
		collisions = append(collisions, Collision{name1: n1, name2: n2})
		if hint != nil {
			h := [2]string{n1, n2}
			hint.Store(&h)
		}
		return !collectAllCollisions
	}

	// The slow-check diagnostic costs two clock syscalls per pair; at millions of
	// pair checks per plan that is real wall time. Only pay it when the debug log
	// would actually be emitted, and even then sample 1 in 16 pairs — recurring
	// slow pairs still surface, without the per-pair clock tax.
	timeChecks := logger.GetLevel() <= logging.DEBUG
	var pairCounter atomic.Uint64
	checkOnePair := func(x, y *namedGeom) (bool, error) {
		xName, yName, xGeometry, yGeometry := x.name, y.name, x.g, y.g
		// Bounding-sphere broadphase: most pairs in the moving-vs-static
		// product are nowhere near each other, and even the cached narrow
		// phase costs three orders of magnitude more than this gap test.
		if x.bok && y.bok {
			if gap := x.bcenter.Sub(y.bcenter).Norm() - x.brad - y.brad; gap > collisionBufferMM {
				minDistance = min(minDistance, gap)
				return false, nil
			}
		}
		var start time.Time
		timeThis := timeChecks && pairCounter.Add(1)&15 == 0
		if timeThis {
			start = time.Now()
		}
		isCollision, distance, err := xGeometry.CollidesWith(yGeometry, collisionBufferMM)
		if err != nil {
			isCollision, distance, err = yGeometry.CollidesWith(xGeometry, collisionBufferMM)
			if err != nil {
				return false, err
			}
		}
		if timeThis {
			if elapsed := time.Since(start); elapsed > slowCollisionThreshold {
				rp := relativePoseHash(xGeometry, yGeometry)
				logger.Debugf("slow collision check %v: %s vs %s collides=%v dist=%.4f rposeHash=%x",
					elapsed, xName, yName, isCollision, distance, rp)
			}
		}
		if isCollision {
			return recordCollision(xName, yName), nil
		}
		minDistance = min(minDistance, distance)
		return false, nil
	}

	// skipPair mirrors the old skipCollisionCheck: identical names never pair,
	// and allowed pairs are skipped. Hashing an allowed key is only worth it
	// when both geometries appear somewhere in the allow list - inAllow is
	// precomputed per geometry so the common no-allows pair costs two bool
	// reads instead of a string-pair hash.
	skipPair := func(x, y *namedGeom) bool {
		if x.name == y.name {
			return true
		}
		return x.inAllow && y.inAllow && allowed[canonicalPair(x.name, y.name)]
	}

	// Hint fast path: try the previously-violated pair first.
	if hint != nil {
		if h := hint.Load(); h != nil {
			findNamed := func(set []namedGeom, name string) *namedGeom {
				for i := range set {
					if set[i].name == name {
						return &set[i]
					}
				}
				return nil
			}
			tryHint := func(xName, yName string) (bool, error) {
				x := findNamed(ggN, xName)
				if x == nil {
					return false, nil
				}
				y := findNamed(otherN, yName)
				if y == nil || skipPair(x, y) {
					return false, nil
				}
				return checkOnePair(x, y)
			}
			for _, pair := range [2][2]string{{h[0], h[1]}, {h[1], h[0]}} {
				stop, err := tryHint(pair[0], pair[1])
				if err != nil {
					return nil, 0, err
				}
				if stop {
					return collisions, math.Inf(1), nil
				}
			}
		}
	}

	for i := range ggN {
		x := &ggN[i]
		yStart := 0
		if sameSet {
			yStart = i + 1
		}
		for j := yStart; j < len(otherN); j++ {
			y := &otherN[j]
			if skipPair(x, y) {
				continue
			}
			stop, err := checkOnePair(x, y)
			if err != nil {
				return nil, math.Inf(-1), err
			}
			if stop {
				return collisions, minDistance, nil
			}
		}
	}

	return collisions, minDistance, nil
}

// namedGeom pairs a geometry with its collision name for the pair loop.
// inAllow records whether the name appears anywhere in the allowed map, so
// pairs with no possible allowance skip the string-pair hash entirely.
type namedGeom struct {
	name    string
	g       spatialmath.Geometry
	inAllow bool
	// Bounding sphere in world coordinates, computed once per geometry per
	// call: deriving it per pair made pose decomposition (DualQuaternion.Point
	// under BoundingSphere) the hottest non-GC entry in mesh-heavy scenes.
	bcenter r3.Vector
	brad    float64
	bok     bool
}

var namedGeomPool = sync.Pool{New: func() any { s := make([]namedGeom, 0, 64); return &s }}

// nameGeoms builds the named slice for one input set. Unnamed geometries get
// synthetic names; unnamedBase keeps them distinct across the two sets of one
// call. Return through putNamedGeoms.
func nameGeoms(geoms []spatialmath.Geometry, unnamedBase int, allowed map[[2]string]bool, allowNames map[string]bool) []namedGeom {
	out := (*namedGeomPool.Get().(*[]namedGeom))[:0]
	for _, g := range geoms {
		label := g.Label()
		if label == "" {
			label = unnamedCollisionGeometryPrefix + strconv.Itoa(unnamedBase)
			unnamedBase++
		}
		ng := namedGeom{name: label, g: g}
		ng.bcenter, ng.brad, ng.bok = spatialmath.BoundingSphere(g)
		switch {
		case allowNames != nil:
			ng.inAllow = allowNames[label]
		case len(allowed) > 0:
			for k := range allowed {
				if k[0] == label || k[1] == label {
					ng.inAllow = true
					break
				}
			}
		}
		out = append(out, ng)
	}
	return out
}

// prenameGeoms builds a long-lived namedGeom slice for a geometry set whose
// poses never change (the checker's static side). The synthetic-name base for
// unnamed geometries is far above anything the per-call moving side uses.
func prenameGeoms(geoms []spatialmath.Geometry, allowed map[[2]string]bool) []namedGeom {
	var allowNames map[string]bool
	if len(allowed) > 16 {
		allowNames = make(map[string]bool, len(allowed)*2)
		for k := range allowed {
			allowNames[k[0]] = true
			allowNames[k[1]] = true
		}
	}
	pooled := nameGeoms(geoms, 1<<20, allowed, allowNames)
	out := make([]namedGeom, len(pooled))
	copy(out, pooled)
	putNamedGeoms(pooled)
	return out
}

func putNamedGeoms(s []namedGeom) {
	for i := range s {
		s[i].g = nil // don't retain geometries across pool reuse
	}
	s = s[:0]
	namedGeomPool.Put(&s)
}

func canonicalPair(a, b string) [2]string {
	if a < b {
		return [2]string{a, b}
	}
	return [2]string{b, a}
}

// Process a []Collision into a map for easy lookups.
func makeAllowedCollisionsLookup(allowedCollisions []Collision) map[[2]string]bool {
	ignoreList := make(map[[2]string]bool, len(allowedCollisions))
	for _, c := range allowedCollisions {
		ignoreList[canonicalPair(c.name1, c.name2)] = true
	}
	return ignoreList
}
