//go:build cgo
package transform

import (
	"errors"

	"github.com/golang/geo/r2"
	"github.com/golang/geo/r3"
	"gonum.org/v1/gonum/mat"

	"go.viam.com/rdk/spatialmath"
)

// CamPose stores the 3x4 pose matrix as well as the 3D Rotation and Translation matrices.
type CamPose struct {
	PoseMat     *mat.Dense
	Rotation    *mat.Dense
	Translation *mat.Dense
}

// NewCamPoseFromMat creates a pointer to a Camera pose from a 4x3 pose dense matrix.
func NewCamPoseFromMat(pose *mat.Dense) *CamPose {
	U3 := pose.ColView(3)
	t := mat.NewDense(3, 1, []float64{U3.AtVec(0), U3.AtVec(1), U3.AtVec(2)})
	rot := mat.NewDense(3, 3, nil)
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			rot.Set(i, j, pose.At(i, j))
		}
	}
	return &CamPose{
		PoseMat:     pose,
		Rotation:    rot,
		Translation: t,
	}
}

// Pose creates a spatialmath.Pose from a CamPose.
func (cp *CamPose) Pose() (spatialmath.Pose, error) {
	translation := r3.Vector{cp.Translation.At(0, 0), cp.Translation.At(1, 0), cp.Translation.At(2, 0)}
	rotation, err := spatialmath.NewRotationMatrix(cp.Rotation.RawMatrix().Data)
	if err != nil {
		return nil, err
	}
	return spatialmath.NewPose(translation, rotation), err
}

// adjustPoseSign adjusts the sign of a pose.
func adjustPoseSign(pose *mat.Dense) *mat.Dense {
	// take 3x3 sub-matrix
	subPose := pose.Slice(0, 3, 0, 3)

	// if determinant is negative, scale by -1
	if m := mat.DenseCopyOf(subPose); mat.Det(m) < 0 {
		pose.Scale(-1, pose)
	}
	return pose
}

// GetPossibleCameraPoses computes all 4 possible poses from the essential matrix.
func GetPossibleCameraPoses(essMat *mat.Dense) ([]*mat.Dense, error) {
	R1, R2, t, err := DecomposeEssentialMatrix(essMat)
	if err != nil {
		return nil, err
	}
	// svd
	var svd mat.SVD
	ok := svd.Factorize(essMat, mat.SVDFull)
	if !ok {
		err = errors.New("failed to factorize A")
		return nil, err
	}
	// poses
	var tOpp mat.Dense
	tOpp.Scale(-1, t)
	poses := make([]mat.Dense, 4)
	poses[0].Augment(R1, t)
	poses[1].Augment(R1, &tOpp)
	poses[2].Augment(R2, t)
	poses[3].Augment(R2, &tOpp)
	// adjust sign of poses
	posesOut := make([]*mat.Dense, 4)
	for i := range poses {
		posesOut[i] = mat.DenseCopyOf(adjustPoseSign(&poses[i]))
	}

	return posesOut, nil
}

// getCrossProductMatFromPoint returns the cross product with point p matrix.
func getCrossProductMatFromPoint(p r3.Vector) *mat.Dense {
	cross := mat.NewDense(3, 3, nil)
	cross.Set(0, 1, -p.Z)
	cross.Set(0, 2, p.Y)
	cross.Set(1, 0, p.Z)
	cross.Set(1, 2, -p.X)
	cross.Set(2, 0, -p.Y)
	cross.Set(2, 1, p.X)
	return cross
}

