package control

import "sync"

// Signal holds any data passed between blocks.
type Signal struct {
	signal    []float64
	time      []int
	dimension int
	name      string
	mu        *sync.Mutex
}

//nolint: unparam
func makeSignal(name string, dimension int) Signal {
	var s Signal
	s.dimension = dimension
	s.signal = make([]float64, dimension)
	s.time = make([]int, dimension)
	s.name = name
	s.mu = &sync.Mutex{}
	return s
}

// GetSignalValueAt returns the value of the signal at an index, threadsafe.
func (s *Signal) GetSignalValueAt(i int) float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i > len(s.signal)-1 {
		return 0.0
	}
	return s.signal[i]
}

// SetSignalValueAt set the value of a signal at an index, threadsafe.
func (s *Signal) SetSignalValueAt(i int, val float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i > len(s.signal)-1 {
		return
	}
	s.signal[i] = val
}
