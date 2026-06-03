package motionplan

import (
	"fmt"
	"math"
	"strconv"
	"sync/atomic"
	"time"

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

// CheckCollisions checks whether any geometries in one set collide with any geometries in another,
// ignoring allowed collisions. It will return a lower-bound estimate of the closest distance between non-colliding geometries.
// If collectAllCollisions is false it will return early after the first collision found. Otherwise it will return all found collisions.
func CheckCollisions(
	gg, other []spatialmath.Geometry,
	allowedCollisions []Collision,
	collisionBufferMM float64,
	collectAllCollisions bool, // Allows us to exit early and skip lots of unnecessary computation
	logger logging.Logger,
) ([]Collision, float64, error) {
	return checkCollisionsHinted(gg, other, allowedCollisions, collisionBufferMM, collectAllCollisions, nil, logger)
}

// checkCollisionsHinted is the workhorse for CheckCollisions plus an optional
// "last-violated pair" hint. When hint is non-nil and the previously-violated
// pair still exists, that pair is checked first; on a new collision the hint
// is atomically updated. Per-mesh witness caching for the inner-loop short-
// circuit lives on spatialmath.Mesh itself; no plumbing needed here.
func checkCollisionsHinted(
	gg, other []spatialmath.Geometry,
	allowedCollisions []Collision,
	collisionBufferMM float64,
	collectAllCollisions bool,
	hint *atomic.Pointer[[2]string],
	logger logging.Logger,
) ([]Collision, float64, error) {
	ggMap, err := createUniqueCollisionMap(gg)
	if err != nil {
		return nil, math.Inf(-1), err
	}
	otherMap, err := createUniqueCollisionMap(other)
	if err != nil {
		return nil, math.Inf(-1), err
	}

	ignoreList := makeAllowedCollisionsLookup(allowedCollisions)

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

	checkOnePair := func(xName, yName string, xGeometry, yGeometry spatialmath.Geometry) (bool, error) {
		start := time.Now()
		isCollision, distance, err := xGeometry.CollidesWith(yGeometry, collisionBufferMM)
		if err != nil {
			isCollision, distance, err = yGeometry.CollidesWith(xGeometry, collisionBufferMM)
			if err != nil {
				return false, err
			}
		}
		if elapsed := time.Since(start); elapsed > slowCollisionThreshold {
			rp := relativePoseHash(xGeometry, yGeometry)
			logger.Debugf("slow collision check %v: %s vs %s collides=%v dist=%.4f rposeHash=%x",
				elapsed, xName, yName, isCollision, distance, rp)
		}
		if isCollision {
			return recordCollision(xName, yName), nil
		}
		minDistance = min(minDistance, distance)
		return false, nil
	}

	// Hint fast path: try the previously-violated pair first.
	if hint != nil {
		if h := hint.Load(); h != nil {
			tryHint := func(xName, yName string) (bool, bool, error) {
				xGeom, ok := ggMap[xName]
				if !ok {
					return false, false, nil
				}
				yGeom, ok := otherMap[yName]
				if !ok {
					return false, false, nil
				}
				if skipCollisionCheck(ignoreList, xName, yName) {
					return false, true, nil
				}
				stop, err := checkOnePair(xName, yName, xGeom, yGeom)
				return stop, true, err
			}
			for _, pair := range [2][2]string{{h[0], h[1]}, {h[1], h[0]}} {
				stop, _, err := tryHint(pair[0], pair[1])
				if err != nil {
					return nil, 0, err
				}
				if stop {
					return collisions, math.Inf(1), nil
				}
			}
		}
	}

	for xName, xGeometry := range ggMap {
		for yName, yGeometry := range otherMap {
			if skipCollisionCheck(ignoreList, xName, yName) {
				continue
			}
			stop, err := checkOnePair(xName, yName, xGeometry, yGeometry)
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

// Process a []Collision into a map for easy lookups.
func makeAllowedCollisionsLookup(allowedCollisions []Collision) map[string]map[string]bool {
	ignoreList := map[string]map[string]bool{}
	for _, collision := range allowedCollisions {
		if _, ok := ignoreList[collision.name1]; !ok {
			ignoreList[collision.name1] = map[string]bool{}
		}
		if _, ok := ignoreList[collision.name2]; !ok {
			ignoreList[collision.name2] = map[string]bool{}
		}
		ignoreList[collision.name1][collision.name2] = true
		ignoreList[collision.name2][collision.name1] = true
	}
	return ignoreList
}

func createUniqueCollisionMap(geoms []spatialmath.Geometry) (map[string]spatialmath.Geometry, error) {
	unnamedCnt := 0
	geomMap := map[string]spatialmath.Geometry{}

	for _, geom := range geoms {
		label := geom.Label()
		if label == "" {
			label = unnamedCollisionGeometryPrefix + strconv.Itoa(unnamedCnt)
			unnamedCnt++
		}
		if _, present := geomMap[label]; present {
			return nil, referenceframe.NewDuplicateGeometryNameError(label)
		}
		geomMap[label] = geom
	}
	return geomMap, nil
}

func skipCollisionCheck(ignoreList map[string]map[string]bool, xName, yName string) bool {
	// Skip comparing a geometry to itself
	if xName == yName {
		return true
	}

	if _, ok := ignoreList[yName]; ok && ignoreList[yName][xName] {
		// Already checked this pair in the other order
		return true
	}

	// We're going to decide if x->y collides. We will not need to check if y->x collides. Mutate
	// the ignoreList to (potentially) avoid that reverse computation.
	for _, pair := range [][2]string{{xName, yName}, {yName, xName}} {
		left, right := pair[0], pair[1]
		if _, ok := ignoreList[left]; !ok {
			ignoreList[left] = map[string]bool{}
		}
		ignoreList[left][right] = true
	}

	return false
}
