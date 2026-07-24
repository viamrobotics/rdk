package builtin

import (
	"context"
	"fmt"
	"time"

	"go.viam.com/rdk/referenceframe"
)

type pvatProducer struct {
	cfg           *streamConfig
	trajexSession *trajexSession
	pvatsCh       chan<- pvatChItem
	trace         *pipelineTrace

	numTrajexSessions                int
	numPVATsSampledThisTrajexSession int
}

func (p *pvatProducer) run(ctx context.Context, targets <-chan jointPositionsChItem, seed []referenceframe.Input, r *producerRunHandle) {
	defer func() {
		close(p.pvatsCh)
		if p.trajexSession != nil {
			p.trajexSession.close()
			p.trace.recordEvent(pipeEventSessionClose, "")
		}
		close(r.done)
	}()

	fail := func(err error) {
		if p.trajexSession != nil {
			err = fmt.Errorf("session #%d (%d pvats sampled, seed=%v): %w",
				p.numTrajexSessions, p.numPVATsSampledThisTrajexSession, p.trajexSession.lastJointPositions, err)
		} else {
			err = fmt.Errorf("after %d trajex sessions: %w", p.numTrajexSessions, err)
		}
		r.err = err
		r.cancel()
	}

	var err error
	if p.trajexSession, err = newTrajexSession(p.cfg, seed); err != nil {
		fail(fmt.Errorf("newTrajexSession (seed=%v): %w", seed, err))
		return
	}
	p.numTrajexSessions++
	p.trace.recordEvent(pipeEventSessionOpen, "")

	var nextPVAT *pvat
	for {
		// nextPVAT is nil when a new trajex session has just been started or if the trajex session was
		// sampled through because no new targets came in time.
		// Since `select` evaluates send operands up front for every case:
		// (1) To avoid a nil deref panic from `pvatsCh <- *nextPVAT`, leave toEnqueue as zero.
		// (2) To avoid pushing a zero value into the real channel, leave sendCh as nil.
		var toEnqueue pvatChItem
		var sendCh chan<- pvatChItem
		if nextPVAT != nil {
			toEnqueue, sendCh = pvatChItem{p: *nextPVAT}, p.pvatsCh
		}

		// Note that if multiple cases are ready, `select` picks one at random.
		select {
		// Cancel was called.
		case <-ctx.Done():
			r.err = ctx.Err()
			return

		// A new target is available.
		case target, ok := <-targets:
			if !ok {
				// Targets channel closed.
				if err := p.enqueueRemainingPVATs(ctx, nextPVAT); err != nil {
					fail(err)
					return
				}
				return
			}
			p.trace.record(pipeChanPlanQ, pipeOpDequeue, len(targets), cap(targets))

			// On a stop-acceptable target or a trajex session that can't be extended to the target,
			// drain the current trajex session and start a fresh one.
			jointPositions := target.Positions
			extendStart := time.Now()
			extendErr := p.trajexSession.addJointPositions(ctx, jointPositions)
			p.trace.recordTiming(pipeTimingExtend, time.Since(extendStart))
			if target.StopAcceptable || extendErr != nil {
				seed = p.trajexSession.lastJointPositions
				if err := p.enqueueRemainingPVATs(ctx, nextPVAT); err != nil {
					fail(err)
					return
				}
				if err := p.enqueueCloseStream(ctx); err != nil {
					fail(err)
					return
				}

				p.trajexSession.close()
				p.trace.recordEvent(pipeEventSessionClose, "")
				if p.trajexSession, err = newTrajexSession(p.cfg, seed); err != nil {
					fail(fmt.Errorf("newTrajexSession (seed=%v): %w", seed, err))
					return
				}
				p.numTrajexSessions++
				p.numPVATsSampledThisTrajexSession = 0
				p.trace.recordEvent(pipeEventSessionOpen, "")
				if err = p.trajexSession.addJointPositions(ctx, jointPositions); err != nil {
					fail(err)
					return
				}

				nextPVAT = nil
			}

		// There's space in sendCh and we have a pvat to enqueue.
		case sendCh <- toEnqueue:
			nextPVAT = nil
			p.trace.record(pipeChanStreamQ, pipeOpEnqueue, len(p.pvatsCh), cap(p.pvatsCh))
		}

		if nextPVAT == nil {
			var err error
			if nextPVAT, err = p.trajexSession.nextPVAT(ctx); err != nil {
				fail(fmt.Errorf("nextPVAT: %w", err))
				return
			}
			p.numPVATsSampledThisTrajexSession++
		}
	}
}

func (p *pvatProducer) enqueueRemainingPVATs(ctx context.Context, nextPVAT *pvat) error {
	if nextPVAT != nil {
		if err := p.enqueuePVAT(ctx, *nextPVAT); err != nil {
			return err
		}
	}
	remaining, err := p.trajexSession.remainingPVATS(ctx)
	if err != nil {
		return fmt.Errorf("remainingPVATS: %w", err)
	}
	for _, pv := range remaining {
		if err := p.enqueuePVAT(ctx, pv); err != nil {
			return err
		}
	}
	return nil
}

func (p *pvatProducer) enqueuePVAT(ctx context.Context, pv pvat) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case p.pvatsCh <- pvatChItem{p: pv}:
		p.trace.record(pipeChanStreamQ, pipeOpEnqueue, len(p.pvatsCh), cap(p.pvatsCh))
		return nil
	}
}

func (p *pvatProducer) enqueueCloseStream(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case p.pvatsCh <- pvatChItem{closeStream: true}:
		p.trace.record(pipeChanStreamQ, pipeOpEnqueue, len(p.pvatsCh), cap(p.pvatsCh))
		return nil
	}
}
