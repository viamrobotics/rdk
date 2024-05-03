package utils

import (
	"sync"
	"testing"
	"time"

	"go.viam.com/test"
)

func TestTryLockWithTimeout(t *testing.T) {
	durt := time.Second / 20

	t.Run("succeed", func(t *testing.T) {
		mut := &sync.Mutex{}
		err := TryLockWithTimeout(mut, durt)
		test.That(t, err, test.ShouldBeNil)
		mut.Unlock()
	})

	t.Run("succeed-after-delay", func(t *testing.T) {
		mut := &sync.Mutex{}
		mut.Lock()
		go func() {
			time.Sleep(durt / 2)
			mut.Unlock()
		}()
		ch := make(chan error)
		go func() {
			ch <- TryLockWithTimeout(mut, durt)
		}()
		err := <-ch
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("timeout", func(t *testing.T) {
		mut := &sync.Mutex{}
		mut.Lock()
		defer mut.Unlock()
		ch := make(chan error)
		go func() {
			ch <- TryLockWithTimeout(mut, durt)
		}()
		err := <-ch
		test.That(t, err, test.ShouldNotBeNil)
	})
}
