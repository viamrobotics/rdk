package streaming

import (
	"context"
	"fmt"
	"time"

	arm "go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/referenceframe"
)

type armStream struct {
	arm arm.Arm

	// These are used to estimate how much runway the arm has buffered on its side.
	timeInTrajectoryClockOfLastSentPVAT time.Duration
	timeFirstBatchWasSent               time.Time

	// State for the current underlying RPC stream.
	started     bool
	batchesCh   chan []arm.TrajectoryPoint
	responsesCh chan arm.Response
	done        chan struct{}
	err         error
}

func (s *armStream) startStream(ctx context.Context) {
	s.batchesCh = make(chan []arm.TrajectoryPoint)
	s.responsesCh = make(chan arm.Response)
	s.done = make(chan struct{})
	s.started = true

	go func() {
		err := s.arm.MoveThroughJointPositionsStreamed(ctx, s.batchesCh, s.responsesCh, nil)
		s.err = err
		close(s.responsesCh)
		close(s.done)
	}()
	go func() {
		// Drain acks so the impl never blocks writing them.
		for range s.responsesCh {
		}
	}()
}

func (s *armStream) send(ctx context.Context, pvats []pvat) error {
	if len(pvats) == 0 {
		return nil
	}

	batch := make([]arm.TrajectoryPoint, 0, len(pvats))
	for _, p := range pvats {
		batch = append(batch, arm.TrajectoryPoint{
			Time:      p.time,
			Positions: append([]referenceframe.Input(nil), p.positions...),
			Constraints: &arm.KinematicConstraints{
				Velocities:    append([]float64(nil), p.velocities...),
				Accelerations: append([]float64(nil), p.accelerations...),
			},
		})
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.done:
		return fmt.Errorf("arm streaming RPC ended before batch could be sent: %w", s.err)
	case s.batchesCh <- batch:
	}

	s.timeInTrajectoryClockOfLastSentPVAT = batch[len(batch)-1].Time
	if s.timeFirstBatchWasSent.IsZero() {
		// First batch was just sent; assume the arm begins executing around now.
		s.timeFirstBatchWasSent = time.Now()
	}
	return nil
}

func (s *armStream) close() error {
	if !s.started {
		return nil
	}
	close(s.batchesCh)
	<-s.done
	s.started = false
	return s.err
}

func (s *armStream) currentEstimatedRunwayInArm() time.Duration {
	// Nothing sent yet. We can't fall through to below because
	// time.Since(s.timeFirstBatchWasSent) (i.e., time.Since(0)) would be very large.
	if s.timeFirstBatchWasSent.IsZero() {
		return 0
	}

	estimatedArmPositionInTrajectoryClock := time.Since(s.timeFirstBatchWasSent)

	return s.timeInTrajectoryClockOfLastSentPVAT - estimatedArmPositionInTrajectoryClock
}
