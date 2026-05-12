package referenceframe

import (
	"fmt"
	"math"

	"github.com/golang/geo/r3"

	"go.viam.com/rdk/spatialmath"
)

// Tolerance constants used across phases.
const (
	axisParallelEpsilon = 1e-9
	dhCompatEpsilon     = 1e-6
)

// URDFToDHParams converts a revolute-only URDF serial chain into a list of
// DHParamConfig rows.
//
// Units. The URDF spec defines lengths in meters and angles in radians, and
// this function preserves those SI units in its output: A and D are in meters,
// Alpha/Theta/Min/Max in radians. Continuous joints become Min = -inf, Max = +inf.
//
// NOTE: this differs from DHParamConfig's JSON-config convention (A/D in mm,
// angles in degrees) used by ToDHFrames and existing files like ur5eDH.json.
// Convert the output before feeding it to ToDHFrames or serializing it to a
// model JSON config. We chose SI here so the function stays unit-pure.
//
// The first row's Parent is the URDF root link; subsequent rows chain via the
// previous row's ID.
//
// Errors if the URDF is not a serial revolute chain or if its end-effector
// frame is not DH-compatible (X-axis not perpendicular to the last joint
// axis, or origin out of the DH plane).
func URDFToDHParams(urdf *ModelConfigURDF) ([]DHParamConfig, error) {
	chain, err := walkURDFChain(urdf)
	if err != nil {
		return nil, err
	}

	axes, origins, endPose, err := jointAxesAtRest(chain)
	if err != nil {
		return nil, err
	}
	n := len(axes)

	zs, xs, pts, err := buildDHFrames(axes, origins, endPose)
	if err != nil {
		return nil, err
	}

	// Validate the end-effector's compatibility with DH row N.
	if err := validateEndEffectorDH(zs[n-1], xs[n], pts[n-1], pts[n]); err != nil {
		return nil, err
	}

	// Collect the revolute joints in chain order so we can read names and limits.
	// This filter must stay in sync with jointAxesAtRest's revolute predicate so
	// that len(revoluteJoints) == n and the indices line up with axes/origins.
	revoluteJoints := make([]*jointXML, 0, n)
	for _, j := range chain {
		if j.Type == RevoluteJoint || j.Type == ContinuousJoint {
			revoluteJoints = append(revoluteJoints, j)
		}
	}

	// First row's parent is the URDF root link; chain[0].Parent.Link is the
	// root by construction (walkURDFChain starts at the unique root).
	rootLink := chain[0].Parent.Link

	result := make([]DHParamConfig, n)
	for i := 0; i < n; i++ {
		d, theta, a, alpha := extractDHRow(zs[i], xs[i], pts[i], zs[i+1], xs[i+1], pts[i+1])

		minRad, maxRad := jointLimitsRadians(revoluteJoints[i])

		parent := rootLink
		if i > 0 {
			parent = result[i-1].ID
		}

		result[i] = DHParamConfig{
			ID:     revoluteJoints[i].Name,
			Parent: parent,
			A:      a,
			D:      d,
			Alpha:  alpha,
			Theta:  theta,
			Min:    minRad,
			Max:    maxRad,
		}
	}
	return result, nil
}

// jointLimitsRadians returns the joint's lower/upper bound in radians, as the
// URDF stores them. Continuous joints have no bounds and become +-inf.
func jointLimitsRadians(j *jointXML) (minRad, maxRad float64) {
	if j.Type == ContinuousJoint || j.Limit == nil {
		return math.Inf(-1), math.Inf(1)
	}
	return j.Limit.Lower, j.Limit.Upper
}

// walkURDFChain returns the joints in chain order from the root link to the
// single leaf. Errors if the URDF is not a serial chain (any link has more
// than one outgoing joint, or there is not exactly one root).
//
// A root link is any link that is never the child of a joint. For URDFs that
// declare an explicit "world" link, this will be it. There must be exactly
// one such link.
func walkURDFChain(urdf *ModelConfigURDF) ([]*jointXML, error) {
	// Index joints by parent-link name.
	jointsByParent := make(map[string][]*jointXML)
	for i := range urdf.Joints {
		j := &urdf.Joints[i]
		jointsByParent[j.Parent.Link] = append(jointsByParent[j.Parent.Link], j)
	}

	// A link is a root iff it is never referenced as a child.
	isChild := make(map[string]bool)
	for i := range urdf.Joints {
		isChild[urdf.Joints[i].Child.Link] = true
	}
	var roots []string
	for i := range urdf.Links {
		name := urdf.Links[i].Name
		if !isChild[name] {
			roots = append(roots, name)
		}
	}
	if len(roots) != 1 {
		return nil, fmt.Errorf("URDFToDHParams: expected exactly one root link, found %d: %v", len(roots), roots)
	}

	// Walk from the root, following the single child at each step.
	ordered := make([]*jointXML, 0, len(urdf.Joints))
	current := roots[0]
	for {
		children := jointsByParent[current]
		if len(children) == 0 {
			break // reached leaf
		}
		if len(children) > 1 {
			return nil, fmt.Errorf("URDFToDHParams: branching topology at link %q (has %d outgoing joints)", current, len(children))
		}
		j := children[0]
		ordered = append(ordered, j)
		current = j.Child.Link
	}

	if len(ordered) != len(urdf.Joints) {
		return nil, fmt.Errorf("URDFToDHParams: chain walk visited %d of %d joints; URDF may be disconnected", len(ordered), len(urdf.Joints))
	}
	return ordered, nil
}

