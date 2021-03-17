package point_cloud_segmentation

import (
	"github.com/golang/geo/r3"
	pc "go.viam.com/robotcore/pointcloud"
	"go.viam.com/robotcore/rimage"
	"gonum.org/v1/gonum/mat"
	"image"
	"image/color"
	"math"
	"math/rand"
	"time"
)

type PlaneSegmenter interface {
	SegmentPlane(int, float64, float64) (*pc.PointCloud, []float64)
}

type Points3D struct {
	points []r3.Vector
}

func New3DPoints() *Points3D {
	pts := make([]r3.Vector, 0)
	return &Points3D{pts}
}

// Convert float 3d points in meters to pointcloud
func (pts *Points3D) convert3DPointsToPointCloud(pixel2Meter float64) *pc.PointCloud {
	pointCloud := pc.New()
	for _, pt := range pts.points {
		x, y, z := MeterToMillimeterPoint(pt.X, pt.Y, pt.Z, pixel2Meter)
		ptPc := pc.NewBasicPoint(x, y, z)
		pointCloud.Set(ptPc)
	}
	return pointCloud
}

func (pts *Points3D) convert3DPointsToPointCloudWithValue(
	pixel2Meter float64,
	selectedPoints map[pc.Vec3]int,
) *pc.PointCloud {
	pointCloud := pc.New()
	for _, pt := range pts.points {
		x, y, z := MeterToMillimeterPoint(pt.X, pt.Y, pt.Z, pixel2Meter)
		val := 0
		if _, ok := selectedPoints[pc.Vec3{x, y, z}]; ok {
			val = 1
		}
		ptPc := pc.NewValuePoint(x, y, z, val)
		pointCloud.Set(ptPc)
	}
	return pointCloud
}

func (pts *Points3D) SegmentPlane(nIterations int, threshold, pixel2meter float64) (*pc.PointCloud, []float64) {
	points3d := pts.points
	nPoints := len(points3d)
	bestEquation := make([]float64, 4)
	currentEquation := make([]float64, 4)
	var bestInliers []r3.Vector

	var currentInliers []r3.Vector
	var p1, p2, p3, v1, v2, cross, vec r3.Vector
	var n1, n2, n3 int
	var d, dist float64

	for i := 0; i < nIterations; i++ {
		// sample 3 points from the slice of 3D points
		n1, n2, n3 = SampleRandomIntRange(1, nPoints-1), SampleRandomIntRange(1, nPoints-1), SampleRandomIntRange(1, nPoints-1)
		p1, p2, p3 = points3d[n1], points3d[n2], points3d[n3]

		// get 2 vectors that are going to define the plane
		v1 = p2.Sub(p1)
		v2 = p3.Sub(p1)
		// cross product to get the normal unit vector to the plane (v1, v2)
		cross = v1.Cross(v2)
		vec = cross.Normalize()
		// find current plane equation denoted as:
		// cross[0]*x + cross[1]*y + cross[2]*z + d = 0
		// to find d, we just need to pick a point and deduce d from the plane equation (vec orth to p1, p2, p3)
		d = -vec.Dot(p2)
		// current plane equation
		currentEquation[0], currentEquation[1], currentEquation[2], currentEquation[3] = vec.X, vec.Y, vec.Z, d

		// compute distance to plane of each point in the cloud
		currentInliers = nil
		// store all the points that are below a certain distance to the plane
		for _, pt := range points3d {
			dist = (currentEquation[0]*pt.X + currentEquation[1]*pt.Y + currentEquation[2]*pt.Z + currentEquation[3]) / vec.Norm()
			if math.Abs(dist) < threshold {
				currentInliers = append(currentInliers, pt)
			}
		}
		// if the current plane contains more pixels than the previously stored one, save this one as the biggest plane
		if len(currentInliers) > len(bestInliers) {
			bestEquation = currentEquation
			bestInliers = nil
			bestInliers = currentInliers
		}
	}
	// Output as slice of r3.Vector
	bestInliersPointCloud := make(map[pc.Vec3]int)
	for _, pt := range bestInliers {
		x, y, z := MeterToMillimeterPoint(pt.X, pt.Y, pt.Z, pixel2meter)
		bestInliersPointCloud[pc.Vec3{x, y, z}] = 1
	}
	return pts.convert3DPointsToPointCloudWithValue(pixel2meter, bestInliersPointCloud), bestEquation
}

