package streaming

import (
	"context"
	"errors"

	arm "go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/referenceframe"
)

// pvatChItem is the producer -> consumer type on the PVATs channel: either a
// PVAT, or a marker telling the consumer to end the current arm stream.
type pvatChItem struct {
	p           pvat
	closeStream bool
}

type coordinator struct {
	producer *pvatProducer
	consumer *pvatConsumer
}

func newCoordinator(a arm.Arm, cfg *Config) *coordinator {
	pvatsCh := make(chan pvatChItem, max(1, cfg.BufferAheadInArmMs/cfg.SendToArmIntervalMs))
	return &coordinator{
		producer: &pvatProducer{cfg: cfg, pvatsCh: pvatsCh},
		consumer: &pvatConsumer{arm: a, cfg: cfg, pvatsCh: pvatsCh},
	}
}

// run starts the producer and consumer goroutines and returns a handle the caller should wait on.
func (c *coordinator) run(ctx context.Context, targets <-chan Target, seed []referenceframe.Input) *runHandle {
	ctx, cancel := context.WithCancel(ctx)
	r := &runHandle{
		producer: producerRunHandle{done: make(chan struct{}), cancel: cancel},
		consumer: consumerRunHandle{done: make(chan struct{}), cancel: cancel},
	}
	go c.producer.run(ctx, targets, seed, &r.producer)
	go c.consumer.run(ctx, &r.consumer)
	return r
}

type producerRunHandle struct {
	done chan struct{} // closed when the producer returns
	err  error
	// cancel stops the consumer when the producer fails
	cancel context.CancelFunc
}

type consumerRunHandle struct {
	done chan struct{} // closed when the consumer returns
	err  error
	// cancel stops the producer when the consumer fails
	cancel context.CancelFunc
}

type runHandle struct {
	producer producerRunHandle
	consumer consumerRunHandle
}

func (r *runHandle) wait() error {
	<-r.producer.done
	<-r.consumer.done
	r.consumer.cancel()
	// Report the root cause. Whichever side fails first cancels the shared context, which surfaces on the
	// other as context.Canceled, so a non-canceled error is the originating failure.
	err := r.producer.err
	if err == nil || errors.Is(err, context.Canceled) {
		err = r.consumer.err
	}
	return err
}
