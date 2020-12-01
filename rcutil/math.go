package rcutil

func AbsInt(n int) int {
	if n < 0 {
		return -1 * n
	}
	return n
}

// Math.pow( x, 2 ) is slow, this is faster
func Square(n float64) float64 {
	return n * n
}

// Math.pow( x, 2 ) is slow, this is faster
func SquareInt(n int) int {
	return n * n
}