// Function that creates a point cloud from a depth image with the intrinsics from the depth sensor camera
// For RGB sensor (depth image aligned with RGB 1280 x 720 resolution)
// cx, cy (Principal Point)         : 648.934, 367.736
// fx, fy (Focal Length)            : 900.538, 900.818
//Distortion Coefficients : [0.158701,-0.485405,-0.00143327,-0.000705919,0.435342]
// depthMin and DepthMax are supposed to contain the range of depth values for which the confidence is high
func (pts *Points3D)CreatePoints3DFromDepthMap(depthImage *rimage.DepthMap, pixel2meter, cx, cy, fx, fy float64, depthMin, depthMax rimage.Depth) {
	// TODO(louise): add distortion model for better accuracy
	// go through depth map pixels and get 3D points

	for y := 0; y < depthImage.Height(); y++ {
		for x := 0; x < depthImage.Width(); x++ {
			// get depth value
			d := depthImage.Get(image.Point{x, y})
			if d > depthMin && d < depthMax { //if depth is valid
				// get z distance to meter for unit uniformity
				z := float64(d) * pixel2meter
				// get x and y of 3D point
				x_, y_, _ := PixelToPoint(x, y, z, cx, cy, fx, fy)
				// Get point in PointCloud format
				pts.points = append(pts.points, r3.Vector{x_, y_, z})
			}
		}
	}
}
// Convert point from meters (float64) to mm (int)
func MeterToMillimeterPoint(x, y, z float64, pixel2Meter float64) (int, int, int) {

	xMm := int(x / pixel2Meter)
	yMm := int(y / pixel2Meter)
	zMm := int(z / pixel2Meter)
	return xMm, yMm, zMm
}

// Function to transform a pixel with depth to a 3D point cloud
// the intrinsics parameters should be the ones of the sensor used to obtain the image that contains the pixel
func PixelToPoint(x, y int, z float64, cx, cy, fx, fy float64) (float64, float64, float64) {
	//TODO(louise): add unit test
	xOverZ := (cx - float64(x)) / fx
	yOverZ := (cy - float64(y)) / fy
	// get x and y
	x_ := xOverZ * z
	y_ := yOverZ * z
	return x_, y_, z
}

// Function to project a 3D point to a pixel in an image plane
// the intrinsics parameters should be the ones of the sensor we want to project to
func PointToPixel(x, y, z float64, cx, cy, fx, fy float64) (float64, float64) {
	//TODO(louise): add unit test
	if z != 0. {
		x_px := math.Round(x/z*fx + cx)
		y_px := math.Round(y/z*fy + cy)
		return x_px, y_px
	}
	// if depth is zero at this pixel, return negative coordinates so that the cropping to RGB bounds will filter it out
	return -1.0, -1.0
}

// Function to apply a rigid body transform between two cameras to a 3D point
func TransformPointToPoint(x, y, z float64, rotationMatrix mat.Dense, translationVector r3.Vector) r3.Vector {
	r, c := rotationMatrix.Dims()
	if r != 3 || c != 3 {
		panic("Rotation Matrix to transform point cloud should be a 3x3 matrix")
	}
	x_transformed := rotationMatrix.At(0, 0)*x + rotationMatrix.At(0, 1)*y + rotationMatrix.At(0, 2)*z + translationVector.X
	y_transformed := rotationMatrix.At(1, 0)*x + rotationMatrix.At(1, 1)*y + rotationMatrix.At(1, 2)*z + translationVector.Y
	z_transformed := rotationMatrix.At(2, 0)*x + rotationMatrix.At(2, 1)*y + rotationMatrix.At(2, 2)*z + translationVector.Z

	return r3.Vector{x_transformed, y_transformed, z_transformed}
}

// Function to sample a random integer within a range given by [mib, max]
func SampleRandomIntRange(min, max int) int {
	// reset seed at every sampling operation
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(max-min+1) + min
}

// Function to detect a plane in a 3D point cloud;
// Returns a slice of PointCloud.keys of the 3D points that belong to found plane
// And the parameters of the plane equation as a slice of float64 (useful for determining points above plane)
func SegmentPlaneFrom3DFloatPoints(points3d []r3.Vector, nIterations int, threshold, pixel2meter float64) ([]r3.Vector, []float64) {

	nPoints := len(points3d)
	bestEquation := make([]float64, 4)
	currentEquation := make([]float64, 4)
	var bestInliers []r3.Vector

	var currentInliers []r3.Vector
	var p1, p2, p3, v1, v2, cross, vec r3.Vector
	var n1, n2, n3 int
	var d, dist float64

	for i := 0; i < nIterations; i++ {
		// sample 3 points from the slice of 3D points
		n1, n2, n3 = SampleRandomIntRange(1, nPoints-1), SampleRandomIntRange(1, nPoints-1), SampleRandomIntRange(1, nPoints-1)
		p1, p2, p3 = points3d[n1], points3d[n2], points3d[n3]

		// get 2 vectors that are going to define the plane
		v1 = p2.Sub(p1)
		v2 = p3.Sub(p1)
		// cross product to get the normal unit vector to the plane (v1, v2)
		cross = v1.Cross(v2)
		vec = cross.Normalize()
		// find current plane equation denoted as:
		// cross[0]*x + cross[1]*y + cross[2]*z + d = 0
		// to find d, we just need to pick a point and deduce d from the plane equation (vec orth to p1, p2, p3)
		d = -vec.Dot(p2)
		// current plane equation
		currentEquation[0], currentEquation[1], currentEquation[2], currentEquation[3] = vec.X, vec.Y, vec.Z, d

		// compute distance to plane of each point in the cloud
		currentInliers = nil
		// store all the points that are below a certain distance to the plane
		for _, pt := range points3d {
			dist = (currentEquation[0]*pt.X + currentEquation[1]*pt.Y + currentEquation[2]*pt.Z + currentEquation[3]) / vec.Norm()
			if math.Abs(dist) < threshold {
				currentInliers = append(currentInliers, pt)
			}
		}
		// if the current plane contains more pixels than the previously stored one, save this one as the biggest plane
		if len(currentInliers) > len(bestInliers) {
			bestEquation = currentEquation
			bestInliers = nil
			bestInliers = currentInliers
		}
	}
	// Output as slice of r3.Vector
	var bestInliersPointCloud []r3.Vector
	for _, pt := range bestInliers {
		bestInliersPointCloud = append(bestInliersPointCloud, pt)
	}
	return bestInliersPointCloud, bestEquation
}

