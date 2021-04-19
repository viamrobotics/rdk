package pointcloudsegmentation

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"math/rand"

	"github.com/golang/geo/r3"
	pc "go.viam.com/robotcore/pointcloud"
	"go.viam.com/robotcore/rimage"
	"go.viam.com/robotcore/rimage/calib"
)

type Points3D struct {
	Points []r3.Vector
}

func New3DPoints() *Points3D {
	pts := make([]r3.Vector, 0)
	return &Points3D{pts}
}

// Convert float 3d Points in meters to pointcloud
func (pts *Points3D) convert3DPointsToPointCloud(pixel2Meter float64) (*pc.PointCloud, error) {
	pointCloud := pc.New()
	for _, pt := range pts.Points {
		x, y, z := MeterToDepthUnit(pt.X, pt.Y, pt.Z, pixel2Meter)
		ptPc := pc.NewBasicPoint(x, y, z)
		err := pointCloud.Set(ptPc)
		if err != nil {
			err = fmt.Errorf("error setting point (%v, %v, %v) in point cloud - %s", x, y, z, err)
			return pointCloud, err
		}
	}
	return pointCloud, nil
}

func (pts *Points3D) convert3DPointsToPointCloudWithValue(
	pixel2Meter float64,
	selectedPoints map[pc.Vec3]int,
) (*pc.PointCloud, error) {
	pointCloud := pc.New()
	for _, pt := range pts.Points {
		x, y, z := MeterToDepthUnit(pt.X, pt.Y, pt.Z, pixel2Meter)
		val := 0
		if _, ok := selectedPoints[pc.Vec3{x, y, z}]; ok {
			val = 1
		}
		ptPc := pc.NewValuePoint(x, y, z, val)
		err := pointCloud.Set(ptPc)
		if err != nil {
			err = fmt.Errorf("error setting point (%v, %v, %v) in point cloud - %s", x, y, z, err)
			return nil, err
		}
	}
	return pointCloud, nil
}

// util to return a random point from the pointcloud
func GetRandomPoint(cloud *pc.PointCloud) pc.Point {
	var randPoint pc.Point
	cloud.Iterate(func(p pc.Point) bool {
		randPoint = p
		return false
	})
	return randPoint
}

// Function to segment the biggest plane in the 3D Points cloud
// nIterations is the number of iteration for ransac
// threshold is the float64 value for the maximum allowed distance to the found plane for a point to belong to it
// pixel2meter is the conversion factor from the depth value to its value in meters
// This function returns a pointcloud with values; the values are set to 1 if a point belongs to the plane, 0 otherwise
func SegmentPlane(cloud *pc.PointCloud, nIterations int, threshold, pixel2meter float64) (*pc.PointCloud, []float64, error) {
	bestEquation := make([]float64, 4)
	currentEquation := make([]float64, 4)
	bestInliers := pc.New()
	var err error

	for i := 0; i < nIterations; i++ {
		// sample 3 Points from the slice of 3D Points
		n1, n2, n3 := GetRandomPoint(cloud), GetRandomPoint(cloud), GetRandomPoint(cloud)
		p1, p2, p3 := r3.Vector(n1.Position()), r3.Vector(n2.Position()), r3.Vector(n3.Position())

		// get 2 vectors that are going to define the plane
		v1 := p2.Sub(p1)
		v2 := p3.Sub(p1)
		// cross product to get the normal unit vector to the plane (v1, v2)
		cross := v1.Cross(v2)
		vec := cross.Normalize()
		// find current plane equation denoted as:
		// cross[0]*x + cross[1]*y + cross[2]*z + d = 0
		// to find d, we just need to pick a point and deduce d from the plane equation (vec orth to p1, p2, p3)
		d := -vec.Dot(p2)
		// current plane equation
		currentEquation[0], currentEquation[1], currentEquation[2], currentEquation[3] = vec.X, vec.Y, vec.Z, d

		// compute distance to plane of each point in the cloud
		currentInliers := pc.New()
		// store all the Points that are below a certain distance to the plane
		cloud.Iterate(func(pt pc.Point) bool {
			dist := (currentEquation[0]*pt.Position().X + currentEquation[1]*pt.Position().Y + currentEquation[2]*pt.Position().Z + currentEquation[3]) / vec.Norm()
			if math.Abs(dist) < threshold {
				err = currentInliers.Set(pt)
				if err != nil {
					err = fmt.Errorf("error setting point (%v, %v, %v) in point cloud - %s", pt.Position().X, pt.Position().Y, pt.Position().Z, err)
					return false
				}
			}
			return true
		})
		if err != nil {
			return nil, nil, err
		}
		// if the current plane contains more pixels than the previously stored one, save this one as the biggest plane
		if currentInliers.Size() > bestInliers.Size() {
			bestEquation = currentEquation
			bestInliers = currentInliers
		}
	}
	return bestInliers, bestEquation, nil
}

