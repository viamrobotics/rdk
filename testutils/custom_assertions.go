package testutils

import (
	"github.com/go-errors/errors"
)

// RunForPanic tests whether a panic was encountered when running
// a function.
// NOTE: if your function takes arguments, you must wrap the function call
// in a function that does not and supply the latter function as the argument
// to RunForPanic
func RunForPanic(f func()) (didPanic bool, err error) {
	defer func() {
		r := recover()
		if r != nil {
			switch x := r.(type) {
			case string:
				err = errors.New(x)
			case error:
				err = x
			default:
				err = errors.New("encountered panic of unknown type")
			}
			didPanic = true
		}
	}()
	f()
	return false, nil
}
