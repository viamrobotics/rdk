// Package streaming implements the ability to stream joint positions to an arm resource.
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
	opts   *StreamOptions
	pvatCh chan<- pvat

	// State for the underlying trajex library session, set up by startSession.
	sess               *totgstream.Session
	dof                int
	lastJointPositions []referenceframe.Input
}

func (s *trajexSession) run(
	ctx context.Context,
	jpCh <-chan JointPositionsChItem,
	flushCh <-chan struct{},
	seed []referenceframe.Input,
	r *trajexSessionRunHandle,
) {
	defer func() {
		close(s.pvatCh)
		if s.sess != nil {
			s.close()
		}
		close(r.done)
	}()

	fail := func(err error) {
		if s.sess != nil {
			err = fmt.Errorf("(seed=%v): %w", s.lastJointPositions, err)
		}
		r.err = err
		r.cancel()
	}

	if err := s.startSession(seed); err != nil {
		fail(fmt.Errorf("startSession (seed=%v): %w", seed, err))
		return
	}

	var nextPVAT *pvat
	for {
		// nextPVAT is nil if the trajex session was sampled through because no new targets came in
		// time.
		// Since `select` evaluates send operands up front for every case:
		// (1) To avoid a nil deref panic from `pvatCh <- *nextPVAT`, leave toSend as zero.
		// (2) To avoid pushing a zero value into the real channel, leave sendCh as nil.
		var toSend pvat
		var sendCh chan<- pvat
		if nextPVAT != nil {
			toSend, sendCh = *nextPVAT, s.pvatCh
		}

		// Accept new joint positions only while the sampled pvat backlog has room.
		recvCh := jpCh
		if nextPVAT != nil && len(s.pvatCh) == cap(s.pvatCh) {
			recvCh = nil
		}

		select {
		// Cancel was called.
		case <-ctx.Done():
			r.err = ctx.Err()
			return

		// A new target is available.
		case target, ok := <-recvCh:
			if !ok {
				// Targets channel closed.
				if err := s.sendRemainingPVATs(ctx, nextPVAT); err != nil {
					fail(err)
				}
				return
			}

			if err := s.addJointPositionsToSession(ctx, target.Positions); err != nil {
				fail(fmt.Errorf("addJointPositionsToSession: %w", err))
				return
			}

		// A flush was requested: stop accepting targets and drain the remaining trajectory.
		case <-flushCh:
			if err := s.sendRemainingPVATs(ctx, nextPVAT); err != nil {
				fail(err)
			}
			return

		// There's space in sendCh and we have a pvat to send.
		case sendCh <- toSend:
			nextPVAT = nil
		}

		// Keep exactly one sampled PVAT queued up to offer via sendCh next iteration. Only refill
		// it once the previous one has actually been sent.
		if nextPVAT == nil {
			var err error
			if nextPVAT, err = s.sampleNextPVATFromSession(ctx); err != nil {
				fail(fmt.Errorf("sampleNextPVATFromSession: %w", err))
				return
			}
		}
	}
}

func (s *trajexSession) sendRemainingPVATs(ctx context.Context, nextPVAT *pvat) error {
	if nextPVAT != nil {
		if err := s.sendPVAT(ctx, *nextPVAT); err != nil {
			return err
		}
	}
	remaining, err := s.sampleRemainingPVATsFromSession(ctx)
	if err != nil {
		return fmt.Errorf("sampleRemainingPVATsFromSession: %w", err)
	}
	for _, pv := range remaining {
		if err := s.sendPVAT(ctx, pv); err != nil {
			return err
		}
	}
	return nil
}

func (s *trajexSession) sendPVAT(ctx context.Context, pv pvat) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case s.pvatCh <- pv:
		return nil
	}
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

func (s *trajexSession) sampleNextPVATFromSession(ctx context.Context) (*pvat, error) {
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

func (s *trajexSession) sampleRemainingPVATsFromSession(ctx context.Context) ([]pvat, error) {
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
