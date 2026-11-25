package streaming

import (
	"context"
	"fmt"
	"time"

	arm "go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/motion/builtin/streaming/diagnostics"
)

type armStream struct {
	arm arm.Arm

	// These are used to estimate how much runway the arm has buffered on its side.
	timeInTrajectoryClockOfLastSentPVAT time.Duration
	wallTimeAtTrajectoryClockZero       time.Time

	// State for the underlying RPC stream.
	batchesCh   chan []arm.TrajectoryPoint
	responsesCh chan arm.Response

	moveThroughJointPositionsStreamedReturned chan struct{}

	err error

	diagnostics *diagnostics.SingleSessionDiagnostics
}

// newArmStream constructs an armStream and starts its RPC stream to the arm.
func newArmStream(ctx context.Context, a arm.Arm, diagnostics *diagnostics.SingleSessionDiagnostics) *armStream {
	s := &armStream{
		arm:         a,
		batchesCh:   make(chan []arm.TrajectoryPoint),
		responsesCh: make(chan arm.Response),

		moveThroughJointPositionsStreamedReturned: make(chan struct{}),

		diagnostics: diagnostics,
	}

	s.diagnostics.RecordArmStreamOpenEvent()
	go func() {
		err := s.arm.MoveThroughJointPositionsStreamed(ctx, s.batchesCh, s.responsesCh, nil)
		s.diagnostics.RecordArmStreamCloseEvent()
		s.err = err
		close(s.responsesCh)
		close(s.moveThroughJointPositionsStreamedReturned)
	}()
	go func() {
		// Drain acks so the impl never blocks writing them.
		for range s.responsesCh {
		}
	}()
	return s
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

	armHasNothingToExecute := s.currentEstimatedRunwayInArm() <= 0
	previousLastSent := s.timeInTrajectoryClockOfLastSentPVAT

	sendStart := time.Now()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.moveThroughJointPositionsStreamedReturned:
		return fmt.Errorf("arm streaming RPC ended before batch could be sent: %w", s.err)
	case s.batchesCh <- batch:
	}
	s.diagnostics.RecordSendToArmLatency(sendStart, time.Since(sendStart))

	s.timeInTrajectoryClockOfLastSentPVAT = batch[len(batch)-1].Time
	if armHasNothingToExecute {
		// The arm was at the end of what it had been sent; assume it starts on this batch around now.
		s.wallTimeAtTrajectoryClockZero = time.Now().Add(-previousLastSent)
	}
	return nil
}

// close ends the RPC stream, waits for it to finish, and returns its final
// error. It must be called exactly once.
func (s *armStream) close() error {
	close(s.batchesCh)
	<-s.moveThroughJointPositionsStreamedReturned
	return s.err
}

func (s *armStream) currentEstimatedRunwayInArm() time.Duration {
	// Nothing sent yet. We can't fall through to below because
	// time.Since(s.wallTimeAtTrajectoryClockZero) (i.e., time.Since(0)) would be very large.
	if s.wallTimeAtTrajectoryClockZero.IsZero() {
		return 0
	}

	estimatedArmPositionInTrajectoryClock := time.Since(s.wallTimeAtTrajectoryClockZero)

	return s.timeInTrajectoryClockOfLastSentPVAT - estimatedArmPositionInTrajectoryClock
}
