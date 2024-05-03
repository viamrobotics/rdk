package utils

import (
	"sync"
	"time"
)

func TryLockWithTimeout(mut *sync.Mutex, timeout time.Duration) error {
	if mut.TryLock() {
		return nil
	}
	time.Sleep(timeout / 2)
	if mut.TryLock() {
		return nil
	}
	time.Sleep(timeout / 2)
	if mut.TryLock() {
		return nil
	}
	return timeoutErrorHelper("TryLockTimeout", timeout, "timed out trying mutex")
}