// poseInMeters parses a URDF <origin rpy="..." xyz="..."/> element into a
// spatialmath.Pose WITHOUT unit conversion — the resulting pose's translation
// is in meters. This differs from (*pose).Parse() which converts to mm.
//
// A nil pose is interpreted as identity (URDF treats missing <origin> as identity).
func poseInMeters(p *pose) spatialmath.Pose {
	if p == nil {
		return spatialmath.NewZeroPose()
	}
	xyz := spaceDelimitedStringToFloatSlice(p.XYZ)
	rpy := spaceDelimitedStringToFloatSlice(p.RPY)
	return spatialmath.NewPose(
		r3.Vector{X: xyz[0], Y: xyz[1], Z: xyz[2]},
		&spatialmath.EulerAngles{Roll: rpy[0], Pitch: rpy[1], Yaw: rpy[2]},
	)
}

// axisInMeters parses a URDF <axis xyz="..."/> element and normalizes it.
// Returns an error if the axis is zero-length (below axisParallelEpsilon).
// A nil axis defaults to the URDF-spec default (1, 0, 0).
func axisInMeters(a *axis) (r3.Vector, error) {
	if a == nil {
		return r3.Vector{X: 1, Y: 0, Z: 0}, nil
	}
	xyz := spaceDelimitedStringToFloatSlice(a.XYZ)
	v := r3.Vector{X: xyz[0], Y: xyz[1], Z: xyz[2]}
	norm := v.Norm()
	if norm < axisParallelEpsilon {
		return r3.Vector{}, fmt.Errorf("URDFToDHParams: joint axis has zero magnitude")
	}
	return v.Mul(1 / norm), nil
}

// rotateVector applies the rotation component of a pose to a vector.
// Uses Compose because spatialmath.RotationMatrix stores the transposed
// (inverse) convention, so calling .Mul(v) directly would rotate the
// wrong direction. Compose is the authoritative transform in spatialmath.
func rotateVector(p spatialmath.Pose, v r3.Vector) r3.Vector {
	rotOnly := spatialmath.NewPoseFromOrientation(p.Orientation())
	return spatialmath.Compose(rotOnly, spatialmath.NewPoseFromPoint(v)).Point()
}

// jointAxesAtRest walks the ordered joint list at zero configuration, composing
// each joint's origin transform. For every revolute (or continuous) joint, it
// records the axis direction and origin position in world coordinates. Fixed
// joints only contribute to the accumulated transform. Non-revolute non-fixed
// joints return an error.
//
// Returns:
//   - axes[i]: unit vector of the i-th revolute joint's axis in world frame
//   - origins[i]: position of the i-th revolute joint's frame origin in world
//   - endPose: pose of the final link (leaf) in world at zero config
//   - err: first error encountered
func jointAxesAtRest(
	joints []*jointXML,
) (axes, origins []r3.Vector, endPose spatialmath.Pose, err error) {
	cumulative := spatialmath.NewZeroPose()

	for _, j := range joints {
		cumulative = spatialmath.Compose(cumulative, poseInMeters(j.Origin))

		switch j.Type {
		case FixedJoint:
			// Nothing to record; the origin is already folded in.
		case RevoluteJoint, ContinuousJoint:
			localAxis, axisErr := axisInMeters(j.Axis)
			if axisErr != nil {
				return nil, nil, nil, fmt.Errorf("joint %q: %w", j.Name, axisErr)
			}
			worldAxis := rotateVector(cumulative, localAxis)
			axes = append(axes, worldAxis)
			origins = append(origins, cumulative.Point())
		default:
			return nil, nil, nil, fmt.Errorf("joint %q has unsupported type %q (only revolute, continuous, and fixed are supported)", j.Name, j.Type)
		}
	}

	if len(axes) == 0 {
		return nil, nil, nil, fmt.Errorf("no revolute joints in chain")
	}
	return axes, origins, cumulative, nil
}

