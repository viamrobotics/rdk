package logging

import "fmt"

// FloatArrayFormat is a helper to format float arrays
type FloatArrayFormat struct {
	Fmt  string // "%0.2f"
	Data []float64
}

// String makes a string
func (a FloatArrayFormat) String() string {
	f := a.Fmt
	if f == "" {
		f = "% 0.2f"
	}
	s := "["
	for idx, v := range a.Data {
		if idx > 0 {
			s += ", "
		}
		s += fmt.Sprintf(f, v)
	}
	return s + "]"
}
