//go:build !windows && !no_cgo && viam_rdk_cgo_have_cxx20_rt

package streaming

import (
	"context"
	"fmt"
	"math"
	"time"

	trajex "github.com/viam-modules/trajex/go"
	totgstream "github.com/viam-modules/trajex/go/totg/streaming"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/utils"
)

const (
	trajexPathToleranceRads = 0.5 * math.Pi / 180
	waypointDedupEps        = 1e-4
)

type trajexSession struct {
	opts StreamOptions

	// State for the underlying trajex library session, set up by startSession.
	sess               *totgstream.Session
	dof                int
	lastJointPositions []referenceframe.Input
}

func (s *trajexSession) startSession(startJointPositions []referenceframe.Input) error {
	trajexOpts, err := trajex.NewTensorMap()
	if err != nil {
		return err
	}
	defer trajexOpts.Close()

	dof := len(startJointPositions)
	vel := make([]float64, dof)
	accel := make([]float64, dof)
	for i := range dof {
		vel[i] = utils.DegToRad(s.opts.VelLimitDegPerSec)
		accel[i] = utils.DegToRad(s.opts.AccelLimitDegPerSec2)
	}
	dofShape := []uint64{uint64(dof)}
	if err := trajexOpts.InsertFloat64s(totgstream.KeyVelocityLimitsRadsPerSec, dofShape, vel); err != nil {
		return err
	}
	if err := trajexOpts.InsertFloat64s(totgstream.KeyAccelerationLimitsRadsPerSec2, dofShape, accel); err != nil {
		return err
	}
	if err := trajexOpts.InsertScalarFloat64(totgstream.KeyPathToleranceDeltaRads, trajexPathToleranceRads); err != nil {
		return err
	}
	// Convert the send interval from milliseconds to Hz
	samplingFrequencyHz := 1000.0 / float64(s.opts.SendToArmIntervalMs)
	if err := trajexOpts.InsertScalarFloat64(totgstream.KeyTrajectorySamplingFreqHz, samplingFrequencyHz); err != nil {
		return err
	}

	sess, err := totgstream.New(trajexOpts)
	if err != nil {
		return err
	}
	s.sess = sess
	s.dof = dof
	s.lastJointPositions = startJointPositions
	return nil
}

func (s *trajexSession) addJointPositionsToSession(ctx context.Context, nextJointPositions []referenceframe.Input) error {
	if inputsWithin(nextJointPositions, s.lastJointPositions, waypointDedupEps) {
		return nil
	}
	if len(nextJointPositions) != s.dof {
		return fmt.Errorf("target has %d joint positions, but the arm has %d joints", len(nextJointPositions), s.dof)
	}

	waypoints, err := trajex.NewTensorMap()
	if err != nil {
		return err
	}
	defer waypoints.Close()

	flat := make([]float64, 0, 2*s.dof)
	flat = append(flat, s.lastJointPositions...)
	flat = append(flat, nextJointPositions...)

	if err := waypoints.InsertFloat64s(totgstream.KeyWaypointsRads, []uint64{2, uint64(s.dof)}, flat); err != nil {
		return err
	}
	if err := s.sess.Extend(ctx, waypoints); err != nil {
		return err
	}
	s.lastJointPositions = nextJointPositions
	return nil
}

func (s *trajexSession) sampleAtLeast(ctx context.Context, horizon time.Duration) ([]pvat, error) {
	out, err := trajex.NewTensorMap()
	if err != nil {
		return nil, err
	}
	defer out.Close()
	// The trajex SampleAtLeast is guaranteed to return at least one pvat, regardless of
	// the size of 'horizon', unless there is no unsampled trajectory left (nothing
	// extended yet, or everything sampled through).
	if err := s.sess.SampleAtLeast(ctx, horizon, out); err != nil {
		return nil, err
	}
	return pvatsFromOutput(out)
}

func (s *trajexSession) close() { s.sess.Close() }

func pvatsFromOutput(out *trajex.TensorMap) ([]pvat, error) {
	view := func(key string) ([]uint64, []float64, error) {
		shape, data, ok, err := out.ViewFloat64s(key)
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			return nil, nil, fmt.Errorf("trajex output missing key %q", key)
		}
		return shape, data, nil
	}

	tShape, times, err := view(totgstream.KeySampleTimesSec)
	if err != nil {
		return nil, err
	}
	if len(tShape) != 1 {
		return nil, fmt.Errorf("trajex %q tensor has rank %d, want 1", totgstream.KeySampleTimesSec, len(tShape))
	}
	cShape, positions, err := view(totgstream.KeyConfigurationsRads)
	if err != nil {
		return nil, err
	}
	if len(cShape) != 2 {
		return nil, fmt.Errorf("trajex %q tensor has rank %d, want 2", totgstream.KeyConfigurationsRads, len(cShape))
	}
	_, velocities, err := view(totgstream.KeyVelocitiesRadsPerSec)
	if err != nil {
		return nil, err
	}
	_, accelerations, err := view(totgstream.KeyAccelerationsRadsPerSec2)
	if err != nil {
		return nil, err
	}
	n, dof := int(tShape[0]), int(cShape[1])
	pvats := make([]pvat, n)
	for i := range n {
		lo, hi := i*dof, (i+1)*dof
		pvats[i] = pvat{
			positions:     append([]float64(nil), positions[lo:hi]...),
			velocities:    append([]float64(nil), velocities[lo:hi]...),
			accelerations: append([]float64(nil), accelerations[lo:hi]...),
			time:          time.Duration(times[i] * float64(time.Second)),
		}
	}
	return pvats, nil
}

func inputsWithin(a, b []referenceframe.Input, eps float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if math.Abs(float64(a[i]-b[i])) > eps {
			return false
		}
	}
	return true
}
