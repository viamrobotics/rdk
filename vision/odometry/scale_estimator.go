// Package odometry implements functions for visual odometry
package odometry

import (
	"errors"
	"math"

	"github.com/golang/geo/r2"
	"github.com/golang/geo/r3"
	"gonum.org/v1/gonum/mat"

	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/vision/delaunay"
)

// ScaleEstimatorConfig contains the parameters that are necessary for scale estimation.
type ScaleEstimatorConfig struct {
	ThresholdNormalAngle float64 `json:"th_normal_angle_rads"`
	ThresholdPlaneInlier float64 `json:"th_plane_inlier"`
}

// estimatePitchFromCameraPose gets a rough estimation of the camera pitch (angle of camera axis with ground plane
// in radians).
func estimatePitchFromCameraPose(pose *transform.CamPose) float64 {
	pitch := math.Asin(pose.Translation.At(1, 0))
	return pitch
}

// estimatePlaneFrom3Points estimate a plane equation from 3 points.
func estimatePlaneFrom3Points(p0, p1, p2 r3.Vector) (r3.Vector, float64) {
	o1 := p1.Sub(p0)
	o2 := p2.Sub(p0)
	normal := o1.Cross(o2)
	offset := -normal.Dot(p0)
	return normal, offset
}

// distToPlane returns the distance of a point to a plane.
func distToPlane(pt, normal r3.Vector, offset float64) float64 {
	dot := pt.Dot(normal)
	num := math.Abs(dot + offset)
	denom := normal.Norm()
	return num / denom
}

// getPlaneInliers returns the indices of 3D points in pts3d that are distant from at most threshold to plane.
func getPlaneInliers(pts3d []r3.Vector, normal r3.Vector, offset, threshold float64) []int {
	inliers := make([]int, 0, len(pts3d))
	for i, pt := range pts3d {
		dist := distToPlane(pt, normal, offset)
		if dist < threshold {
			inliers = append(inliers, i)
		}
	}
	return inliers
}

func getCameraHeightFromGroundPoint(pt r3.Vector, pitch float64) float64 {
	return pt.Y*math.Cos(pitch) - pt.Z*math.Sin(pitch)
}

// getAverageHeightGroundPoints returns the average height of 3d ground points wrt to the gr.
// pitch angle should be expressed in radians.
func getAverageHeightGroundPoints(groundPoints []r3.Vector, pitch float64) float64 {
	if len(groundPoints) < 1 {
		panic("no ground points to get height from")
	}
	height := 0.
	for _, pt := range groundPoints {
		height += getCameraHeightFromGroundPoint(pt, pitch)
		// height += pt.Y
	}
	return height / float64(len(groundPoints))
}

// remap3dFeatures remaps the y and z coordinates so that the y coordinate is the up-down coordinate and the
// z coordinate is the in-out coordinate, given a 3D feature vector.
func remap3dFeatures(f3d []r3.Vector, pitch float64) []r3.Vector {
	remappedF3d := make([]r3.Vector, len(f3d))
	for i, pt := range f3d {
		y := pt.Y*math.Cos(pitch) - pt.Z*math.Sin(pitch)
		z := pt.Y*math.Sin(pitch) + pt.Z*math.Cos(pitch)
		remappedF3d[i] = r3.Vector{pt.X, y, z}
	}
	return remappedF3d
}

// getSelected3DFeatures returns the 3D features whose ids are selected.
func getSelected3DFeatures(f3d []r3.Vector, ids []int) []r3.Vector {
	f3dSelected := make([]r3.Vector, 0, len(f3d))
	for _, id := range ids {
		f3dSelected = append(f3dSelected, f3d[id])
	}
	return f3dSelected
}

// GetTriangulated3DPointsFrom2DKeypoints gets the triangulated 3D point cloud from the matched 2D keypoints, the
// second camera pose and the intrinsic camera matrix.
func GetTriangulated3DPointsFrom2DKeypoints(pts1, pts2 []r2.Point, pose *transform.CamPose,
	intrinsics *transform.PinholeCameraIntrinsics,
) ([]r3.Vector, error) {
	intrinsicK := intrinsics.GetCameraMatrix()
	// homogenize 2d keypoints in image coordinates
	pts1H := transform.Convert2DPointsToHomogeneousPoints(pts1)
	pts2H := transform.Convert2DPointsToHomogeneousPoints(pts2)
	// Create projection matrix: intrinsicK@Pose
	projectionMatrix := mat.NewDense(3, 4, nil)
	projectionMatrix.Mul(intrinsicK, pose.PoseMat)
	// get triangulated 3d points in camera1 reference through projection matrix
	pts3d, err := transform.GetLinearTriangulatedPoints(projectionMatrix, pts1H, pts2H)
	if err != nil {
		return nil, err
	}
	return pts3d, nil
}