// commonNormal computes the common normal between two infinite lines in 3D,
// given by (direction, point) pairs. Returns:
//   - xDir: unit vector along the common normal, pointing from line0 toward line1.
//     For parallel non-coincident lines, this is the perpendicular from line0 to line1.
//     For coincident lines (parallel and zero perpendicular), returns the zero vector.
//   - foot0: closest point on line0 to line1 (along xDir).
//   - foot1: closest point on line1 to line0 (along xDir).
//   - parallel: true if the two axis directions are (anti-)parallel within axisParallelEpsilon.
//
// Both z0 and z1 MUST already be unit vectors.
func commonNormal(
	z0, p0, z1, p1 r3.Vector,
) (xDir, foot0, foot1 r3.Vector, parallel bool) {
	cross := z0.Cross(z1)
	if cross.Norm() < axisParallelEpsilon {
		parallel = true
		// Perpendicular from line0 to line1: project (p1 - p0) onto the plane
		// perpendicular to z0.
		d := p1.Sub(p0)
		perp := d.Sub(z0.Mul(d.Dot(z0)))
		perpNorm := perp.Norm()
		if perpNorm < axisParallelEpsilon {
			// Coincident lines.
			xDir = r3.Vector{}
			foot0 = p0
			foot1 = p0
			return
		}
		xDir = perp.Mul(1 / perpNorm)
		foot0 = p0 // any point on line0 works; pick p0
		foot1 = foot0.Add(xDir.Mul(perpNorm))
		return
	}

	// Non-parallel lines. Use standard skew-line closest-point formula.
	// Find t0, t1 such that foot0 = p0 + t0*z0 and foot1 = p1 + t1*z1 are the
	// closest points between the two lines.
	//
	// Derivation: (foot1 - foot0) must be perpendicular to both z0 and z1,
	// i.e., parallel to cross. Solve the linear system:
	//   (p1 - p0 + t1*z1 - t0*z0) · z0 = 0
	//   (p1 - p0 + t1*z1 - t0*z0) · z1 = 0
	d := p1.Sub(p0)
	a := z0.Dot(z0) // = 1 since unit
	b := z0.Dot(z1)
	c := z1.Dot(z1) // = 1
	det := a*c - b*b
	// det = 1 - b^2 = |cross|^2 > 0 since we handled parallel above.
	t0 := (d.Dot(z0)*c - d.Dot(z1)*b) / det
	t1 := (d.Dot(z0)*b - d.Dot(z1)*a) / det

	foot0 = p0.Add(z0.Mul(t0))
	foot1 = p1.Add(z1.Mul(t1))

	// xDir points from foot0 to foot1.
	diff := foot1.Sub(foot0)
	diffNorm := diff.Norm()
	if diffNorm < axisParallelEpsilon {
		// Lines intersect. Use the cross-product direction as the common normal.
		xDir = cross.Mul(1 / cross.Norm())
	} else {
		xDir = diff.Mul(1 / diffNorm)
	}
	return
}

// buildDHFrames constructs DH frames 0 through N from the list of N joint axes,
// N joint origins (in world), and the end-effector pose.
//
// Returns three slices of length N+1:
//   - zs[i]: Z-axis direction of DH frame i
//   - xs[i]: X-axis direction of DH frame i (common normal from zs[i-1] to zs[i])
//   - pts[i]: origin of DH frame i
//
// Frame 0: zs[0] = axes[0], pts[0] = point on axes[0] closest to world origin,
//
//	xs[0] = world X projected perpendicular to zs[0] (fallback world Y).
//
// Frame i (1 <= i < N): standard DH placement at common normal intersection.
// Frame N: forced to match the end-effector pose exactly (zs[N] = endZ,
//
//	xs[N] = endX, pts[N] = endP). Whether this is DH-compatible is
//	validated separately in Task 7.
func buildDHFrames(
	axes, origins []r3.Vector, endPose spatialmath.Pose,
) (zs, xs, pts []r3.Vector, err error) {
	n := len(axes)

	// Extract end-effector Z and X axes from its rotation.
	endZ := rotateVector(endPose, r3.Vector{X: 0, Y: 0, Z: 1})
	endX := rotateVector(endPose, r3.Vector{X: 1, Y: 0, Z: 0})
	endP := endPose.Point()

	// Assemble full Z-axis list including frame N's Z.
	allZ := append([]r3.Vector{}, axes...)
	allZ = append(allZ, endZ)
	allP := append([]r3.Vector{}, origins...)
	allP = append(allP, endP)

	zs = make([]r3.Vector, n+1)
	xs = make([]r3.Vector, n+1)
	pts = make([]r3.Vector, n+1)

	// Frame 0: Z is joint 1's axis.
	zs[0] = axes[0]
	// Origin of frame 0: point on axes[0] closest to the world origin.
	p := origins[0]
	t := -p.Dot(axes[0])
	pts[0] = p.Add(axes[0].Mul(t))
	// X_0: world X projected perpendicular to Z_0; fallback world Y.
	xs[0] = pickBaseX(axes[0])

	// Frames 1..N: common normal with previous axis.
	for i := 1; i <= n; i++ {
		zPrev := zs[i-1]
		zCurr := allZ[i]
		pPrev := pts[i-1]
		pCurr := allP[i]

		xDir, _, foot1, _ := commonNormal(zPrev, pPrev, zCurr, pCurr)

		// Coincident axes: maintain previous X direction.
		if xDir == (r3.Vector{}) {
			xs[i] = xs[i-1]
			pts[i] = pCurr
			zs[i] = zCurr
			continue
		}

		// Continuity sign correction: keep X direction consistent through chain.
		if xDir.Dot(xs[i-1]) < 0 {
			xDir = xDir.Mul(-1)
		}
		xs[i] = xDir
		pts[i] = foot1
		zs[i] = zCurr
	}

	// Final frame N: overwrite what the loop just wrote at index n. The loop
	// produced a canonical DH placement for frame N, but frame N must coincide
	// with the URDF's end-effector frame for the final DH row to describe the
	// full kinematics. validateEndEffectorDH enforces that the override is
	// consistent with a single DH row (X⊥Z and origin-in-plane).
	pts[n] = endP
	zs[n] = endZ
	xs[n] = endX

	return zs, xs, pts, nil
}

