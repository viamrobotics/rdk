package rtppassthrough

import (
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"
)

const queueSize int = 16

func TestStreamSubscription(t *testing.T) {
	t.Run("NewStreamSubscription", func(t *testing.T) {
		onError := func(error) {
			t.Log("should not be called")
			t.FailNow()
		}
		t.Run("returns an err if queueSize is negative power of two", func(t *testing.T) {
			_, err := NewStreamSubscription(-1, nil)
			test.That(t, err, test.ShouldBeError, ErrNegativeQueueSize)
			_, err = NewStreamSubscription(-1, onError)
			test.That(t, err, test.ShouldBeError, ErrNegativeQueueSize)
		})

		t.Run("returns an err if queueSize is not power of two", func(t *testing.T) {
			_, err := NewStreamSubscription(3, nil)
			test.That(t, err, test.ShouldBeError, errors.New("size must be a power of two"))
			_, err = NewStreamSubscription(3, onError)
			test.That(t, err, test.ShouldBeError, errors.New("size must be a power of two"))
		})

		t.Run("returns no err otherwise", func(t *testing.T) {
			_, err := NewStreamSubscription(0, nil)
			test.That(t, err, test.ShouldBeNil)
			_, err = NewStreamSubscription(0, onError)
			test.That(t, err, test.ShouldBeNil)
			_, err = NewStreamSubscription(1, nil)
			test.That(t, err, test.ShouldBeNil)
			_, err = NewStreamSubscription(1, onError)
			test.That(t, err, test.ShouldBeNil)
			_, err = NewStreamSubscription(2, nil)
			test.That(t, err, test.ShouldBeNil)
			_, err = NewStreamSubscription(2, onError)
			test.That(t, err, test.ShouldBeNil)
			_, err = NewStreamSubscription(4, nil)
			test.That(t, err, test.ShouldBeNil)
			_, err = NewStreamSubscription(4, onError)
			test.That(t, err, test.ShouldBeNil)
			_, err = NewStreamSubscription(8, nil)
			test.That(t, err, test.ShouldBeNil)
			_, err = NewStreamSubscription(8, onError)
			test.That(t, err, test.ShouldBeNil)
		})
	})

	t.Run("ID", func(t *testing.T) {
		t.Run("is unique", func(t *testing.T) {
			a, err := NewStreamSubscription(queueSize, nil)
			test.That(t, err, test.ShouldBeNil)
			b, err := NewStreamSubscription(queueSize, nil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, a.ID(), test.ShouldNotResemble, b.ID())
		})
	})

	t.Run("Publish", func(t *testing.T) {
		t.Run("calls onError with error when callback returns error", func(t *testing.T) {
			expectedErr := errors.New("testerror")
			errChan := make(chan error)
			onError := func(err error) {
				errChan <- err
			}

			ss, err := NewStreamSubscription(queueSize, onError)
			test.That(t, err, test.ShouldBeNil)
			defer ss.Close()

			ss.Start()

			err = ss.Publish(func() error {
				return expectedErr
			})
			test.That(t, err, test.ShouldBeNil)

			err = <-errChan
			test.That(t, err, test.ShouldBeError, expectedErr)

			select {
			case <-errChan:
				t.Log("should not happen")
				t.FailNow()
			default:
			}
		})

		t.Run("does not execute subsequent callback functions after the first one returns err", func(t *testing.T) {
			expectedErr := errors.New("testerror")
			unexpectedErr := errors.New("should not happen")
			errChan := make(chan error)
			onError := func(err error) {
				errChan <- err
			}

			ss, err := NewStreamSubscription(queueSize, onError)
			test.That(t, err, test.ShouldBeNil)
			defer ss.Close()

			ss.Start()

			err = ss.Publish(func() error {
				return expectedErr
			})
			test.That(t, err, test.ShouldBeNil)

			err = <-errChan
			test.That(t, err, test.ShouldBeError, expectedErr)

			err = ss.Publish(func() error {
				t.Log("this should not happen")
				t.FailNow()
				return unexpectedErr
			})
			test.That(t, err, test.ShouldBeError, expectedErr)

			select {
			case <-errChan:
				t.Log("should not happen")
				t.FailNow()
			default:
			}
		})

		t.Run("defers processing callbacks until after Start is called", func(t *testing.T) {
			onError := func(err error) {
				t.Log("should not happen")
				t.FailNow()
			}

			ss, err := NewStreamSubscription(queueSize, onError)
			test.That(t, err, test.ShouldBeNil)

			publishCalledChan := make(chan struct{}, queueSize*2)
			err = ss.Publish(func() error {
				publishCalledChan <- struct{}{}
				return nil
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
			onError := func(err error) {
				t.Log("should not happen")
				t.FailNow()
			}

			ss, err := NewStreamSubscription(queueSize, onError)
			test.That(t, err, test.ShouldBeNil)

			ss.Start()
			ss.Close()

			err = ss.Publish(func() error {
				t.Log("should not happen")
				t.FailNow()
				return nil
			})

			test.That(t, err, test.ShouldBeError, ErrClosed)
		})

		t.Run("drops callbacks after the queue size is reached", func(t *testing.T) {
			onError := func(err error) {
				t.Log("should not happen")
				t.FailNow()
			}

			ss, err := NewStreamSubscription(queueSize, onError)
			test.That(t, err, test.ShouldBeNil)
			defer ss.Close()

			publishCalledChan := make(chan int, queueSize*2)
			for i := 0; i < queueSize; i++ {
				tmp := i
				err = ss.Publish(func() error {
					publishCalledChan <- tmp
					return nil
				})
				test.That(t, err, test.ShouldBeNil)
			}

			err = ss.Publish(func() error {
				t.Log("should not happen")
				t.FailNow()
				return nil
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
