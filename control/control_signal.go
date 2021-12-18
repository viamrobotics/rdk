package control

//Signal holds any data passed between blocks
type Signal struct {
	signal    []float64
	time      []int
	dimension int
	name      string
}

func makeSignal(name string, dimension int) Signal {
	var s Signal
	s.dimension = dimension
	s.signal = make([]float64, dimension)
	s.time = make([]int, dimension)
	s.name = name
	return s
}

// GetValAt Return the signal value at index
func (s *Signal) GetValAt(i int) float64 {
	if i > len(s.signal)-1 {
		return 0.0
	}
	return s.signal[i]
}
