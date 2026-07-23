package builtin

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
	sess               *totgstream.Session
	cfg                *streamConfig
	dof                int
	lastJointPositions []referenceframe.Input
}

func newTrajexSession(cfg *streamConfig, startJointPositions []referenceframe.Input) (*trajexSession, error) {
	opts, err := trajex.NewTensorMap()
	if err != nil {
		return nil, err
	}
	defer opts.Close()

	dof := len(startJointPositions)
	vel := make([]float64, dof)
	accel := make([]float64, dof)
	for i := range dof {
		vel[i] = utils.DegToRad(cfg.VelLimitDegPerSec)
		accel[i] = utils.DegToRad(cfg.AccelLimitDegPerSec2)
	}
	dofShape := []uint64{uint64(dof)}
	if err := opts.InsertFloat64s(totgstream.KeyVelocityLimitsRadsPerSec, dofShape, vel); err != nil {
		return nil, err
	}
	if err := opts.InsertFloat64s(totgstream.KeyAccelerationLimitsRadsPerSec2, dofShape, accel); err != nil {
		return nil, err
	}
	if err := opts.InsertScalarFloat64(totgstream.KeyPathToleranceDeltaRads, trajexPathToleranceRads); err != nil {
		return nil, err
	}
	// Convert the send interval from milliseconds to Hz
	samplingFrequencyHz := 1000.0 / float64(cfg.SendToArmIntervalMs)
	if err := opts.InsertScalarFloat64(totgstream.KeyTrajectorySamplingFreqHz, samplingFrequencyHz); err != nil {
		return nil, err
	}

	sess, err := totgstream.New(opts)
	if err != nil {
		return nil, err
	}
	return &trajexSession{
		sess:               sess,
		cfg:                cfg,
		dof:                dof,
		lastJointPositions: startJointPositions,
	}, nil
}

func (s *trajexSession) addJointPositions(ctx context.Context, nextJointPositions []referenceframe.Input) error {
	if inputsWithin(nextJointPositions, s.lastJointPositions, waypointDedupEps) {
		return nil
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

type pvat struct {
	positions     []float64
	velocities    []float64
	accelerations []float64
	time          time.Duration
}

func (s *trajexSession) nextPVAT(ctx context.Context) (*pvat, error) {
	out, err := trajex.NewTensorMap()
	if err != nil {
		return nil, err
	}
	defer out.Close()
	if err := s.sess.SampleNext(ctx, 1, out); err != nil {
		return nil, err
	}
	pvats, err := pvatsFromOutput(out)
	if err != nil || len(pvats) == 0 {
		return nil, err
	}
	return &pvats[0], nil
}

func (s *trajexSession) remainingPVATS(ctx context.Context) ([]pvat, error) {
	const drainBatch = 64
	out, err := trajex.NewTensorMap()
	if err != nil {
		return nil, err
	}
	defer out.Close()

	var all []pvat
	for {
		if err := s.sess.SampleNext(ctx, drainBatch, out); err != nil {
			return nil, err
		}
		batch, err := pvatsFromOutput(out)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			return all, nil
		}
		all = append(all, batch...)
	}
}

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

func (s *trajexSession) close() { s.sess.Close() }

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
