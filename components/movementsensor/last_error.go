package movementsensor

import (
	"sync"
)

type LastError struct {
	err error
	mu  sync.Mutex
}

func (le *LastError) Set(err error) {
	le.mu.Lock()
	defer le.mu.Unlock()
	le.err = err
}

func (le *LastError) Get() error {
	le.mu.Lock()
	defer le.mu.Unlock()

	err := le.err
	le.err = nil
	return err
}
