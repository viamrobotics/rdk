package rimage

import (
	"image"
	"math"

	"github.com/aybabtme/uniplot/histogram"
	"github.com/muesli/clusters"
	"github.com/muesli/kmeans"

	"go.viam.com/rdk/utils"
)

// FillDepthMap finds regions of connected missing data, and for those below a certain size, fills them in with
// an average of the surrounding pixels by using 16-point ray-marching, taking care of regions that are on the
// boundaries between objects. Assumes rgb image and depth map are aligned.
func FillDepthMap(dm *DepthMap, img *Image) (*DepthMap, error) {
	iwd := &imageWithDepth{img, dm, true}
	validData := MissingDepthData(iwd.Depth)
	missingData := invertGrayImage(validData)
	holeMap := segmentBinaryImage(missingData)
	for _, seg := range holeMap {
		borderPoints := getPointsOnHoleBorder(seg, iwd.Depth)
		avgDepth := averageDepthInSegment(borderPoints, iwd.Depth)
		threshold := thresholdFromDepth(avgDepth, iwd.Width()*iwd.Height())
		if len(seg) < threshold {
			if isMultiModal(borderPoints, iwd.Depth, 3) { // hole most likely on an edge
				for point := range seg {
					rayPoints := pointsFromRayMarching(point.X, point.Y, 8, sixteenPoints, iwd)
					clusterDepths, clusterColors, err := clusterEdgePoints(rayPoints, iwd)
					if err != nil {
						return nil, err
					}
					val := matchDepthToClosestColor(iwd.Color.Get(point), clusterColors[0], clusterColors[1], clusterDepths[0], clusterDepths[1])
					iwd.Depth.Set(point.X, point.Y, val)
				}
			} else {
				for point := range seg {
					val := depthRayMarching(point.X, point.Y, 8, sixteenPoints, iwd)
					iwd.Depth.Set(point.X, point.Y, val)
				}
			}
		}
	}
	return iwd.Depth, nil
}

// directions for ray-marching.
var (
	sixteenPoints = []image.Point{
		{0, 2},
		{0, -2},
		{-2, 0},
		{2, 0},
		{-2, 2},
		{2, 2},
		{-2, -2},
		{2, -2},
		{-2, 1},
		{-1, 2},
		{1, 2},
		{2, 1},
		{-2, -1},
		{-1, -2},
		{1, -2},
		{2, -1},
	}
)

// function returns a map of the filled-in points on the border of a contiguous segment of holes in a depth map.
func getPointsOnHoleBorder(segment map[image.Point]bool, dm *DepthMap) map[image.Point]bool {
	directions := []image.Point{
		{0, 1},  // up
		{0, -1}, // down
		{-1, 0}, // left
		{1, 0},  // right
	}
	borderPoints := make(map[image.Point]bool)
	for hole := range segment {
		for _, dir := range directions {
			point := image.Point{hole.X + dir.X, hole.Y + dir.Y}
			if !dm.Contains(point.X, point.Y) {
				continue
			}
			if dm.GetDepth(point.X, point.Y) != 0 {
				borderPoints[point] = true
			}
		}
	}
	return borderPoints
}

// Quick way of calculating the number of modes/peaks in a collection of points, to distinguish if the collection
// of points is from one object, or a mixture of foreground and background objects. Bin widths are 100 mm.
// threshold sets how many zero bins between filled bins do there need to be to count as separate peaks.
// Could use kernel smoothing and the calculation of first derivatives to definitively find all the peaks in a collection of points.
func isMultiModal(segment map[image.Point]bool, dm *DepthMap, threshold int) bool {
	depths := pointsMap2Slice(segment, dm)
	if len(depths) == 0 {
		return false
	}
	min, max := minmax(depths)
	nbins := utils.MaxInt(1, int((max-min)/100.)) // bin widths 100mm
	hist := histogram.Hist(nbins, depths)
	peaks := 0
	zeros := threshold
	for _, bkt := range hist.Buckets {
		if bkt.Count != 0 {
			if zeros >= threshold {
				peaks++
			}
			zeros = 0
		} else {
			zeros++
		}
	}
	return peaks > 1
}

// get the minimum and the maximum from a slice of float64s.
func minmax(slice []float64) (float64, float64) {
	max := slice[0]
	min := slice[0]
	for _, value := range slice {
		if max < value {
			max = value
		}
		if min > value {
			min = value
		}
	}
	return min, max
}

// colorDepthPoints are used with kmeans clustering functions. Points are clustered according to
// their depth value. Other properties are their 2D coordinates and color. To be used with the kmeans module,
// we need to define a Coordinates and Distance method on colorDepthPoint.
type colorDepthPoint struct {
	p image.Point
	c Color
	d Depth
}

