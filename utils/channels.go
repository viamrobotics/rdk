package utils

import "sync"

// WaitWithError blocks until the WaitGroup counter is zero. After that, it returns the
// first error received by the given error channel or nil if there are no errors.
func WaitWithError(wg *sync.WaitGroup, errors chan error) error {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		done <- struct{}{}
	}()
	select {
	case err := <-errors:
		return err
	case <-done:
	}

	return nil
}
