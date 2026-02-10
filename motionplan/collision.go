package motionplan

import (
	"fmt"
	"math"
	"strconv"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

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
// ignoring allowed collisions. It will return -infinity for minDistance if there is a collision, otherwise a lower-bound estimate
// of the closest distance between non-colliding geometries.
// If collectAllCollisions is false it will return early after the first collision found. Otherwise it will return all found collisions.
// isSelfCollision should be true when gg and other represent the same set of geometries (to avoid double-counting pairs).
func CheckCollisions(
	gg, other []spatialmath.Geometry,
	allowedCollisions []Collision,
	collisionBufferMM float64,
	collectAllCollisions bool, // Allows us to exit early and skip lots of unnecessary computation
	isSelfCollision bool,
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

	// Check each geometry in gg against each in other, unless `skipCollisionCheck` says we shouldn't.
	for xName, xGeometry := range ggMap {
		for yName, yGeometry := range otherMap {
			if skipCollisionCheck(ignoreList, xName, yName) {
				continue
			}

			// For self collision checks, mark this pair as checked so we don't check it again in the other direction.
			if isSelfCollision {
				if _, ok := ignoreList[xName]; !ok {
					ignoreList[xName] = map[string]bool{}
				}
				ignoreList[xName][yName] = true
			}

			dist, err := checkCollision(xGeometry, yGeometry, collisionBufferMM)
			if err != nil {
				return nil, math.Inf(-1), err
			}
			// If we have a collision, store it, and return if the caller wants to fast fail.
			if math.IsInf(dist, -1) {
				collisions = append(collisions, Collision{name1: xName, name2: yName})
				if !collectAllCollisions {
					return collisions, dist, nil
				}
			}
			if dist < minDistance {
				minDistance = dist
			}
		}
	}

	return collisions, minDistance, nil
}

// checkCollision takes a pair of geometries and returns the reported (lower bound estimate) distance between them if they are not
// in collision. If they are in collision, returns math.Inf(-1).
func checkCollision(x, y spatialmath.Geometry, collisionBufferMM float64) (float64, error) {
	col, d, err := x.CollidesWith(y, collisionBufferMM)
	if err != nil {
		col, d, err = y.CollidesWith(x, collisionBufferMM)
		if err != nil {
			return math.Inf(-1), err
		}
	}
	if col {
		return math.Inf(-1), nil
	}
	return d, nil
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

func firstMovingParentOrself(fs *referenceframe.FrameSystem, f referenceframe.Frame) referenceframe.Frame {
	var err error
	for f != fs.World() {
		if len(f.DoF()) > 0 {
			return f
		}

		f, err = fs.Parent(f)
		if err != nil {
			panic(err) // should be impossible
		}
	}
	return f
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
	return false
}
