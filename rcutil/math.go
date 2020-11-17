package rcutil

func AbsInt(n int) int {
	if n < 0 {
		return -1 * n
	}
	return n
}
