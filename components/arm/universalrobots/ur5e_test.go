package universalrobots

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"net"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/go-gl/mathgl/mgl64"
	"github.com/golang/geo/r3"
	"go.viam.com/test"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/artifact"
	"go.viam.com/utils/testutils"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/num/quat"

	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func testUR5eForwardKinematics(t *testing.T, jointRadians []float64, correct r3.Vector) {
	t.Helper()
	m, err := referenceframe.UnmarshalModelJSON(ur5modeljson, "")
	test.That(t, err, test.ShouldBeNil)

	pos, err := motionplan.ComputePosition(m, referenceframe.JointPositionsFromRadians(jointRadians))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatialmath.PoseAlmostCoincidentEps(pos, spatialmath.NewPoseFromPoint(correct), 0.01), test.ShouldBeTrue)

	fromDH := computeUR5ePosition(t, jointRadians)
	test.That(t, spatialmath.PoseAlmostEqual(pos, fromDH), test.ShouldBeTrue)
}

func testUR5eInverseKinematics(t *testing.T, pos spatialmath.Pose) {
	t.Helper()
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	m, err := referenceframe.UnmarshalModelJSON(ur5modeljson, "")
	test.That(t, err, test.ShouldBeNil)
	steps, err := motionplan.PlanFrameMotion(ctx, logger, pos, m, referenceframe.FloatsToInputs([]float64{0, 0, 0, 0, 0, 0}), nil, nil)

	test.That(t, err, test.ShouldBeNil)
	solution := steps[len(steps)-1]

	// we test that if we go forward from these joints, we end up in the same place
	jointRadians := referenceframe.InputsToFloats(solution)
	fromDH := computeUR5ePosition(t, jointRadians)
	test.That(t, spatialmath.PoseAlmostCoincidentEps(pos, fromDH, 0.01), test.ShouldBeTrue)
}