// Function that creates a point cloud from a depth image with the intrinsics from the depth sensor camera
// For RGB sensor (depth image aligned with RGB 1280 x 720 resolution)
// cx, cy (Principal Point)         : 648.934, 367.736
// fx, fy (Focal Length)            : 900.538, 900.818
//Distortion Coefficients : [0.158701,-0.485405,-0.00143327,-0.000705919,0.435342]
// depthMin and DepthMax are supposed to contain the range of depth values for which the confidence is high
func CreatePoints3DFromDepthMap(depthImage *rimage.DepthMap, pixel2meter float64, params calib.PinholeCameraIntrinsics, depthMin, depthMax rimage.Depth) *Points3D {
	// TODO(louise): add distortion model for better accuracy
	// go through depth map pixels and get 3D Points
	pts := New3DPoints()

	for y := 0; y < depthImage.Height(); y++ {
		for x := 0; x < depthImage.Width(); x++ {
			// get depth value
			d := depthImage.Get(image.Point{x, y})
			if d > depthMin && d < depthMax { //if depth is valid
				// get z distance to meter for unit uniformity
				z := float64(d) * pixel2meter
				// get x and y of 3D point
				xPoint, yPoint, _ := params.PixelToPoint(float64(x), float64(y), z)
				// Get point in PointCloud format
				pts.Points = append(pts.Points, r3.Vector{xPoint, yPoint, z})
			}
		}
	}
	return pts
}

// Function to project 3D point in a given camera image plane
func (pts *Points3D) ProjectPlane3dPointsToRGBPlane(h, w int, params calib.PinholeCameraIntrinsics, pixel2meter float64) ([]r3.Vector, error) {
	var coordinates []r3.Vector
	for _, pt := range pts.Points {
		j, i := params.PointToPixel(pt.X, pt.Y, pt.Z)
		j = math.Round(j)
		i = math.Round(i)
		// if point is inside the RGB image bounds, add it to the coordinate list
		if j >= 0 && j < float64(w) && i >= 0 && i < float64(h) {
			pt2d := r3.Vector{j, i, pt.Z}
			coordinates = append(coordinates, pt2d)
		}
	}
	return coordinates, nil
}

