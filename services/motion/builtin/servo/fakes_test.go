package servo

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.viam.com/rdk/services/motion/builtin/streaming"
)

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

// fakeStreamer records points; End(err) simulates the arm ending the stream.
type fakeStreamer struct {
	mu      sync.Mutex
	pts     []streaming.Point
	done    chan struct{}
	endErr  error
	closed  int
	block   chan struct{} // if set, Close waits for it
	sendErr error         // if set, Send fails with it
}

func newFakeStreamer() *fakeStreamer { return &fakeStreamer{done: make(chan struct{})} }

func (f *fakeStreamer) Send(_ context.Context, p []streaming.Point) error {
	select {
	case <-f.done:
		return fmt.Errorf("stream ended: %w", f.endErr)
	default:
	}
	if f.sendErr != nil {
		return f.sendErr
	}
	f.mu.Lock()
	f.pts = append(f.pts, p...)
	f.mu.Unlock()
	return nil
}

func (f *fakeStreamer) Runway() time.Duration { return 0 }
func (f *fakeStreamer) Done() <-chan struct{} { return f.done }

func (f *fakeStreamer) Close() error {
	f.mu.Lock()
	f.closed++
	f.mu.Unlock()
	if f.block != nil {
		<-f.block
	}
	return nil
}

func (f *fakeStreamer) End(err error) {
	f.mu.Lock()
	f.endErr = err
	f.mu.Unlock()
	close(f.done)
}

func (f *fakeStreamer) points() []streaming.Point {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]streaming.Point(nil), f.pts...)
}
