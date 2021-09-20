package pointcloud

import (
	"container/list"
	"sort"
)

// LabelVoxels performs voxel plane labeling
// If a voxel contains points from one plane, voxel propagation is done to the neighboring voxels that are also planar
// and share the same plane equation
func (vg *VoxelGrid) LabelVoxels(sortedKeys []VoxelCoords, wTh, thetaTh, phiTh float64) {
	currentLabel := 1
	visited := make(map[VoxelCoords]bool)
	//nZeroWeight := 0
	for _, k := range sortedKeys {
		// If current voxel has a weight above threshold (plane data is relevant)
		// and has not been visited yet
		if vg.Voxels[k].Weight > wTh && !visited[k] && vg.Voxels[k].Label == 0 {
			// BFS traversal
			vg.labelComponentBFS(vg.Voxels[k], currentLabel, wTh, thetaTh, phiTh, visited)
			vg.maxLabel = currentLabel
			currentLabel = currentLabel + 1
		}

	}
}

// labelComponentBFS is a helper function to perform BFS per connected component
func (vg *VoxelGrid) labelComponentBFS(vox *Voxel, label int, wTh, thetaTh, phiTh float64, visited map[VoxelCoords]bool) {
	queue := list.New()
	queue.PushBack(vox.Key)
	visited[vox.Key] = true
	for queue.Len() > 0 {
		e := queue.Front() // First element
		// interface to VoxelCoords type
		coords := e.Value.(VoxelCoords)
		// Set label of Voxel
		vg.Voxels[coords].SetLabel(label)
		// Add current key to visited set
		// Get adjacent voxels
		neighbors := vg.GetAdjacentVoxels(vg.Voxels[coords])
		for _, c := range neighbors {
			// if pair voxels satisfies smoothness and continuity constraints and
			// neighbor voxel plane data is relevant enough
			// and neighbor is not visited yet
			if vg.Voxels[coords].CanMerge(vg.Voxels[c], thetaTh, phiTh) && vg.Voxels[c].Weight > wTh && !visited[c] {
				queue.PushBack(c)
				visited[c] = true
			}
		}
		queue.Remove(e)
	}
}

// GetUnlabeledVoxels gathers in a slice all voxels whose label is 0
func (vg *VoxelGrid) GetUnlabeledVoxels() []VoxelCoords {
	unlabeled := make([]VoxelCoords, 0)
	for _, vox := range vg.Voxels {
		if vox.Label == 0 {
			unlabeled = append(unlabeled, vox.Key)
		}
	}
	return unlabeled
}

// GetPlanesFromLabels returns a slice containing all the planes in the point cloud
func (vg *VoxelGrid) GetPlanesFromLabels() ([]Plane, PointCloud, error) {
	pointsByLabel := make(map[int][]Point)
	keysByLabel := make(map[int][]VoxelCoords)
	seen := make(map[Vec3]bool)
	for _, vox := range vg.Voxels {
		// if voxel is entirely included in a plane, add all the points
		if vox.Label > 0 {
			keysByLabel[vox.Label] = append(keysByLabel[vox.Label], vox.Key)
			for _, pt := range vox.Points {
				p := pt.Position()
				if _, ok := seen[p]; ok { // already assigned point to another label
					continue
				} else {
					seen[p] = true
					pointsByLabel[vox.Label] = append(pointsByLabel[vox.Label], pt)
				}
			}
		} else {
			// voxel has points for either no plane or at least two planes
			// add point by point
			if len(vox.Points) == len(vox.PointLabels) {
				for ptIdx, pt := range vox.Points {
					p := pt.Position()
					if _, ok := seen[p]; ok { // already assigned point to another label
						continue
					} else {
						seen[p] = true
						ptLabel := vox.PointLabels[ptIdx]
						pointsByLabel[ptLabel] = append(pointsByLabel[ptLabel], pt)
					}
				}
			}
		}
	}

	planes := make([]Plane, 0)
	nonPlane := New()
	for label, pts := range pointsByLabel {
		if label == 0 { // create a point cloud of non-planar points
			for _, pt := range pts {
				err := nonPlane.Set(pt)
				if err != nil {
					return nil, nil, err
				}
			}
		} else { // create an array of planes
			positions := GetPositions(pts)
			normalVector := estimatePlaneNormalFromPoints(positions)
			center := GetVoxelCenter(positions)
			offset := GetOffset(center, normalVector)
			currentPlane := &voxelPlane{
				normal:    normalVector,
				center:    center,
				offset:    offset,
				points:    pts,
				voxelKeys: keysByLabel[label],
			}
			planes = append(planes, currentPlane)
		}
	}
	return planes, nonPlane, nil
}

// LabelNonPlanarVoxels labels potential planar parts in Voxels that are containing more than one plane
// if a voxel contains no plane, the minimum distance of a point to one of the surrounding plane should be above
// the threshold dTh
func (vg *VoxelGrid) LabelNonPlanarVoxels(unlabeledVoxels []VoxelCoords, dTh float64) {
	for _, k := range unlabeledVoxels {
		vox := vg.Voxels[k]
		vox.PointLabels = make([]int, len(vox.Points))
		nbVoxels := vg.GetAdjacentVoxels(vox)
		plane := vox.GetPlane()
		for i, pt := range vox.Positions() {
			dMin := 100000.0
			outLabel := 0
			for _, kNb := range nbVoxels {
				voxNb := vg.Voxels[kNb]
				if voxNb.Label > 0 {
					d := plane.Distance(Vec3(pt))
					if d < dMin {
						dMin = d
						outLabel = voxNb.Label
					}
				}
			}
			if dMin < dTh {
				vox.PointLabels[i] = outLabel
			}
		}
	}
}

// GetKeysByDecreasingOrderWeights get the voxels keys in decreasing weight order
func (vg *VoxelGrid) GetKeysByDecreasingOrderWeights() []VoxelCoords {
	// Sort voxels by weights
	s := make(VoxelSlice, 0, len(vg.Voxels))
	for _, vox := range vg.Voxels {
		s = append(s, vox)
	}
	sort.Sort(s)
	// sort in decreasing order
	ReverseVoxelSlice(s)
	// slice of keys / voxel coordinates in decreasing order
	decreasingKeys := make([]VoxelCoords, 0, len(s))
	for _, vox := range s {
		decreasingKeys = append(decreasingKeys, vox.Key)
	}
	return decreasingKeys
}

// SegmentPlanesRegionGrowing segments planes in the points in the VoxelGrid
// This segmentation only takes into account the coordinates of the points
func (vg *VoxelGrid) SegmentPlanesRegionGrowing(wTh, thetaTh, phiTh, dTh float64) {

	// Sort voxels by decreasing order of relevance weights
	decreasingKeys := vg.GetKeysByDecreasingOrderWeights()
	// Planar voxels labeling by region growing
	vg.LabelVoxels(decreasingKeys, wTh, thetaTh, phiTh)
	// For remaining voxels, labels points that are likely to belong to a plane
	unlabeledVoxels := vg.GetUnlabeledVoxels()
	vg.LabelNonPlanarVoxels(unlabeledVoxels, dTh)
}
