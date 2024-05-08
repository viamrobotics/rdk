package rtppassthrough

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"
)

const queueSize int = 16

func TestStreamSubscription(t *testing.T) {
	t.Run("NewSubscription", func(t *testing.T) {
		t.Run("returns an err if queueSize is negative power of two", func(t *testing.T) {
			_, _, err := NewSubscription(-1)
			test.That(t, err, test.ShouldBeError, ErrBufferSize)
		})

		t.Run("returns an err if queueSize is not power of two", func(t *testing.T) {
			_, _, err := NewSubscription(3)
			test.That(t, err, test.ShouldBeError, errors.New("size must be a power of two"))
		})

		t.Run("returns no err otherwise", func(t *testing.T) {
			_, _, err := NewSubscription(0)
			test.That(t, err, test.ShouldBeNil)
			_, _, err = NewSubscription(1)
			test.That(t, err, test.ShouldBeNil)
			_, _, err = NewSubscription(2)
			test.That(t, err, test.ShouldBeNil)
			_, _, err = NewSubscription(4)
			test.That(t, err, test.ShouldBeNil)
			_, _, err = NewSubscription(8)
			test.That(t, err, test.ShouldBeNil)
		})
	})

	t.Run("ID", func(t *testing.T) {
		t.Run("is unique", func(t *testing.T) {
			subA, _, err := NewSubscription(queueSize)
			test.That(t, err, test.ShouldBeNil)
			subB, _, err := NewSubscription(queueSize)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, subA.ID, test.ShouldNotResemble, subB.ID)
		})
	})

	t.Run("Publish", func(t *testing.T) {
		t.Run("defers processing callbacks until after Start is called", func(t *testing.T) {
			_, buffer, err := NewSubscription(queueSize)
			test.That(t, err, test.ShouldBeNil)

			publishCalledChan := make(chan struct{}, queueSize*2)
			err = buffer.Publish(func() {
				publishCalledChan <- struct{}{}
			})

			test.That(t, err, test.ShouldBeNil)
			select {
			case <-publishCalledChan:
				t.Log("should not happen")
				t.FailNow()
			default:
			}

			buffer.Start()
			defer buffer.Close()
			<-publishCalledChan
		})

		t.Run("returns err if called after Close is called and does not process callback", func(t *testing.T) {
			_, buffer, err := NewSubscription(queueSize)
			test.That(t, err, test.ShouldBeNil)

			buffer.Start()
			buffer.Close()

			err = buffer.Publish(func() {
				t.Log("should not happen")
				t.FailNow()
			})

			test.That(t, err, test.ShouldBeError, ErrClosed)
		})

		t.Run("drops callbacks after the queue size is reached", func(t *testing.T) {
			_, buffer, err := NewSubscription(queueSize)
			test.That(t, err, test.ShouldBeNil)
			defer buffer.Close()

			publishCalledChan := make(chan int, queueSize*2)
			for i := 0; i < queueSize; i++ {
				tmp := i
				err = buffer.Publish(func() {
					publishCalledChan <- tmp
				})
				test.That(t, err, test.ShouldBeNil)
			}

			err = buffer.Publish(func() {
				t.Log("should not happen")
				t.FailNow()
			})

			test.That(t, err, test.ShouldBeError, ErrQueueFull)

			buffer.Start()

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

	t.Run("Close", func(t *testing.T) {
		t.Run("succeeds if called before Start()", func(t *testing.T) {
			_, buffer, err := NewSubscription(queueSize)
			test.That(t, err, test.ShouldBeNil)
			buffer.Close()
		})

		t.Run("succeeds if called after Start()", func(t *testing.T) {
			_, buffer, err := NewSubscription(queueSize)
			test.That(t, err, test.ShouldBeNil)
			buffer.Start()
			buffer.Close()
		})

		t.Run("terminates the Subscription", func(t *testing.T) {
			sub, buffer, err := NewSubscription(queueSize)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, sub.Terminated.Err(), test.ShouldBeNil)
			buffer.Close()
			test.That(t, sub.Terminated.Err(), test.ShouldBeError, context.Canceled)
		})
	})
}
