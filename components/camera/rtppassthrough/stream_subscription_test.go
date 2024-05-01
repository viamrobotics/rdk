package rtppassthrough

import (
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"
)

const queueSize int = 16

func TestStreamSubscription(t *testing.T) {
	t.Run("NewStreamSubscription", func(t *testing.T) {
		t.Run("returns an err if queueSize is negative power of two", func(t *testing.T) {
			_, err := NewStreamSubscription(-1)
			test.That(t, err, test.ShouldBeError, ErrNegativeQueueSize)
		})

		t.Run("returns an err if queueSize is not power of two", func(t *testing.T) {
			_, err := NewStreamSubscription(3)
			test.That(t, err, test.ShouldBeError, errors.New("size must be a power of two"))
		})

		t.Run("returns no err otherwise", func(t *testing.T) {
			_, err := NewStreamSubscription(0)
			test.That(t, err, test.ShouldBeNil)
			_, err = NewStreamSubscription(1)
			test.That(t, err, test.ShouldBeNil)
			_, err = NewStreamSubscription(2)
			test.That(t, err, test.ShouldBeNil)
			_, err = NewStreamSubscription(4)
			test.That(t, err, test.ShouldBeNil)
			_, err = NewStreamSubscription(8)
			test.That(t, err, test.ShouldBeNil)
		})
	})

	t.Run("ID", func(t *testing.T) {
		t.Run("is unique", func(t *testing.T) {
			a, err := NewStreamSubscription(queueSize)
			test.That(t, err, test.ShouldBeNil)
			b, err := NewStreamSubscription(queueSize)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, a.ID(), test.ShouldNotResemble, b.ID())
		})
	})

	t.Run("Publish", func(t *testing.T) {
		t.Run("defers processing callbacks until after Start is called", func(t *testing.T) {
			ss, err := NewStreamSubscription(queueSize)
			test.That(t, err, test.ShouldBeNil)

			publishCalledChan := make(chan struct{}, queueSize*2)
			err = ss.Publish(func() {
				publishCalledChan <- struct{}{}
			})

			test.That(t, err, test.ShouldBeNil)
			select {
			case <-publishCalledChan:
				t.Log("should not happen")
				t.FailNow()
			default:
			}

			ss.Start()
			defer ss.Close()
			<-publishCalledChan
		})

		t.Run("returns err if called after Close is called and does not process callback", func(t *testing.T) {
			ss, err := NewStreamSubscription(queueSize)
			test.That(t, err, test.ShouldBeNil)

			ss.Start()
			ss.Close()

			err = ss.Publish(func() {
				t.Log("should not happen")
				t.FailNow()
			})

			test.That(t, err, test.ShouldBeError, ErrClosed)
		})

		t.Run("drops callbacks after the queue size is reached", func(t *testing.T) {
			ss, err := NewStreamSubscription(queueSize)
			test.That(t, err, test.ShouldBeNil)
			defer ss.Close()

			publishCalledChan := make(chan int, queueSize*2)
			for i := 0; i < queueSize; i++ {
				tmp := i
				err = ss.Publish(func() {
					publishCalledChan <- tmp
				})
				test.That(t, err, test.ShouldBeNil)
			}

			err = ss.Publish(func() {
				t.Log("should not happen")
				t.FailNow()
			})

			test.That(t, err, test.ShouldBeError, ErrQueueFull)

			ss.Start()

			for i := 0; i < queueSize; i++ {
				tmp := <-publishCalledChan
				test.That(t, tmp, test.ShouldEqual, i)
			}
			select {
			case <-publishCalledChan:
				t.Log("should not happen")
				t.FailNow()
			default:
			}
		})
	})
}