func TestKin1(t *testing.T) {
	// data came from excel file found here
	// https://www.universal-robots.com/articles/ur/application-installation/dh-parameters-for-calculations-of-kinematics-and-dynamics/
	// https://s3-eu-west-1.amazonaws.com/ur-support-site/45257/DH-Transformation.xlsx
	// Note: we use millimeters, they use meters

	// Section 1 - first we test each joint independently

	//    Home
	testUR5eForwardKinematics(t, []float64{0, 0, 0, 0, 0, 0}, r3.Vector{X: -817.2, Y: -232.90, Z: 62.80})

	//    Joint 0
	testUR5eForwardKinematics(t, []float64{math.Pi / 2, 0, 0, 0, 0, 0}, r3.Vector{X: 232.90, Y: -817.2, Z: 62.80})
	testUR5eForwardKinematics(t, []float64{math.Pi, 0, 0, 0, 0, 0}, r3.Vector{X: 817.2, Y: 232.90, Z: 62.80})

	//    Joint 1
	testUR5eForwardKinematics(t, []float64{0, math.Pi / -2, 0, 0, 0, 0}, r3.Vector{X: -99.7, Y: -232.90, Z: 979.70})
	testUR5eForwardKinematics(t, []float64{0, math.Pi / 2, 0, 0, 0, 0}, r3.Vector{X: 99.7, Y: -232.90, Z: -654.70})
	testUR5eForwardKinematics(t, []float64{0, math.Pi, 0, 0, 0, 0}, r3.Vector{X: 817.2, Y: -232.90, Z: 262.2})

	//    Joint 2
	testUR5eForwardKinematics(t, []float64{0, 0, math.Pi / 2, 0, 0, 0}, r3.Vector{X: -325.3, Y: -232.90, Z: -229.7})
	testUR5eForwardKinematics(t, []float64{0, 0, math.Pi, 0, 0, 0}, r3.Vector{X: -32.8, Y: -232.90, Z: 262.2})

	//    Joint 3
	testUR5eForwardKinematics(t, []float64{0, 0, 0, math.Pi / 2, 0, 0}, r3.Vector{X: -717.5, Y: -232.90, Z: 162.5})
	testUR5eForwardKinematics(t, []float64{0, 0, 0, math.Pi, 0, 0}, r3.Vector{X: -817.2, Y: -232.90, Z: 262.2})

	//    Joint 4
	testUR5eForwardKinematics(t, []float64{0, 0, 0, 0, math.Pi / 2, 0}, r3.Vector{X: -916.80, Y: -133.3, Z: 62.8})
	testUR5eForwardKinematics(t, []float64{0, 0, 0, 0, math.Pi, 0}, r3.Vector{X: -817.2, Y: -33.7, Z: 62.8})

	//    Joint 5
	testUR5eForwardKinematics(t, []float64{0, 0, 0, 0, 0, math.Pi / 2}, r3.Vector{X: -817.2, Y: -232.90, Z: 62.80})
	testUR5eForwardKinematics(t, []float64{0, 0, 0, 0, 0, math.Pi}, r3.Vector{X: -817.2, Y: -232.90, Z: 62.80})

	// Section 2 - try some consistent angle
	rad := math.Pi / 4
	testUR5eForwardKinematics(t, []float64{rad, rad, rad, rad, rad, rad}, r3.Vector{X: 16.62, Y: -271.49, Z: -509.52})

	rad = math.Pi / 2
	testUR5eForwardKinematics(t, []float64{rad, rad, rad, rad, rad, rad}, r3.Vector{X: 133.3, Y: 292.5, Z: -162.9})

	rad = math.Pi
	testUR5eForwardKinematics(t, []float64{rad, rad, rad, rad, rad, rad}, r3.Vector{X: -32.8, Y: 33.7, Z: 262.2})

	// Section 3 - try some random angles
	testUR5eForwardKinematics(t,
		[]float64{math.Pi / 4, math.Pi / 2, 0, math.Pi / 4, math.Pi / 2, 0},
		r3.Vector{X: 193.91, Y: 5.39, Z: -654.63},
	)
	testUR5eForwardKinematics(t,
		[]float64{0, math.Pi / 4, math.Pi / 2, 0, math.Pi / 4, math.Pi / 2},
		r3.Vector{X: 97.11, Y: -203.73, Z: -394.65},
	)

	testUR5eInverseKinematics(t, spatialmath.NewPose(
		r3.Vector{X: -202.31, Y: -577.75, Z: 318.58},
		&spatialmath.OrientationVectorDegrees{Theta: 51.84, OX: 0.47, OY: -.42, OZ: -.78},
	))
}

type dhConstants struct {
	a, d, alpha float64
}

func (d dhConstants) matrix(theta float64) *mat.Dense {
	m := mat.NewDense(4, 4, nil)

	m.Set(0, 0, math.Cos(theta))
	m.Set(0, 1, -1*math.Sin(theta)*math.Cos(d.alpha))
	m.Set(0, 2, math.Sin(theta)*math.Sin(d.alpha))
	m.Set(0, 3, d.a*math.Cos(theta))

	m.Set(1, 0, math.Sin(theta))
	m.Set(1, 1, math.Cos(theta)*math.Cos(d.alpha))
	m.Set(1, 2, -1*math.Cos(theta)*math.Sin(d.alpha))
	m.Set(1, 3, d.a*math.Sin(theta))

	m.Set(2, 0, 0)
	m.Set(2, 1, math.Sin(d.alpha))
	m.Set(2, 2, math.Cos(d.alpha))
	m.Set(2, 3, d.d)

	m.Set(3, 3, 1)

	return m
}

var jointConstants = []dhConstants{
	{0.0000, 0.1625, math.Pi / 2},
	{-0.4250, 0.0000, 0},
	{-0.3922, 0.0000, 0},
	{0.0000, 0.1333, math.Pi / 2},
	{0.0000, 0.0997, -1 * math.Pi / 2},
	{0.0000, 0.0996, 0},
}

var orientationDH = dhConstants{0, 1, math.Pi / -2}

