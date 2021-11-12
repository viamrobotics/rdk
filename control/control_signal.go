package control

//Signal holds an data passed between blocks
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
