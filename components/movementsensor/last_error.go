package movementsensor

import (
	"sync"
)

type lastError struct {
	err error
	mu  sync.Mutex
}

func (le *lastError) Set(err error) {
	le.mu.Lock()
	defer le.mu.Unlock()
	le.err = err
}

func (le *lastError) Get() error {
	le.mu.Lock()
	defer le.mu.Unlock()

	err := le.err
	le.err = nil
	return err
}