// extractDHRow computes the four DH parameters (d, theta, a, alpha) that
// describe the transform from the frame (zPrev, xPrev, pPrev) to the frame
// (zCurr, xCurr, pCurr).
//
// Formulas (angles signed around their pivot axis via atan2):
//
//	theta = atan2((xPrev × xCurr) · zPrev, xPrev · xCurr)
//	alpha = atan2((zPrev × zCurr) · xCurr, zPrev · zCurr)
//	d     = (pCurr - pPrev) · zPrev
//	a     = (pCurr - pPrev) · xCurr
//
// Preconditions: all direction vectors are unit length. xCurr is perpendicular
// to zPrev (enforced by DH frame construction). This function does not
// validate those preconditions.
func extractDHRow(
	zPrev, xPrev, pPrev, zCurr, xCurr, pCurr r3.Vector,
) (d, theta, a, alpha float64) {
	delta := pCurr.Sub(pPrev)
	d = delta.Dot(zPrev)
	a = delta.Dot(xCurr)

	theta = math.Atan2(xPrev.Cross(xCurr).Dot(zPrev), xPrev.Dot(xCurr))
	alpha = math.Atan2(zPrev.Cross(zCurr).Dot(xCurr), zPrev.Dot(zCurr))
	return
}

// validateEndEffectorDH verifies that the URDF's end-effector frame is
// expressible as a single DH row relative to frame N-1.
//
// Two conditions must hold (within dhCompatEpsilon):
//  1. xEnd perpendicular to zPrev: otherwise X_N cannot be a valid DH X-axis
//     (the DH row's rotation part requires T[2][0] = 0).
//  2. (pEnd - originPrev) in the plane spanned by zPrev and xEnd: otherwise
//     no (d, a) pair can realize the translation.
func validateEndEffectorDH(
	zPrev, xEnd, originPrev, pEnd r3.Vector,
) error {
	perpDot := xEnd.Dot(zPrev)
	if math.Abs(perpDot) > dhCompatEpsilon {
		return fmt.Errorf(
			"URDFToDHParams: end-effector X-axis not perpendicular to last joint axis (residual dot = %g)",
			perpDot,
		)
	}

	delta := pEnd.Sub(originPrev)
	yDir := zPrev.Cross(xEnd)
	planeResidual := delta.Dot(yDir)
	if math.Abs(planeResidual) > dhCompatEpsilon {
		return fmt.Errorf(
			"URDFToDHParams: end-effector origin out of DH plane (residual along y = %g)",
			planeResidual,
		)
	}
	return nil
}

// pickBaseX returns a unit vector perpendicular to z, preferring world X and
// falling back to world Y if z is close to world X.
func pickBaseX(z r3.Vector) r3.Vector {
	worldX := r3.Vector{X: 1, Y: 0, Z: 0}
	proj := worldX.Sub(z.Mul(z.Dot(worldX)))
	if proj.Norm() < axisParallelEpsilon {
		worldY := r3.Vector{X: 0, Y: 1, Z: 0}
		proj = worldY.Sub(z.Mul(z.Dot(worldY)))
	}
	return proj.Mul(1 / proj.Norm())
}
