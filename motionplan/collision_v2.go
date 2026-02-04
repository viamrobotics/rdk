package motionplan

import (
	//~ "fmt"
	"math"
	//~ "strconv"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	//~ "go.viam.com/rdk/utils"
)

// GeometryGroup is a struct that stores a set of geometries, to be used for collision detection
type GeometryGroup struct {
	// TODO: This may be able to be faster by putting all geometries in a BVH
	geometries map[string]spatialmath.Geometry
}

// NewGeometryGroup instantiates a GeometryGroup with the x and y geometry sets.
func NewGeometryGroup(geometries []spatialmath.Geometry, collisionBufferMM float64) (*GeometryGroup, error) {
	geomMap, err := createUniqueCollisionMap(geometries)
	if err != nil {
		return nil, err
	}
	return &GeometryGroup{
		geometries:         geomMap,
	}, nil
}

func (gg *GeometryGroup) SelfCollisions(
	fs *referenceframe.FrameSystem,
	allowedCollisions []*Collision,
	collisionBufferMM float64,
) ([]*Collision, error) {
	return gg.CollidesWith(gg, fs, allowedCollisions, collisionBufferMM)
}

func (gg *GeometryGroup) CollidesWith(
	other *GeometryGroup,
	fs *referenceframe.FrameSystem,
	allowedCollisions []*Collision,
	collisionBufferMM float64,
) ([]*Collision, error) {
	
	ignoreList := makeAllowedCollisionsLookup(allowedCollisions)
	
	collisions := []*Collision{}
	
	for xName, xGeometry := range gg.geometries {
		for yName, yGeometry := range other.geometries {
			
			if _, ok := ignoreList[yName]; ok && ignoreList[yName][xName] {
				// We are comparing to ourselves and we already did this check in the other order
				continue
			}
			if _, ok := ignoreList[xName]; !ok {
				ignoreList[xName] = map[string]bool{}
			}
			ignoreList[xName][yName] = true
			
			if skipCollisionCheck(fs, xName, yName) {
				continue
			}
			
			dist, err := checkCollision(xGeometry, yGeometry, collisionBufferMM)
			if err != nil {
				return nil, err
			}
			if dist < 0 {
				collisions = append(collisions, &Collision{name1: xName, name2: yName, penetrationDepth: dist})
			}
		}
	}
	
	return collisions, nil
}

// checkCollision takes a pair of geometries and returns the distance between them.
// If this number is less than the CollisionBuffer they can be considered to be in collision.
func checkCollision(x, y spatialmath.Geometry, collisionBufferMM float64) (float64, error) {
	col, d, err := x.CollidesWith(y, collisionBufferMM)
	if err != nil {
		col, d, err = y.CollidesWith(x, collisionBufferMM)
		if err != nil {
			return math.Inf(-1), err
		}
	}
	if col {
		return math.Inf(-1), err
	}

	return d, err
}

func makeAllowedCollisionsLookup(allowedCollisions []*Collision) map[string]map[string]bool {
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
