package spatialmath

import (
	"fmt"
	"math"
	"sort"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
)

// ConservativeDecimate returns a mesh with at most the requested number of triangles.
// If this mesh has more triangles than requested, it is replaced by an enclosing conservative hull mesh
// that guarantees containment and avoids collision false negatives.
func (m *Mesh) ConservativeDecimate(targetTriangles int) (*Mesh, error) {
	if targetTriangles <= 0 {
		return nil, errors.New("target triangle count must be positive")
	}
	if len(m.triangles) == 0 {
		return nil, errors.New("cannot decimate mesh with no triangles")
	}
	if targetTriangles < len(boxTriangles) {
		return nil, errors.Errorf("target triangle count must be at least %d", len(boxTriangles))
	}
	if len(m.triangles) <= targetTriangles {
		return m, nil
	}

	enclosingTris, err := conservativeHullDecimateTriangles(m.triangles, targetTriangles)
	if err != nil {
		// Fallback for degenerate/pathological meshes.
		minPt, maxPt := localAABBForTriangles(m.triangles)
		enclosingTris = tessellatedAABBTriangles(minPt, maxPt, targetTriangles)
	}

	decimated := &Mesh{
		pose:      m.pose,
		triangles: enclosingTris,
		label:     m.label,
		fileType:  plyType,
	}
	decimated.rawBytes = decimated.TrianglesToPLYBytes(false)
	decimated.SetOriginalFilePath(m.originalFilePath)
	return decimated, nil
}

type quickHullFace struct {
	a, b, c int
	normal  r3.Vector
	offset  float64
	outside []int
	deleted bool
}

func conservativeHullDecimateTriangles(triangles []*Triangle, targetTriangles int) ([]*Triangle, error) {
	if targetTriangles < len(boxTriangles) {
		return nil, errors.Errorf("target triangle count must be at least %d", len(boxTriangles))
	}

	vertices := uniqueTriangleVertices(triangles)
	if len(vertices) < 4 {
		return nil, errors.New("need at least 4 unique vertices to build conservative hull")
	}

	// For triangular convex hulls, F <= 2V-4. Keep V bounded so F stays <= target.
	vertexBudget := (targetTriangles + 4) / 2
	if vertexBudget < 4 {
		vertexBudget = 4
	}

	hullInput := vertices
	if len(hullInput) > vertexBudget {
		hullInput = selectSupportVertices(vertices, vertexBudget)
	}

	faces, hullPoints, err := quickHull3D(hullInput, floatEpsilon)
	if err != nil {
		return nil, err
	}
	hullTris := hullFacesToTriangles(faces, hullPoints)
	if len(hullTris) == 0 {
		return nil, errors.New("failed to build conservative hull")
	}

	// Strict containment: if sampled hull misses extremes, scale it outward just enough to contain all vertices.
	hullCenter := centroidOfPoints(hullPoints)
	scale := requiredHullScale(vertices, faces, hullCenter)
	if scale > 1.0 {
		hullTris = scaleTrianglesAboutPoint(hullTris, hullCenter, scale*(1.0+1e-9))
	}

	if len(hullTris) > targetTriangles {
		return nil, errors.Errorf("conservative hull has %d triangles, expected <= %d", len(hullTris), targetTriangles)
	}
	return hullTris, nil
}

func uniqueTriangleVertices(triangles []*Triangle) []r3.Vector {
	pointMap := make(map[string]r3.Vector)
	for _, tri := range triangles {
		for _, pt := range tri.Points() {
			key := fmt.Sprintf("%.10f,%.10f,%.10f", pt.X, pt.Y, pt.Z)
			pointMap[key] = pt
		}
	}
	out := make([]r3.Vector, 0, len(pointMap))
	for _, pt := range pointMap {
		out = append(out, pt)
	}
	return out
}

