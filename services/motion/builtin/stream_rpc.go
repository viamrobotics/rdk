package builtin

import (
	"context"
	"fmt"
	"time"

	arm "go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/utils"
)

type armStream struct {
	batchesCh   chan []arm.TrajectoryPoint
	responsesCh chan arm.Response
	done        chan struct{}
	err         error

	firstPointReceived bool

	startTime         time.Time
	lastPVATPointTime time.Duration
	cumTime           time.Duration

	points []arm.TrajectoryPoint // points built since the last send
}

func newArmStream(ctx context.Context, a arm.Arm) *armStream {
	s := &armStream{
		batchesCh:   make(chan []arm.TrajectoryPoint),
		responsesCh: make(chan arm.Response),
		done:        make(chan struct{}),
	}
	go func() {
		err := a.MoveThroughJointPositionsStreamed(ctx, s.batchesCh, s.responsesCh, nil)
		s.err = err
		close(s.responsesCh)
		close(s.done)
	}()
	go func() {
		// Drain acks so the impl never blocks writing them.
		for range s.responsesCh {
		}
	}()
	return s
}

func (s *armStream) add(p pvat) error {
	// Only the first point of the stream may have a zero delta.
	dt := p.time - s.lastPVATPointTime
	if dt < 0 {
		return fmt.Errorf("negative dt=%v (time must strictly increase)", dt)
	}
	if dt == 0 && s.firstPointReceived {
		return fmt.Errorf("zero dt=%v after first point (time must strictly increase)", dt)
	}
	s.firstPointReceived = true
	s.lastPVATPointTime = p.time
	s.cumTime += dt

	velsDeg := make([]float64, len(p.velocities))
	for j, v := range p.velocities {
		velsDeg[j] = utils.RadToDeg(v)
	}
	accsDeg := make([]float64, len(p.accelerations))
	for j, a := range p.accelerations {
		accsDeg[j] = utils.RadToDeg(a)
	}
	s.points = append(s.points, arm.TrajectoryPoint{
		Time:      s.cumTime,
		Positions: append([]referenceframe.Input(nil), p.positions...),
		Constraints: &arm.KinematicConstraints{
			Velocities:    velsDeg,
			Accelerations: accsDeg,
		},
	})
	return nil
}

func (s *armStream) started() bool {
	return !s.startTime.IsZero()
}

func (s *armStream) estimatedDurationRemainingInArm() time.Duration {
	buffered := s.cumTime
	if s.started() {
		buffered -= time.Since(s.startTime)
	}
	return buffered
}

func (s *armStream) send(ctx context.Context) error {
	if len(s.points) == 0 {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.done:
		return fmt.Errorf("trajex stream RPC ended before batch could be sent: %w", s.err)
	case s.batchesCh <- s.points:
	}
	if s.startTime.IsZero() {
		// First batch delivered: the arm starts executing now
		s.startTime = time.Now()
	}
	s.points = nil
	return nil
}

func (s *armStream) close() error {
	close(s.batchesCh)
	<-s.done
	return s.err
}