// Function to create indices mask in depth image coordinates
func CreateMaskPlaneDepthIndices(pts []r3.Vector, planeInliers []int) []float64 {
	nPoints := len(pts)
	// create new mask filled with ones
	mask := make([]float64, nPoints)
	for j := range mask {
		mask[j] = 1.
	}
	// fill points corresponding to plane to 0
	for _, idx := range planeInliers {
		mask[idx] = 0.
	}
	return mask
}

// Function that creates a point cloud from a depth image with the intrinsics from the depth sensor camera
// For RGB sensor (depth image aligned with RGB 1280 x 720 resolution)
// cx, cy (Principal Point)         : 648.934, 367.736
// fx, fy (Focal Length)            : 900.538, 900.818
//Distortion Coefficients : [0.158701,-0.485405,-0.00143327,-0.000705919,0.435342]
// depthMin and DepthMax are supposed to contain the range of depth values for which the confidence is high
func DepthMapToPoints3D(depthImage *rimage.DepthMap, pixel2meter, cx, cy, fx, fy float64, depthMin, depthMax rimage.Depth) []r3.Vector {
	// TODO(louise): add distortion model for better accuracy
	// output slice
	var points []r3.Vector
	// go through depth map pixels and get 3D points

	for y := 0; y < depthImage.Height(); y++ {
		for x := 0; x < depthImage.Width(); x++ {
			// get depth value
			d := depthImage.Get(image.Point{x, y})
			if d > depthMin && d < depthMax { //if depth is valid
				// get z distance to meter for unit uniformity
				z := float64(d) * pixel2meter
				// get x and y of 3D point
				x_, y_, _ := PixelToPoint(x, y, z, cx, cy, fx, fy)
				// Get point in PointCloud format
				points = append(points, r3.Vector{x_, y_, z})
			}
		}
	}
	return points
}

// Convert Depth map to point cloud (units in mm to get int coordinates) as defined in pointcloud/pointcloud.go
func DepthMapToPointCloud(depthImage *rimage.DepthMap, pixel2meter, cx, cy, fx, fy float64) *pc.PointCloud {
	// create new point cloud
	pc_ := pc.New()
	// go through depth map pixels and get 3D points
	for y := 0; y < depthImage.Height(); y++ {
		for x := 0; x < depthImage.Width(); x++ {
			// get depth value
			d := depthImage.Get(image.Point{x, y})
			// get z distance to meter for unit uniformity
			z := float64(d) * pixel2meter
			// get x and y of 3D point
			x_, y_, z := PixelToPoint(x, y, z, cx, cy, fx, fy)
			// Get point in PointCloud format
			x_int := int(math.Round(x_ / pixel2meter))
			y_int := int(math.Round(y_ / pixel2meter))
			z_int := int(math.Round(z / pixel2meter))
			pt := pc.NewBasicPoint(x_int, y_int, z_int)
			pc_.Set(pt)
		}
	}
	return pc_
}

// Function to project 3D point in a given camera image plane
func ProjectPlane3dPointsToRGBPlane(points3d []r3.Vector, h, w int, cx, cy, fx, fy float64) []r3.Vector {
	var coordinates []r3.Vector
	for _, pt := range points3d {
		j, i := PointToPixel(pt.X, pt.Y, pt.Z, cx, cy, fx, fy)
		j = math.Round(j)
		i = math.Round(i)
		// if point is inside the RGB image bounds, add it to the coordinate list
		if j >= 0 && j < float64(w) && i >= 0 && i < float64(h) {
			pt2d := r3.Vector{j, i, pt.Z * 1000.}
			coordinates = append(coordinates, pt2d)
		}
	}
	return coordinates
}

// Get plane 2D mask obtained from depth data in RGB image coordinates
func GetPlaneMaskRGBCoordinates(depthImage *rimage.DepthMap, coordinates []r3.Vector) image.Image {
	h, w := depthImage.Height(), depthImage.Width()
	upLeft := image.Point{0, 0}
	lowRight := image.Point{w, h}

	maskPlane := image.NewGray(image.Rectangle{upLeft, lowRight})

	// Colors are defined by Red, Green, Blue, Alpha uint8 values.
	black := color.Gray{0}

	// Set color for each pixel.
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			maskPlane.Set(y, x, black)
		}
	}
	white := color.Gray{255}
	for _, pt := range coordinates {
		j, i := pt.X, pt.Y
		maskPlane.Set(int(j), int(i), white)
	}
	return maskPlane
}
