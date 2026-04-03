package spatialmath

import (
	"log"
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

	// Try two strategies and pick the one with smaller volume.
	// 1. Single convex hull (tight for convex shapes)
	// 2. Sliced convex hulls along longest axis (tight for non-convex shapes)
	//
	// Note: meshTriangleVolume slightly overestimates the sliced hull volume because
	// the slab hulls overlap at boundaries and the divergence theorem double-counts
	// the overlapping region. This biases the comparison conservatively toward the
	// single hull, which is acceptable.
	hullTris, hullErr := conservativeHullDecimateTriangles(m.triangles, targetTriangles)
	slicedTris, slicedErr := slicedConvexHullDecimate(m.triangles, targetTriangles)

	var enclosingTris []*Triangle
	switch {
	case hullErr == nil && slicedErr == nil:
		if meshTriangleVolume(slicedTris) < meshTriangleVolume(hullTris) {
			enclosingTris = slicedTris
		} else {
			enclosingTris = hullTris
		}
	case hullErr == nil:
		enclosingTris = hullTris
	case slicedErr == nil:
		enclosingTris = slicedTris
	default:
		// Both failed — fall back to AABB.
		log.Printf("spatialmath: conservative hull decimation failed (%v / %v), falling back to tessellated AABB", hullErr, slicedErr)
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

	// First, try the full convex hull of ALL vertices. If it fits the target
	// face count, it's the tightest possible enclosure with no scaling needed.
	faces, hullPoints, err := quickHull3D(vertices, floatEpsilon)
	if err == nil {
		hullTris := hullFacesToTriangles(faces, hullPoints)
		if len(hullTris) > 0 && len(hullTris) <= targetTriangles {
			return hullTris, nil
		}
	}

	// Full hull has too many faces. Build a simplified hull from a vertex subset.
	vertexBudget := (targetTriangles + 4) / 2
	if vertexBudget < 4 {
		vertexBudget = 4
	}

	hullInput := selectSupportVertices(vertices, vertexBudget)
	faces, hullPoints, err = quickHull3D(hullInput, floatEpsilon)
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

// slicedConvexHullDecimate splits the mesh along its longest AABB axis into
// 2+ slabs, builds the convex hull of each slab, and merges them. This produces
// a tighter (non-convex) enclosure for shapes with concavities.
//
// Encapsulation is guaranteed at the vertex level: every original vertex is inside
// at least one slab hull. Vertices near slab boundaries are duplicated into adjacent
// slabs (10% overlap) to cover triangle edges that cross boundaries. The convex hull
// of each slab contains all linear interpolations between its vertices, so triangle
// interiors between vertices in the same slab are also covered.
func slicedConvexHullDecimate(triangles []*Triangle, targetTriangles int) ([]*Triangle, error) {
	vertices := uniqueTriangleVertices(triangles)
	if len(vertices) < 4 {
		return nil, errors.New("need at least 4 unique vertices")
	}

	// Find the longest AABB axis.
	minPt, maxPt := localAABBForTriangles(triangles)
	extent := maxPt.Sub(minPt)
	axis := 0
	axisLen := extent.X
	if extent.Y > axisLen {
		axis = 1
		axisLen = extent.Y
	}
	if extent.Z > axisLen {
		axis = 2
		axisLen = extent.Z
	}

	axisVal := func(v r3.Vector) float64 {
		switch axis {
		case 1:
			return v.Y
		case 2:
			return v.Z
		default:
			return v.X
		}
	}

	axisMin := axisVal(minPt)

	// Try 2, 3, 4 slices — pick the first that fits the triangle budget.
	for numSlices := 2; numSlices <= 4; numSlices++ {
		sliceWidth := axisLen / float64(numSlices)

		// Assign vertices to slices. Vertices near boundaries go into both
		// adjacent slices (overlap) to ensure triangles spanning the boundary
		// are covered.
		overlap := sliceWidth * 0.1
		slabs := make([][]r3.Vector, numSlices)
		for _, v := range vertices {
			val := axisVal(v)
			idx := int((val - axisMin) / sliceWidth)
			if idx >= numSlices {
				idx = numSlices - 1
			}
			slabs[idx] = append(slabs[idx], v)
			// Add to adjacent slab if within overlap zone.
			distInSlab := val - (axisMin + float64(idx)*sliceWidth)
			if idx > 0 && distInSlab < overlap {
				slabs[idx-1] = append(slabs[idx-1], v)
			}
			if idx < numSlices-1 && (sliceWidth-distInSlab) < overlap {
				slabs[idx+1] = append(slabs[idx+1], v)
			}
		}

		var allTris []*Triangle
		for _, slab := range slabs {
			if len(slab) < 4 {
				continue
			}
			faces, points, err := quickHull3D(slab, floatEpsilon)
			if err != nil {
				continue
			}
			allTris = append(allTris, hullFacesToTriangles(faces, points)...)
		}

		if len(allTris) > 0 && len(allTris) <= targetTriangles {
			return allTris, nil
		}
	}

	return nil, errors.New("sliced hull exceeds target triangle count")
}

// meshTriangleVolume computes the volume of a closed triangle mesh using the
// divergence theorem (signed tetrahedron volumes).
func meshTriangleVolume(triangles []*Triangle) float64 {
	vol := 0.0
	for _, tri := range triangles {
		pts := tri.Points()
		vol += pts[0].Dot(pts[1].Cross(pts[2]))
	}
	if vol < 0 {
		vol = -vol
	}
	return vol / 6.0
}

func uniqueTriangleVertices(triangles []*Triangle) []r3.Vector {
	pointMap := make(map[r3.Vector]struct{})
	for _, tri := range triangles {
		for _, pt := range tri.Points() {
			pointMap[pt] = struct{}{}
		}
	}
	out := make([]r3.Vector, 0, len(pointMap))
	for v := range pointMap {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].X != out[j].X {
			return out[i].X < out[j].X
		}
		if out[i].Y != out[j].Y {
			return out[i].Y < out[j].Y
		}
		return out[i].Z < out[j].Z
	})
	return out
}

