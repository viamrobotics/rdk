package coverage

import (
	"bufio"
	"errors"
	"os"
	"math"
	"sort"
	"fmt"
	//~ "context"
	//~ "errors"

	//~ "go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	
	"github.com/chenzhekl/goply"
	"github.com/golang/geo/r3"
)

type plane struct {
	pt, normal r3.Vector
}

func ReadPLY(path string) (*spatialmath.Mesh,  error) {
	readerRaw, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	reader := bufio.NewReader(readerRaw)
	ply := goply.New(reader)
	vertices := ply.Elements("vertex")
	faces := ply.Elements("face")
	triangles := []*spatialmath.Triangle{}
	for _, face := range faces {
		
		pts := []r3.Vector{}
		idxIface := face["vertex_indices"]
		for _, i := range idxIface.([]interface{}) {
			pts = append(pts, r3.Vector{
				1000*vertices[int(i.(uint32))]["x"].(float64),
				1000*vertices[int(i.(uint32))]["y"].(float64),
				1000*vertices[int(i.(uint32))]["z"].(float64)})
		}
		if len(pts) != 3 {
			return nil, errors.New("triangle did not have three points")
		}
		tri := spatialmath.NewTriangle(pts[0], pts[1], pts[2])
		triangles = append(triangles, tri)
	}
	return spatialmath.NewMesh(spatialmath.NewZeroPose(), triangles), nil
}

// Naive algorithm to find waypoints on a mesh
func MeshWaypoints(
	mesh *spatialmath.Mesh,
	step, trim float64, // Step is how far apart the up/down strokes will be. Trim will stay that far back from the edges of the mesh
	source spatialmath.Pose, // Used to determine which side of mesh to write on
) ([]spatialmath.Pose, error) {
	meshNorm := calcMeshNormal(mesh)
	normPose := spatialmath.NewPoseFromOrientation(&spatialmath.OrientationVector{OX: meshNorm.X, OY: meshNorm.Y, OZ: meshNorm.Z})
	if step <= 0 {
		return nil, errors.New("step must be >0")
	}
	
	// Rotate mesh to project onto X-Y plane, and determine limits of rotated mesh
	xLim := referenceframe.Limit{Min: math.Inf(1), Max: math.Inf(-1)}
	yLim := referenceframe.Limit{Min: math.Inf(1), Max: math.Inf(-1)}
	rotTriangles := []*spatialmath.Triangle{}
	vMap := map[r3.Vector][]*spatialmath.Triangle{}
	for _, t := range mesh.Triangles() {
		pts := []r3.Vector{}
		for _, pt := range t.Points() {
			rotPt := spatialmath.Compose(spatialmath.PoseInverse(normPose), spatialmath.NewPoseFromPoint(pt)).Point()
			if rotPt.Y > yLim.Max {
				yLim.Max = rotPt.Y
			}
			if rotPt.Y < yLim.Min {
				yLim.Min = rotPt.Y
			}
			if rotPt.X > xLim.Max {
				xLim.Max = rotPt.X
			}
			if rotPt.X < xLim.Min {
				xLim.Min = rotPt.X
			}
			pts = append(pts, rotPt)
		}
		tri := spatialmath.NewTriangle(pts[0], pts[1], pts[2])
		vMap[pts[0]] = append(vMap[pts[0]], tri)
		vMap[pts[1]] = append(vMap[pts[1]], tri)
		vMap[pts[2]] = append(vMap[pts[2]], tri)
		rotTriangles = append(rotTriangles, tri)
	}
	strokes := [][]spatialmath.Pose{}
	for xVal := xLim.Min + step + trim ; xVal < xLim.Max - trim; xVal += step {
		stroke := planePoses(
			rotTriangles,
			plane{pt: r3.Vector{xVal,0,0}, normal: r3.Vector{1,0,0}},
			source,
		)
		// Sort by Y value
		sort.Slice(stroke, func(i, j int) bool {
			return stroke[i].Point().Y > stroke[j].Point().Y
		})
		strokes = append(strokes, stroke)
	}
	// Invert every other stroke to make a contiguous zigzag
	finalPath := []spatialmath.Pose{}
	flip := false
	// Un-rotate poses back to original mesh position
	fmt.Println("got ", len(strokes), " strokes")
	for _, stroke := range strokes {
		if flip {
			for i := len(stroke)-1; i >= 0; i-- {
				if stroke[i].Point().Y > xLim.Max - trim || stroke[i].Point().Y < xLim.Min + trim {
					continue
				}
				finalPath = append(finalPath, spatialmath.Compose(normPose, stroke[i]))
			}
		} else {
			for _, pose := range stroke {
				if pose.Point().Y > xLim.Max - trim || pose.Point().Y < xLim.Min + trim {
					continue
				}
				finalPath = append(finalPath, spatialmath.Compose(normPose, pose))
			}
		}
		flip = !flip
	}
	
	return finalPath, nil
}