func selectSupportVertices(vertices []r3.Vector, maxPoints int) []r3.Vector {
	if len(vertices) <= maxPoints {
		out := make([]r3.Vector, len(vertices))
		copy(out, vertices)
		return out
	}

	directions := fibonacciSphereDirections(maxPoints)
	directions = append(directions,
		r3.Vector{X: 1, Y: 0, Z: 0}, r3.Vector{X: -1, Y: 0, Z: 0},
		r3.Vector{X: 0, Y: 1, Z: 0}, r3.Vector{X: 0, Y: -1, Z: 0},
		r3.Vector{X: 0, Y: 0, Z: 1}, r3.Vector{X: 0, Y: 0, Z: -1},
	)

	supportMap := make(map[string]r3.Vector)
	for _, dir := range directions {
		best := vertices[0]
		bestDot := best.Dot(dir)
		for i := 1; i < len(vertices); i++ {
			d := vertices[i].Dot(dir)
			if d > bestDot {
				bestDot = d
				best = vertices[i]
			}
		}
		key := fmt.Sprintf("%.10f,%.10f,%.10f", best.X, best.Y, best.Z)
		supportMap[key] = best
	}

	support := make([]r3.Vector, 0, len(supportMap))
	for _, pt := range supportMap {
		support = append(support, pt)
	}
	if len(support) > maxPoints {
		center := centroidOfPoints(vertices)
		sort.Slice(support, func(i, j int) bool {
			return support[i].Sub(center).Norm2() > support[j].Sub(center).Norm2()
		})
		support = support[:maxPoints]
	}

	if len(support) < 4 {
		for _, pt := range vertices {
			key := fmt.Sprintf("%.10f,%.10f,%.10f", pt.X, pt.Y, pt.Z)
			if _, ok := supportMap[key]; ok {
				continue
			}
			support = append(support, pt)
			supportMap[key] = pt
			if len(support) >= 4 {
				break
			}
		}
	}
	return support
}

func fibonacciSphereDirections(n int) []r3.Vector {
	if n <= 0 {
		return nil
	}
	if n == 1 {
		return []r3.Vector{{X: 0, Y: 0, Z: 1}}
	}

	goldenAngle := math.Pi * (3 - math.Sqrt(5))
	dirs := make([]r3.Vector, n)
	for i := range n {
		y := 1 - (2 * float64(i) / float64(n-1))
		radius := math.Sqrt(math.Max(0, 1-y*y))
		theta := goldenAngle * float64(i)
		dirs[i] = r3.Vector{
			X: math.Cos(theta) * radius,
			Y: y,
			Z: math.Sin(theta) * radius,
		}
	}
	return dirs
}

func quickHull3D(points []r3.Vector, eps float64) ([]quickHullFace, []r3.Vector, error) {
	if len(points) < 4 {
		return nil, nil, errors.New("need at least 4 points for 3D hull")
	}

	i0, i1 := 0, 0
	for i := 1; i < len(points); i++ {
		if points[i].X < points[i0].X {
			i0 = i
		}
		if points[i].X > points[i1].X {
			i1 = i
		}
	}
	if i0 == i1 {
		return nil, nil, errors.New("degenerate point set")
	}

	lineDir := points[i1].Sub(points[i0])
	i2, maxLineDist := -1, -1.0
	for i := range points {
		if i == i0 || i == i1 {
			continue
		}
		dist := lineDir.Cross(points[i].Sub(points[i0])).Norm()
		if dist > maxLineDist {
			maxLineDist = dist
			i2 = i
		}
	}
	if i2 < 0 || maxLineDist <= eps {
		return nil, nil, errors.New("points are nearly collinear")
	}

	baseNormal := PlaneNormal(points[i0], points[i1], points[i2])
	i3, maxPlaneDist := -1, -1.0
	for i := range points {
		if i == i0 || i == i1 || i == i2 {
			continue
		}
		dist := math.Abs(baseNormal.Dot(points[i].Sub(points[i0])))
		if dist > maxPlaneDist {
			maxPlaneDist = dist
			i3 = i
		}
	}
	if i3 < 0 || maxPlaneDist <= eps {
		return nil, nil, errors.New("points are nearly coplanar")
	}

	interior := centroidOfPoints([]r3.Vector{points[i0], points[i1], points[i2], points[i3]})
	faces := []quickHullFace{
		newQuickHullFace(points, i0, i1, i2, interior),
		newQuickHullFace(points, i0, i3, i1, interior),
		newQuickHullFace(points, i1, i3, i2, interior),
		newQuickHullFace(points, i2, i3, i0, interior),
	}

	tetra := map[int]struct{}{i0: {}, i1: {}, i2: {}, i3: {}}
	for pIdx := range points {
		if _, ok := tetra[pIdx]; ok {
			continue
		}
		assignPointToHullFace(points, pIdx, faces, eps)
	}

	for {
		faceIdx := -1
		for i := range faces {
			if !faces[i].deleted && len(faces[i].outside) > 0 {
				faceIdx = i
				break
			}
		}
		if faceIdx < 0 {
			break
		}

		eye := farthestOutsidePoint(points, faces[faceIdx])
		if eye < 0 {
			faces[faceIdx].outside = nil
			continue
		}

		visible := make([]int, 0)
		for i := range faces {
			if faces[i].deleted {
				continue
			}
			if facePointDistance(faces[i], points[eye]) > eps {
				visible = append(visible, i)
			}
		}
		if len(visible) == 0 {
			faces[faceIdx].outside = removePointFromSlice(faces[faceIdx].outside, eye)
			continue
		}

		horizon := make(map[[2]int]struct{})
		reassign := make(map[int]struct{})
		for _, vi := range visible {
			f := &faces[vi]
			for _, pIdx := range f.outside {
				if pIdx != eye {
					reassign[pIdx] = struct{}{}
				}
			}
			f.deleted = true
			addHorizonEdge(horizon, f.a, f.b)
			addHorizonEdge(horizon, f.b, f.c)
			addHorizonEdge(horizon, f.c, f.a)
		}
		if len(horizon) == 0 {
			continue
		}

		newFaces := make([]int, 0, len(horizon))
		for edge := range horizon {
			nf := newQuickHullFace(points, edge[0], edge[1], eye, interior)
			if nf.normal.Norm2() <= 0 {
				continue
			}
			faces = append(faces, nf)
			newFaces = append(newFaces, len(faces)-1)
		}
		if len(newFaces) == 0 {
			continue
		}

		for pIdx := range reassign {
			bestFace, bestDist := -1, eps
			for _, fi := range newFaces {
				d := facePointDistance(faces[fi], points[pIdx])
				if d > bestDist {
					bestDist = d
					bestFace = fi
				}
			}
			if bestFace >= 0 {
				faces[bestFace].outside = append(faces[bestFace].outside, pIdx)
			}
		}
	}

	return faces, points, nil
}