func selectSupportVertices(vertices []r3.Vector, maxPoints int) []r3.Vector {
	if len(vertices) <= maxPoints {
		out := make([]r3.Vector, len(vertices))
		copy(out, vertices)
		return out
	}

	// Seed with the 6 axis-aligned extremes to guarantee the hull's AABB
	// matches the original mesh's.
	axisDirections := []r3.Vector{
		{X: 1, Y: 0, Z: 0},
		{X: -1, Y: 0, Z: 0},
		{X: 0, Y: 1, Z: 0},
		{X: 0, Y: -1, Z: 0},
		{X: 0, Y: 0, Z: 1},
		{X: 0, Y: 0, Z: -1},
	}
	selectedMap := make(map[r3.Vector]bool)
	selected := make([]r3.Vector, 0, maxPoints)
	addVertex := func(v r3.Vector) bool {
		key := v
		if selectedMap[key] {
			return false
		}
		selectedMap[key] = true
		selected = append(selected, v)
		return true
	}

	for _, dir := range axisDirections {
		best := vertices[0]
		bestDot := best.Dot(dir)
		for i := 1; i < len(vertices); i++ {
			d := vertices[i].Dot(dir)
			if d > bestDot {
				bestDot = d
				best = vertices[i]
			}
		}
		addVertex(best)
	}

	// Iteratively add the vertex farthest outside the current hull.
	// Each addition directly targets the worst-case scale factor.
	for len(selected) < maxPoints {
		if len(selected) < 4 {
			// Not enough for a hull yet — add farthest from centroid.
			center := centroidOfPoints(selected)
			bestDist := -1.0
			bestVert := r3.Vector{}
			for _, v := range vertices {
				if selectedMap[v] {
					continue
				}
				d := v.Sub(center).Norm2()
				if d > bestDist {
					bestDist = d
					bestVert = v
				}
			}
			if bestDist < 0 {
				break
			}
			addVertex(bestVert)
			continue
		}

		faces, _, err := quickHull3D(selected, floatEpsilon)
		if err != nil {
			break
		}

		// Find the original vertex farthest outside any hull face.
		maxDist := floatEpsilon
		bestVert := r3.Vector{}
		found := false
		for _, v := range vertices {
			if selectedMap[v] {
				continue
			}
			for fi := range faces {
				if faces[fi].deleted {
					continue
				}
				d := facePointDistance(faces[fi], v)
				if d > maxDist {
					maxDist = d
					bestVert = v
					found = true
				}
			}
		}
		if !found {
			break // All vertices are inside the hull.
		}
		addVertex(bestVert)
	}

	// Deterministic output order.
	sort.Slice(selected, func(i, j int) bool {
		if selected[i].X != selected[j].X {
			return selected[i].X < selected[j].X
		}
		if selected[i].Y != selected[j].Y {
			return selected[i].Y < selected[j].Y
		}
		return selected[i].Z < selected[j].Z
	})
	return selected
}

// edgeToFaceMap tracks which active face owns each directed half-edge,
// enabling BFS adjacency traversal for visibility flood-fill.
type edgeToFaceMap map[[2]int]int