func (sp colorDepthPoint) Coordinates() clusters.Coordinates {
	coord := []float64{float64(sp.d)}
	return clusters.Coordinates(coord)
}

func (sp colorDepthPoint) Distance(p clusters.Coordinates) float64 {
	return math.Abs(float64(sp.d) - p[0])
}

// if the segment is multimodal in depth, cluster the colors and depths into 2 groups using kmeans clustering,
// to distinguish between the points associated with the foreground and background object.
func clusterEdgePoints(borderPoints map[image.Point]bool, iwd *imageWithDepth) ([]float64, []Color, error) {
	var d clusters.Observations
	for pt := range borderPoints {
		sp := colorDepthPoint{pt, iwd.Color.Get(pt), iwd.Depth.Get(pt)}
		d = append(d, sp)
	}

	km := kmeans.New()
	clusters, err := km.Partition(d, 2) // cluster into 2 partitions
	if err != nil {
		return nil, nil, err
	}
	clusterDepths := make([]float64, 0, 2)
	clusterColors := make([]Color, 0, 2)
	for _, c := range clusters {
		clusterDepths = append(clusterDepths, c.Center[0])
		colorSlice := make([]Color, 0)
		for _, point := range c.Observations {
			colorSlice = append(colorSlice, point.(colorDepthPoint).c)
		}
		clusterColors = append(clusterColors, AverageColor(colorSlice))
	}
	return clusterDepths, clusterColors, nil
}

// match the point's color to the closest cluster, and return the depth associated with that cluster.
func matchDepthToClosestColor(inColor, color1, color2 Color, depth1, depth2 float64) Depth {
	if inColor.Distance(color1) <= inColor.Distance(color2) {
		return Depth(depth1)
	}
	return Depth(depth2)
}

// get a slice of float64 from a map of points, skipping zero points.
func pointsMap2Slice(points map[image.Point]bool, dm *DepthMap) []float64 {
	slice := make([]float64, 0, len(points))
	for point := range points {
		if !dm.Contains(point.X, point.Y) {
			continue
		}
		if dm.Get(point) != 0 {
			slice = append(slice, float64(dm.GetDepth(point.X, point.Y)))
		}
	}
	return slice
}

// depthRayMarching uses multi-point ray-marching to fill in missing data. It marches out in N directions from the missing pixel until
// it encounters a pixel with data, and then averages the values of the non-zero pixels it finds to fill the missing value.
// Uses color info to help. If the color changes "too much" between pixels (exponential weighing), the depth will contribute
// less to the average.
func depthRayMarching(x, y, iterations int, directions []image.Point, iwd *imageWithDepth) Depth {
	rayPoints := pointsFromRayMarching(x, y, iterations, directions, iwd)
	imputedDepth := imputeMissingDepth(x, y, rayPoints, iwd)
	return imputedDepth
}

func imputeMissingDepth(x, y int, points map[image.Point]bool, iwd *imageWithDepth) Depth {
	colorGaus := gaussianFunction1D(0.1)
	spatialGaus := gaussianFunction2D(2.0)
	depthAvg := 0.0
	weightTot := 0.0
	centerColor := iwd.Color.GetXY(x, y)
	for pt := range points {
		depth := float64(iwd.Depth.Get(pt))
		col := iwd.Color.Get(pt)
		colorDistance := centerColor.Distance(col) // 0.0 is same color, >=1.0 is extremely different
		weight := colorGaus(colorDistance) * spatialGaus(float64(pt.X-x), float64(pt.Y-y))
		depthAvg = (depthAvg*weightTot + depth*weight) / (weightTot + weight)
		weightTot += weight
	}
	depthAvg = math.Max(depthAvg, 0.0) // depth cannot be zero
	return Depth(depthAvg)
}

// collects points used for imputation of a missing pixel by collecting  the surrounding filled-in points
// 'iterations' times in the N directions given.
func pointsFromRayMarching(x, y, iterations int, directions []image.Point, iwd *imageWithDepth) map[image.Point]bool {
	rayMarchingPoints := make(map[image.Point]bool)
	for _, dir := range directions {
		i, j := x, y
		for iter := 0; iter < iterations; iter++ { // continue in the same direction iter times
			depthIter := 0.0
			for depthIter == 0.0 { // increment in the given direction until you reach a filled pixel
				i += dir.X
				j += dir.Y
				if !iwd.Depth.Contains(i, j) { // skip if out of picture bounds
					break
				}
				depthIter = float64(iwd.Depth.GetDepth(i, j))
			}
			if depthIter != 0.0 {
				rayMarchingPoints[image.Point{i, j}] = true
			}
		}
	}
	return rayMarchingPoints
}