func computeUR5ePosition(t *testing.T, jointRadians []float64) spatialmath.Pose {
	t.Helper()
	res := jointConstants[0].matrix(jointRadians[0])
	for x, theta := range jointRadians {
		if x == 0 {
			continue
		}

		temp := mat.NewDense(4, 4, nil)
		temp.Mul(res, jointConstants[x].matrix(theta))
		res = temp
	}

	var o mat.Dense
	o.Mul(res, orientationDH.matrix(0))

	ov := spatialmath.OrientationVector{
		OX: o.At(0, 3) - res.At(0, 3),
		OY: o.At(1, 3) - res.At(1, 3),
		OZ: o.At(2, 3) - res.At(2, 3),
		// Theta: utils.RadToDeg(math.Acos(o.At(0,0))), // TODO(erh): fix this
	}
	ov.Normalize()

	resMgl := mgl64.Ident4()
	// Copy to a mgl64 4x4 to convert to quaternion
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			resMgl.Set(r, c, res.At(r, c))
		}
	}
	q := mgl64.Mat4ToQuat(resMgl)
	poseOV := spatialmath.QuatToOV(quat.Number{q.W, q.X(), q.Y(), q.Z()})

	// Confirm that our matrix -> quaternion -> OV conversion yields the same result as the OV calculated from the DH param
	test.That(t, poseOV.OX, test.ShouldAlmostEqual, ov.OX, .01)
	test.That(t, poseOV.OY, test.ShouldAlmostEqual, ov.OY, .01)
	test.That(t, poseOV.OZ, test.ShouldAlmostEqual, ov.OZ, .01)

	return spatialmath.NewPose(
		r3.Vector{X: res.At(0, 3), Y: res.At(1, 3), Z: res.At(2, 3)}.Mul(1000),
		&spatialmath.OrientationVectorDegrees{OX: poseOV.OX, OY: poseOV.OY, OZ: poseOV.OZ, Theta: utils.RadToDeg(poseOV.Theta)},
	)
}

func setupListeners(ctx context.Context, statusBlob []byte,
	remote *atomic.Bool,
) (func(), error) {
	listener29999, err := net.Listen("tcp", "localhost:29999")
	if err != nil {
		return nil, err
	}

	listener30001, err := net.Listen("tcp", "localhost:30001")
	if err != nil {
		return nil, err
	}

	listener30011, err := net.Listen("tcp", "localhost:30011")
	if err != nil {
		return nil, err
	}

	goutils.PanicCapturingGo(func() {
		for {
			if ctx.Err() != nil {
				break
			}
			conn, err := listener29999.Accept()
			if err != nil {
				break
			}
			ioReader := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
			if _, err = ioReader.WriteString("hello test dashboard\n"); err != nil {
				break
			}

			if ioReader.Flush() != nil {
				break
			}
			for {
				_, _, err := ioReader.ReadLine()
				if err != nil {
					return
				}
				if _, err = ioReader.WriteString(fmt.Sprintf("%v\n", remote.Load())); err != nil {
					break
				}
				if ioReader.Flush() != nil {
					break
				}
				timeout := time.NewTimer(100 * time.Millisecond)
				select {
				case <-ctx.Done():
					return
				case <-timeout.C:
					continue
				}
			}
		}
	})
	goutils.PanicCapturingGo(func() {
		for {
			if ctx.Err() != nil {
				break
			}
			if _, err := listener30001.Accept(); err != nil {
				break
			}
		}
	})
	goutils.PanicCapturingGo(func() {
		for {
			if ctx.Err() != nil {
				break
			}
			conn, err := listener30011.Accept()
			if err != nil {
				break
			}
			for {
				if ctx.Err() != nil {
					break
				}
				_, err = conn.Write(statusBlob)
				if err != nil {
					break
				}
				if !goutils.SelectContextOrWait(ctx, 100*time.Millisecond) {
					return
				}
			}
		}
	})

	closer := func() {
		listener30001.Close()
		listener29999.Close()
		listener30011.Close()
	}
	return closer, nil
}

