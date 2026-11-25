package streaming

import (
	"context"
	"time"

	arm "go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/services/motion/builtin/streaming/diagnostics"
)

// Point is one timed joint-space point sent to an arm stream. Time is measured from the first
// point of the stream, which must be at zero; later points must strictly increase.
type Point struct {
	Positions     []float64
	Velocities    []float64
	Accelerations []float64
	Time          time.Duration
}

// ArmStreamer streams timed joint points to an arm through MoveThroughJointPositionsStreamed.
// It wraps the arm-stream layer TempStreamArmJointPositions uses, without trajex, so it builds
// on every platform. Not safe for concurrent use: one goroutine sends, and Close counts as a use.
// Close must be called by the sending goroutine after its last Send; Close concurrent with Send,
// or Send after Close, can panic. To stop from another goroutine, cancel the ctx passed to
// NewArmStreamer (which ends the RPC) or signal the sending loop.
type ArmStreamer struct{ s *armStream }

// NewArmStreamer opens the stream to a. diag may be diagnostics.New(0). Cancelling ctx ends the
// arm RPC, after which Done is closed.
func NewArmStreamer(ctx context.Context, a arm.Arm, diag *diagnostics.SingleSessionDiagnostics) *ArmStreamer {
	return &ArmStreamer{s: newArmStream(ctx, a, diag)}
}

// Send sends one batch. It fails, wrapping the RPC's error, once the arm has ended the stream.
func (a *ArmStreamer) Send(ctx context.Context, pts []Point) error {
	p := make([]pvat, len(pts))
	for i, q := range pts {
		p[i] = pvat{positions: q.Positions, velocities: q.Velocities, accelerations: q.Accelerations, time: q.Time}
	}
	return a.s.send(ctx, p)
}

// Runway estimates how much sent motion the arm has yet to execute. It goes negative once the arm
// has executed past the last point sent, so treat <= 0 as starved.
func (a *ArmStreamer) Runway() time.Duration { return a.s.currentEstimatedRunwayInArm() }

// Done is closed when the arm's RPC returns, for any reason.
func (a *ArmStreamer) Done() <-chan struct{} { return a.s.moveThroughJointPositionsStreamedReturned }

// Close ends the stream and returns the RPC's final error. Call it exactly once.
func (a *ArmStreamer) Close() error { return a.s.close() }