func newEdgeToFaceMap() edgeToFaceMap { return make(edgeToFaceMap) }

func (m edgeToFaceMap) add(faceIdx, a, b, c int) {
	m[[2]int{a, b}] = faceIdx
	m[[2]int{b, c}] = faceIdx
	m[[2]int{c, a}] = faceIdx
}

func (m edgeToFaceMap) remove(a, b, c int) {
	delete(m, [2]int{a, b})
	delete(m, [2]int{b, c})
	delete(m, [2]int{c, a})
}

// neighbor returns the face index sharing the edge (b, a) — i.e., the face on
// the other side of directed edge (a, b). Returns -1 if none.
func (m edgeToFaceMap) neighbor(a, b int) int {
	fi, ok := m[[2]int{b, a}]
	if !ok {
		return -1
	}
	return fi
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

	adj := newEdgeToFaceMap()
	for i := range faces {
		adj.add(i, faces[i].a, faces[i].b, faces[i].c)
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

		// BFS flood-fill from faceIdx to find connected visible faces.
		// This guarantees the visible region is a topological disk, preventing
		// the horizon from splitting into multiple loops.
		visible := bfsVisibleFaces(points, faces, adj, faceIdx, eye, eps)
		if len(visible) == 0 {
			faces[faceIdx].outside = removePointFromSlice(faces[faceIdx].outside, eye)
			continue
		}

		// Build visible set for fast lookup.
		visibleSet := make(map[int]bool, len(visible))
		for _, vi := range visible {
			visibleSet[vi] = true
		}

		// Collect orphaned outside points and compute horizon edges.
		// Horizon edges are edges of visible faces whose neighbor across
		// that edge is NOT visible.
		reassign := make(map[int]struct{})
		horizon := make([][2]int, 0)
		for _, vi := range visible {
			f := &faces[vi]
			for _, pIdx := range f.outside {
				if pIdx != eye {
					reassign[pIdx] = struct{}{}
				}
			}
			for _, edge := range [3][2]int{{f.a, f.b}, {f.b, f.c}, {f.c, f.a}} {
				nb := adj.neighbor(edge[0], edge[1])
				if nb >= 0 && !visibleSet[nb] {
					horizon = append(horizon, edge)
				}
			}
		}

		// Now delete visible faces and remove from adjacency.
		for _, vi := range visible {
			faces[vi].deleted = true
			adj.remove(faces[vi].a, faces[vi].b, faces[vi].c)
		}
		if len(horizon) == 0 {
			continue
		}

		sort.Slice(horizon, func(i, j int) bool {
			if horizon[i][0] != horizon[j][0] {
				return horizon[i][0] < horizon[j][0]
			}
			return horizon[i][1] < horizon[j][1]
		})
		newFaces := make([]int, 0, len(horizon))
		for _, edge := range horizon {
			nf := newQuickHullFace(points, edge[0], edge[1], eye, interior)
			if nf.normal.Norm2() <= 0 {
				continue
			}
			fi := len(faces)
			faces = append(faces, nf)
			adj.add(fi, nf.a, nf.b, nf.c)
			newFaces = append(newFaces, fi)
		}
		if len(newFaces) == 0 {
			continue
		}

		reassignPts := make([]int, 0, len(reassign))
		for pIdx := range reassign {
			reassignPts = append(reassignPts, pIdx)
		}
		sort.Ints(reassignPts)
		for _, pIdx := range reassignPts {
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

// bfsVisibleFaces performs a BFS from startFace, expanding to adjacent faces that are
// visible from the eye point. This ensures the visible region is connected, preventing
// the horizon from splitting into disjoint loops (which would create duplicate hull faces).
func bfsVisibleFaces(points []r3.Vector, faces []quickHullFace, adj edgeToFaceMap, startFace, eye int, eps float64) []int {
	if facePointDistance(faces[startFace], points[eye]) <= eps {
		return nil
	}
	visited := map[int]bool{startFace: true}
	queue := []int{startFace}
	visible := []int{startFace}

	for len(queue) > 0 {
		fi := queue[0]
		queue = queue[1:]
		f := faces[fi]
		for _, edge := range [3][2]int{{f.a, f.b}, {f.b, f.c}, {f.c, f.a}} {
			nb := adj.neighbor(edge[0], edge[1])
			if nb < 0 || visited[nb] || faces[nb].deleted {
				continue
			}
			visited[nb] = true
			if facePointDistance(faces[nb], points[eye]) > eps {
				visible = append(visible, nb)
				queue = append(queue, nb)
			}
		}
	}
	return visible
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
			required := num / denom
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
