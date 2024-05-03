package utils

import (
	"sync"
	"time"
)

// TryLockWithTimeout tries mut twice during timeout then errors. It returns nil if it gets the lock.
// If timeout is nil, it calls mut.Lock.
func TryLockWithTimeout(mut *sync.Mutex, timeout *time.Duration) error {
	if timeout == nil {
		mut.Lock()
		return nil
	}
	if mut.TryLock() {
		return nil
	}
	time.Sleep(*timeout / 2)
	if mut.TryLock() {
		return nil
	}
	time.Sleep(*timeout / 2)
	if mut.TryLock() {
		return nil
	}
	return timeoutErrorHelper("TryLockTimeout", *timeout, "timed out trying mutex")
}