// GetLinearTriangulatedPoints computes triangulated 3D points with linear method.
func GetLinearTriangulatedPoints(pose *mat.Dense, pts1, pts2 []r3.Vector) ([]r3.Vector, error) {
	// set identity pose for pts1
	P := mat.NewDense(3, 4, nil)
	P.Set(0, 0, 1)
	P.Set(1, 1, 1)
	P.Set(2, 2, 1)
	// copy pose for pts2
	Pdash := mat.DenseCopyOf(pose)
	// initialize 3D points
	nPoints := len(pts1)
	pts3d := make([]r3.Vector, nPoints)
	for i := range pts1 {
		p1 := pts1[i]
		p2 := pts2[i]
		p1Cross := getCrossProductMatFromPoint(p1)
		p2Cross := getCrossProductMatFromPoint(p2)
		p1CrossP := mat.NewDense(3, 4, nil)
		p1CrossP.Mul(p1Cross, P)
		p2CrossPdash := mat.NewDense(3, 4, nil)
		p2CrossPdash.Mul(p2Cross, Pdash)
		var A mat.Dense
		A.Stack(p1CrossP, p2CrossPdash)
		// svd
		var svd mat.SVD
		ok := svd.Factorize(&A, mat.SVDFull)
		if !ok {
			err := errors.New("failed to factorize A")
			return nil, err
		}
		// Determine the rank of the A matrix with a near zero condition threshold.
		const rcond = 1e-15
		rank := svd.Rank(rcond)
		if rank == 0 {
			err := errors.New("zero rank system")
			return nil, err
		}
		var V mat.Dense
		svd.VTo(&V)
		pt3d := V.ColView(2)
		pts3d[i] = r3.Vector{
			X: pt3d.At(0, 0) / pt3d.At(3, 0),
			Y: pt3d.At(1, 0) / pt3d.At(3, 0),
			Z: pt3d.At(2, 0) / pt3d.At(3, 0),
		}
	}

	return pts3d, nil
}

// GetNumberPositiveDepth computes the number of positive zs in triangulated points pts1 and pts2.
func GetNumberPositiveDepth(pose *mat.Dense, pts1, pts2 []r3.Vector, useNonLinear bool) (int, *mat.Dense) {
	// get vectors from pose that are necessary to check if depth is positive
	rot3 := r3.Vector{pose.At(2, 0), pose.At(2, 1), pose.At(2, 2)}
	c := r3.Vector{pose.At(0, 3), pose.At(1, 3), pose.At(2, 3)}

	// triangulated points
	pts3D, err := GetLinearTriangulatedPoints(pose, pts1, pts2)
	if err != nil {
		return 0, nil
	}
	//	Non linear triangulation can be done here to get better approximation of the 3D point
	//	points_3D = get_nonlinear_triangulated_points(points_3D, pose, point_list1, point_list2)
	//	We can then have a better approximation of the pose to get R and t
	//	better_approx_pose = get_approx_pose_by_non_linear_pnp(points_3D, pose, point_list1, point_list2)

	// get number of positive depths in 3d points wrt to camera
	nPositiveDepth := 0
	for _, pt := range pts3D {
		if rot3.Dot(pt.Sub(c)) > 0 {
			nPositiveDepth++
		}
	}
	return nPositiveDepth, pose
}

// GetCorrectCameraPose returns the best pose, which is the pose with the most positive depth values.
func GetCorrectCameraPose(poses []*mat.Dense, pts1, pts2 []r3.Vector) *mat.Dense {
	maxNumPosDepth := 0
	correctPose := poses[0]
	for _, pose := range poses {
		nPosDepth, betterPoseApprox := GetNumberPositiveDepth(pose, pts1, pts2, false)
		if nPosDepth > maxNumPosDepth {
			maxNumPosDepth = nPosDepth
			correctPose = mat.DenseCopyOf(betterPoseApprox)
		}
	}
	return correctPose
}

// EstimateNewPose estimates the pose of the camera in the second set of points wrt the pose of the camera in the first
// set of points
// pts1 and pts2 are matches in 2 images (successive in time or from 2 different cameras of the same scene
// at the same time).
func EstimateNewPose(pts1, pts2 []r2.Point, k *mat.Dense) (*CamPose, error) {
	if len(pts1) != len(pts2) {
		return nil, errors.New("the 2 sets of points don't have the same number of elements")
	}
	fundamentalMatrix, err := ComputeFundamentalMatrixAllPoints(pts1, pts2, true)
	if err != nil {
		return nil, err
	}

	essentialMatrix, err := GetEssentialMatrixFromFundamental(k, k, fundamentalMatrix)
	if err != nil {
		return nil, err
	}
	poses, err := GetPossibleCameraPoses(essentialMatrix)
	if err != nil {
		return nil, err
	}
	pts1H := Convert2DPointsToHomogeneousPoints(pts1)
	pts2H := Convert2DPointsToHomogeneousPoints(pts2)
	pose := GetCorrectCameraPose(poses, pts1H, pts2H)
	return NewCamPoseFromMat(pose), nil
}