func planePoses(triangles []*spatialmath.Triangle, p plane, approachPose spatialmath.Pose) []spatialmath.Pose {
	normalPoses := []spatialmath.Pose{}
	approachVec := approachPose.Orientation().OrientationVectorRadians().Vector()
	
	for _, t := range triangles {
		tp1, tp2, intersects := t.TrianglePlaneIntersectingSegment(p.pt, p.normal)
		if !intersects || tp1.ApproxEqual(tp2) {
			continue
		}
		// There are two normals to each triangle; get both and use the one closest to the approach orientation
		triVec := t.Normal()
		dist := approachVec.Sub(triVec).Norm2()
		tNormInv := triVec.Mul(-1)
		if approachVec.Sub(tNormInv).Norm2() < dist {
			triVec = tNormInv
		}
		approachOrient := &spatialmath.OrientationVector{OX: triVec.X, OY: triVec.Y, OZ: triVec.Z}
		// 5% point
		pose1 := spatialmath.NewPose(
			tp1.Sub(tp1.Sub(tp2).Mul(0.05)),
			approachOrient,
		)
		// 95% point
		pose2 := spatialmath.NewPose(
			tp1.Sub(tp1.Sub(tp2).Mul(0.95)),
			approachOrient,
		)
		normalPoses = append(normalPoses, pose1, pose2)
	}
	return normalPoses
}

// Won't work on e.g. a sphere. Has to be "facing" somewhere.
func calcMeshNormal(mesh *spatialmath.Mesh) r3.Vector {
	normal := r3.Vector{}
	// Find mean normal of mesh
	// TODO: Scale by triangle area?
	for _, t := range mesh.Triangles() {
		tNorm := t.Normal()
		//~ fmt.Println("tNorm", tNorm)
		// Ensure same hemisphere
		if tNorm.Z < 0 {
			normal = normal.Add(tNorm.Mul(-1))
		}else {
			normal = normal.Add(tNorm)
		}
	}
	normal = normal.Normalize()
	return normal
}

// CalculateNeighbors returns a map of triangle indices to their neighboring triangle indices.
// Two triangles are considered neighbors if they share an edge.
func CalculateNeighbors(triangles []*spatialmath.Triangle) map[int][]int {
	neighbors := make(map[int][]int)

	// Helper function to check if two line segments overlap
	segmentsOverlap := func(a0, a1, b0, b1 r3.Vector) bool {
		const epsilon = 1e-10 // Small value for floating-point comparisons

		// Check if segments are collinear
		dir := a1.Sub(a0)
		if dir.Norm2() < epsilon {
			return false // Degenerate segment
		}

		// Check if b0 and b1 lie on the line of a0-a1
		crossB0 := b0.Sub(a0).Cross(dir)
		crossB1 := b1.Sub(a0).Cross(dir)
		if crossB0.Norm2() > epsilon || crossB1.Norm2() > epsilon {
			return false // Not collinear
		}

		// Project onto the line
		dirNorm := dir.Norm()
		t0 := b0.Sub(a0).Dot(dir) / dirNorm
		t1 := b1.Sub(a0).Dot(dir) / dirNorm

		// Check overlap
		return (t0 >= -epsilon && t0 <= dirNorm+epsilon) ||
			(t1 >= -epsilon && t1 <= dirNorm+epsilon) ||
			(t0 <= -epsilon && t1 >= dirNorm+epsilon)
	}

	// Helper function to check if two triangles share an edge
	sharesEdge := func(t1, t2 *spatialmath.Triangle) bool {
		t1p := t1.Points()
		t2p := t2.Points()
		t1Edges := [][2]r3.Vector{
			{t1p[0], t1p[1]},
			{t1p[1], t1p[2]},
			{t1p[2], t1p[0]},
		}
		t2Edges := [][2]r3.Vector{
			{t2p[0], t2p[1]},
			{t2p[1], t2p[2]},
			{t2p[2], t2p[0]},
		}

		for _, e1 := range t1Edges {
			for _, e2 := range t2Edges {
				if segmentsOverlap(e1[0], e1[1], e2[0], e2[1]) {
					return true
				}
			}
		}
		return false
	}

	// Check each triangle against all other triangles
	for i := range triangles {
		neighbors[i] = make([]int, 0)
		for j := range triangles {
			if i != j && sharesEdge(triangles[i], triangles[j]) {
				neighbors[i] = append(neighbors[i], j)
			}
		}
	}

	return neighbors
}