// Same as above, but with PointCloud to PointCloud
func ProjectPointCloudToRGBPlane(pts *pc.PointCloud, h, w int, params calib.PinholeCameraIntrinsics, pixel2meter float64) (*pc.PointCloud, error) {
	coordinates := pc.New()
	var err error
	pts.Iterate(func(pt pc.Point) bool {
		j, i := params.PointToPixel(pt.Position().X, pt.Position().Y, pt.Position().Z)
		j = math.Round(j)
		i = math.Round(i)
		// if point has color is inside the RGB image bounds, add it to the new pointcloud
		if j >= 0 && j < float64(w) && i >= 0 && i < float64(h) && pt.HasColor() {
			pt2d := pc.NewColoredPoint(j, i, pt.Position().Z, pt.Color().(color.NRGBA))
			err = coordinates.Set(pt2d)
			if err != nil {
				err = fmt.Errorf("error setting point (%v, %v, %v) in point cloud - %s", j, i, pt.Position().Z, err)
				return false
			}
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	return coordinates, nil
}

//TODO(louise): Add Depth Map dilation function as in the librealsense library

// util for reprojected depth image dilation
func IsPixelInImage(i, j, h, w int) bool {
	if i >= 0 && i < h && j >= 0 && j < w {
		return true
	}
	return false
}

// Function to project 3D point in a given camera image plane
func (pts *Points3D) ApplyRigidBodyTransform(params *calib.Extrinsics) (*Points3D, error) {
	transformedPoints := New3DPoints()
	for _, pt := range pts.Points {
		x, y, z := params.TransformPointToPoint(pt.X, pt.Y, pt.Z)
		ptTransformed := r3.Vector{x, y, z}
		transformedPoints.Points = append(transformedPoints.Points, ptTransformed)
	}
	return transformedPoints, nil
}

// Same as above but with pointclouds, return new pointclouds leaving original unchanged
func ApplyRigidBodyTransform(pts *pc.PointCloud, params *calib.Extrinsics) (*pc.PointCloud, error) {
	transformedPoints := pc.New()
	var err error
	pts.Iterate(func(pt pc.Point) bool {
		x, y, z := params.TransformPointToPoint(pt.Position().X, pt.Position().Y, pt.Position().Z)
		var ptTransformed pc.Point
		if pt.HasColor() {
			ptTransformed = pc.NewColoredPoint(x, y, z, pt.Color().(color.NRGBA))
		} else if pt.HasValue() {
			ptTransformed = pc.NewValuePoint(x, y, z, pt.Value())
		} else {
			ptTransformed = pc.NewBasicPoint(x, y, z)
		}
		err = transformedPoints.Set(ptTransformed)
		if err != nil {
			err = fmt.Errorf("error setting point (%v, %v, %v) in point cloud - %s", x, y, z, err)
			return false
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	return transformedPoints, nil
}

// utils for 3D float Points transforms

// Convert point from meters (float64) to mm (int)
func MeterToDepthUnit(x, y, z float64, pixel2Meter float64) (float64, float64, float64) {
	if pixel2Meter < 0.0000001 {
		panic("pixel2Meter is too close to zero to make the conversion from meters to millimeters.")
	}
	xMm := x / pixel2Meter
	yMm := y / pixel2Meter
	zMm := z / pixel2Meter
	return xMm, yMm, zMm
}

// Function to sample a random integer within a range given by [min, max]
func SampleRandomIntRange(min, max int) int {
	return rand.Intn(max-min+1) + min
}

// Convert Depth map to point cloud (units in mm to get int coordinates) as defined in pointcloud/pointcloud.go
func DepthMapToPointCloud(depthImage *rimage.DepthMap, pixel2meter float64, params calib.PinholeCameraIntrinsics) (*pc.PointCloud, error) {
	// create new point cloud
	pcOut := pc.New()
	// go through depth map pixels and get 3D Points
	for y := 0; y < depthImage.Height(); y++ {
		for x := 0; x < depthImage.Width(); x++ {
			// get depth value
			d := depthImage.Get(image.Point{x, y})
			// get z distance to meter for unit uniformity
			z := float64(d) * pixel2meter
			// get x and y of 3D point
			xPoint, yPoint, z := params.PixelToPoint(float64(x), float64(y), z)
			// Get point in PointCloud format
			xPoint = xPoint / pixel2meter
			yPoint = yPoint / pixel2meter
			z = z / pixel2meter
			pt := pc.NewBasicPoint(xPoint, yPoint, z)
			err := pcOut.Set(pt)
			if err != nil {
				err = fmt.Errorf("error setting point (%v, %v, %v) in point cloud - %s", xPoint, yPoint, z, err)
				return nil, err
			}
		}
	}
	return pcOut, nil
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

// Same as above, but with a pointcloud plane of RGB coorindates
func GetPlaneMaskRGBPointCloud(depthImage *rimage.DepthMap, coordinates *pc.PointCloud) image.Image {
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
	coordinates.Iterate(func(pt pc.Point) bool {
		j, i := pt.Position().X, pt.Position().Y
		maskPlane.Set(int(j), int(i), white)
		return true
	})
	return maskPlane
}
