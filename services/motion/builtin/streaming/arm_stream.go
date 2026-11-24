package streaming

import (
	"context"
	"fmt"
	"time"

	arm "go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/motion/builtin/streaming/diagnostics"
)

const stopArmTimeout = time.Minute

// ArmStream is one open MoveThroughJointPositionsStreamed call to an arm, with an estimate of
// how much trajectory the arm has yet to execute. It is not safe for concurrent use.
type ArmStream struct {
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

// NewArmStream starts a stream to the arm. The stream lasts until Close is called or ctx is
// canceled.
func NewArmStream(ctx context.Context, a arm.Arm, diagnostics *diagnostics.SingleSessionDiagnostics) *ArmStream {
	s := &ArmStream{
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

func (s *ArmStream) send(ctx context.Context, pvats []pvat) error {
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

	return s.Send(ctx, batch)
}

// Send appends points to the arm's trajectory, blocking until the arm takes them. The first
// point of a stream must be at time zero, and times must increase.
func (s *ArmStream) Send(ctx context.Context, batch []arm.TrajectoryPoint) error {
	if len(batch) == 0 {
		return nil
	}

	armHasNothingToExecute := s.Runway() <= 0
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

// Close ends the RPC stream, waits for it to finish, and returns its final
// error. It must be called exactly once.
func (s *ArmStream) Close() error {
	close(s.batchesCh)
	<-s.moveThroughJointPositionsStreamedReturned
	return s.err
}

// Runway estimates how much of the sent trajectory the arm has yet to execute. Once the arm has
// run out it is negative: how long ago that was.
func (s *ArmStream) Runway() time.Duration {
	// Nothing sent yet. We can't fall through to below because
	// time.Since(s.wallTimeAtTrajectoryClockZero) (i.e., time.Since(0)) would be very large.
	if s.wallTimeAtTrajectoryClockZero.IsZero() {
		return 0
	}

	estimatedArmPositionInTrajectoryClock := time.Since(s.wallTimeAtTrajectoryClockZero)

	return s.timeInTrajectoryClockOfLastSentPVAT - estimatedArmPositionInTrajectoryClock
}

// StopArm stops the arm within a fixed timeout, whether or not ctx has been canceled.
func StopArm(ctx context.Context, a arm.Arm) error {
	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), stopArmTimeout)
	defer cancel()
	return a.Stop(stopCtx, nil)
}