func newQuickHullFace(points []r3.Vector, a, b, c int, interior r3.Vector) quickHullFace {
	normal := PlaneNormal(points[a], points[b], points[c])
	if normal.Norm2() <= 0 {
		return quickHullFace{a: a, b: b, c: c}
	}
	offset := normal.Dot(points[a])
	if normal.Dot(interior)-offset > 0 {
		b, c = c, b
		normal = PlaneNormal(points[a], points[b], points[c])
		offset = normal.Dot(points[a])
	}
	return quickHullFace{
		a:      a,
		b:      b,
		c:      c,
		normal: normal,
		offset: offset,
	}
}

func facePointDistance(face quickHullFace, pt r3.Vector) float64 {
	return face.normal.Dot(pt) - face.offset
}

func assignPointToHullFace(points []r3.Vector, pIdx int, faces []quickHullFace, eps float64) {
	bestFace, bestDist := -1, eps
	for i := range faces {
		if faces[i].deleted {
			continue
		}
		d := facePointDistance(faces[i], points[pIdx])
		if d > bestDist {
			bestDist = d
			bestFace = i
		}
	}
	if bestFace >= 0 {
		faces[bestFace].outside = append(faces[bestFace].outside, pIdx)
	}
}

func farthestOutsidePoint(points []r3.Vector, face quickHullFace) int {
	bestIdx := -1
	bestDist := -1.0
	for _, pIdx := range face.outside {
		d := facePointDistance(face, points[pIdx])
		if d > bestDist {
			bestDist = d
			bestIdx = pIdx
		}
	}
	return bestIdx
}

func addHorizonEdge(horizon map[[2]int]struct{}, a, b int) {
	rev := [2]int{b, a}
	if _, ok := horizon[rev]; ok {
		delete(horizon, rev)
		return
	}
	horizon[[2]int{a, b}] = struct{}{}
}

func removePointFromSlice(points []int, target int) []int {
	out := points[:0]
	for _, p := range points {
		if p != target {
			out = append(out, p)
		}
	}
	return out
}

func hullFacesToTriangles(faces []quickHullFace, points []r3.Vector) []*Triangle {
	tris := make([]*Triangle, 0, len(faces))
	for _, face := range faces {
		if face.deleted || face.normal.Norm2() <= 0 {
			continue
		}
		tris = append(tris, NewTriangle(points[face.a], points[face.b], points[face.c]))
	}
	return tris
}