func TestArmReconnection(t *testing.T) {
	var remote atomic.Bool

	remote.Store(false)

	statusBlob, err := os.ReadFile(artifact.MustPath("components/arm/universalrobots/armBlob"))
	test.That(t, err, test.ShouldBeNil)

	logger := golog.NewTestLogger(t)
	parentCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx, childCancel := context.WithCancel(parentCtx)

	closer, err := setupListeners(ctx, statusBlob, &remote)

	test.That(t, err, test.ShouldBeNil)
	cfg := resource.Config{
		Name: "testarm",
		ConvertedAttributes: &Config{
			Speed:               0.3,
			Host:                "localhost",
			ArmHostedKinematics: false,
		},
	}

	arm, err := URArmConnect(parentCtx, cfg, logger)

	test.That(t, err, test.ShouldBeNil)
	ua, ok := arm.(*URArm)
	test.That(t, ok, test.ShouldBeTrue)

	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		ua.mu.Lock()
		test.That(tb, ua.isConnected, test.ShouldBeTrue)
		test.That(tb, ua.inRemoteMode, test.ShouldBeFalse)
		ua.mu.Unlock()
	})

	remote.Store(true)

	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		ua.mu.Lock()
		test.That(tb, ua.isConnected, test.ShouldBeTrue)
		test.That(tb, ua.inRemoteMode, test.ShouldBeTrue)
		ua.mu.Unlock()
	})

	remote.Store(false)

	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		ua.mu.Lock()
		test.That(tb, ua.isConnected, test.ShouldBeTrue)
		test.That(tb, ua.inRemoteMode, test.ShouldBeFalse)
		ua.mu.Unlock()
	})

	closer()
	childCancel()

	test.That(t, goutils.SelectContextOrWait(parentCtx, time.Millisecond*500), test.ShouldBeTrue)

	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		ua.mu.Lock()
		test.That(tb, ua.isConnected, test.ShouldBeFalse)
		ua.mu.Unlock()
	})

	ctx, childCancel = context.WithCancel(parentCtx)

	closer, err = setupListeners(ctx, statusBlob, &remote)
	test.That(t, err, test.ShouldBeNil)
	remote.Store(true)

	testutils.WaitForAssertion(t, func(tb testing.TB) {
		tb.Helper()
		ua.mu.Lock()
		test.That(tb, ua.isConnected, test.ShouldBeTrue)
		test.That(tb, ua.inRemoteMode, test.ShouldBeTrue)
		ua.mu.Unlock()
	})

	closer()
	childCancel()
	_ = ua.Close(ctx)
}

func TestReconfigure(t *testing.T) {
	cfg := resource.Config{
		Name: "testarm",
		ConvertedAttributes: &Config{
			Speed:               0.3,
			Host:                "localhost",
			ArmHostedKinematics: false,
		},
	}

	conf1 := resource.Config{
		Name: "testarm",
		ConvertedAttributes: &Config{
			Speed:               0.5,
			Host:                "localhost",
			ArmHostedKinematics: false,
		},
	}

	conf2 := resource.Config{
		Name: "testarm",
		ConvertedAttributes: &Config{
			Speed:               0.5,
			Host:                "new",
			ArmHostedKinematics: false,
		},
	}

	conf, err := resource.NativeConfig[*Config](cfg)
	test.That(t, err, test.ShouldBeNil)

	ur5e := &URArm{
		speed:              conf.Speed,
		urHostedKinematics: conf.ArmHostedKinematics,
		host:               conf.Host,
	}

	// scenario where we do not reconfigure
	test.That(t, ur5e.Reconfigure(context.Background(), nil, conf1), test.ShouldBeNil)
	test.That(t, ur5e.speed, test.ShouldEqual, 0.5)

	// scenario where we have to configure
	test.That(t, ur5e.Reconfigure(context.Background(), nil, conf2), test.ShouldBeNil)
	test.That(t, ur5e.speed, test.ShouldEqual, 0.5)
	test.That(t, ur5e.host, test.ShouldEqual, "new")
}