// getGroundInlierPoints takes a list of 3D points, a list of 3D triangles, and two thresholds, and returns the indices
// of the points that are inliers of the ground plane.
func getGroundInlierPoints(p3d []r3.Vector, triangles3D [][]r3.Vector, thresholdNormalAngle,
	thresholdPlaneInlier float64,
) ([]int, error) {
	inliersGround := make([]int, 0, len(p3d))
	maxInliers := 0
	groundFound := false
	for _, triangle := range triangles3D {
		normal, offset := estimatePlaneFrom3Points(triangle[0], triangle[1], triangle[2])
		angularDiff := math.Abs(normal.Dot(r3.Vector{0, 1, 0})) / normal.Norm()
		// if current normal vector is almost collinear with Y unit vector
		if angularDiff > thresholdNormalAngle {
			inliers := getPlaneInliers(p3d, normal, offset, thresholdPlaneInlier)
			if len(inliers) > maxInliers {
				maxInliers = len(inliers)
				inliersGround = make([]int, len(inliers))
				copy(inliersGround, inliers)
				groundFound = true
			}
		}
	}
	if groundFound {
		return inliersGround, nil
	}
	err := errors.New("ground plane not found")
	return nil, err
}

// GetPointsOnGroundPlane gets the ids of matched keypoints that belong to the ground plane.
func GetPointsOnGroundPlane(pts1, pts2 []r2.Point, pose *transform.CamPose,
	thresholdNormalAngle, thresholdPlaneInlier float64,
	intrinsics *transform.PinholeCameraIntrinsics,
) ([]int, []r3.Vector, error) {
	// get 3D points
	f3d, err := GetTriangulated3DPointsFrom2DKeypoints(pts1, pts2, pose, intrinsics)
	if err != nil {
		return nil, nil, err
	}
	// get camera pitch
	pitch := estimatePitchFromCameraPose(pose)
	// remap 3d features
	p3d := remap3dFeatures(f3d, pitch)
	// get 2d Delaunay triangulation; 2D Delaunay triangulation can be obtained from either pts1 or pts2, as there is a
	// bijection between the 2 sets
	pts2dDelaunay := make([]delaunay.Point, len(pts1))
	for i, pt := range pts1 {
		pts2dDelaunay[i] = delaunay.Point{pt.X, pt.Y}
	}
	tri, err := delaunay.Triangulate(pts2dDelaunay)
	if err != nil {
		return nil, nil, err
	}
	triangleMap := tri.GetTrianglesPointsMap()
	// get 3D triangles
	triangles3D := make([][]r3.Vector, len(triangleMap))
	for k, triangle := range triangleMap {
		p0 := p3d[triangle[0]]
		p1 := p3d[triangle[1]]
		p2 := p3d[triangle[2]]
		triangles3D[k] = []r3.Vector{p0, p1, p2}
	}
	// get plane equation for every 3D triangle and get the one which normal is quasi collinear with (0, -1, 0) and
	// with most inliers
	inliersGround, err := getGroundInlierPoints(p3d, triangles3D, thresholdNormalAngle, thresholdPlaneInlier)
	if err != nil {
		return nil, nil, err
	}
	// if found ground plane, get ground plane 3d points in original reference
	pointsGround := getSelected3DFeatures(p3d, inliersGround)
	return inliersGround, pointsGround, nil
}

// EstimateCameraHeight estimates the camera height wrt to ground plane.
func EstimateCameraHeight(pts1, pts2 []r2.Point, pose *transform.CamPose,
	cfg *ScaleEstimatorConfig, intrinsics *transform.PinholeCameraIntrinsics,
) (float64, error) {
	_, pointsGround, err := GetPointsOnGroundPlane(pts1, pts2, pose, cfg.ThresholdNormalAngle, cfg.ThresholdPlaneInlier, intrinsics)
	if err != nil {
		return 0, err
	}
	// get average height of camera from the points in estimated ground plane
	pitch := estimatePitchFromCameraPose(pose)
	height := getAverageHeightGroundPoints(pointsGround, pitch)
	return height, nil
}