func centroidOfPoints(points []r3.Vector) r3.Vector {
	if len(points) == 0 {
		return r3.Vector{}
	}
	acc := r3.Vector{}
	for _, p := range points {
		acc = acc.Add(p)
	}
	return acc.Mul(1.0 / float64(len(points)))
}

func requiredHullScale(original []r3.Vector, faces []quickHullFace, center r3.Vector) float64 {
	scale := 1.0
	for _, face := range faces {
		if face.deleted || face.normal.Norm2() <= 0 {
			continue
		}
		centerDot := face.normal.Dot(center)
		denom := face.offset - centerDot
		if denom <= floatEpsilon {
			continue
		}
		for _, pt := range original {
			num := face.normal.Dot(pt) - centerDot
			required := (num + defaultCollisionBufferMM) / denom
			if required > scale {
				scale = required
			}
		}
	}
	return scale
}

func scaleTrianglesAboutPoint(triangles []*Triangle, center r3.Vector, scale float64) []*Triangle {
	scaled := make([]*Triangle, len(triangles))
	for i, tri := range triangles {
		pts := tri.Points()
		scaled[i] = NewTriangle(
			center.Add(pts[0].Sub(center).Mul(scale)),
			center.Add(pts[1].Sub(center).Mul(scale)),
			center.Add(pts[2].Sub(center).Mul(scale)),
		)
	}
	return scaled
}

func localAABBForTriangles(triangles []*Triangle) (r3.Vector, r3.Vector) {
	minPt := r3.Vector{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)}
	maxPt := r3.Vector{X: math.Inf(-1), Y: math.Inf(-1), Z: math.Inf(-1)}
	for _, tri := range triangles {
		for _, pt := range tri.Points() {
			minPt, maxPt = expandAABB(minPt, maxPt, pt)
		}
	}
	return minPt, maxPt
}

func tessellatedAABBTriangles(minPt, maxPt r3.Vector, targetTriangles int) []*Triangle {
	triangles := meshTrianglesForAABB(minPt, maxPt)
	for len(triangles) < targetTriangles {
		idx := largestTriangleIndex(triangles)
		t0, t1 := splitTriangleOnLongestEdge(triangles[idx])
		triangles[idx] = t0
		triangles = append(triangles, t1)
	}
	return triangles
}

func meshTrianglesForAABB(minPt, maxPt r3.Vector) []*Triangle {
	center := minPt.Add(maxPt).Mul(0.5)
	half := maxPt.Sub(minPt).Mul(0.5)

	verts := make([]r3.Vector, len(boxVertices))
	for i, v := range boxVertices {
		verts[i] = r3.Vector{
			X: center.X + v.X*half.X,
			Y: center.Y + v.Y*half.Y,
			Z: center.Z + v.Z*half.Z,
		}
	}

	triangles := make([]*Triangle, 0, len(boxTriangles))
	for _, tri := range boxTriangles {
		triangles = append(triangles, NewTriangle(verts[tri[0]], verts[tri[1]], verts[tri[2]]))
	}
	return triangles
}

func largestTriangleIndex(triangles []*Triangle) int {
	largestIdx := 0
	largestArea := triangles[0].Area()
	for i := 1; i < len(triangles); i++ {
		area := triangles[i].Area()
		if area > largestArea {
			largestArea = area
			largestIdx = i
		}
	}
	return largestIdx
}

func splitTriangleOnLongestEdge(tri *Triangle) (*Triangle, *Triangle) {
	pts := tri.Points()
	edgeA, edgeB, opposite := 0, 1, 2
	longest := pts[0].Sub(pts[1]).Norm2()

	if edgeLen := pts[1].Sub(pts[2]).Norm2(); edgeLen > longest {
		edgeA, edgeB, opposite = 1, 2, 0
		longest = edgeLen
	}
	if edgeLen := pts[2].Sub(pts[0]).Norm2(); edgeLen > longest {
		edgeA, edgeB, opposite = 2, 0, 1
	}

	mid := pts[edgeA].Add(pts[edgeB]).Mul(0.5)
	return NewTriangle(pts[edgeA], mid, pts[opposite]), NewTriangle(mid, pts[edgeB], pts[opposite])
}
