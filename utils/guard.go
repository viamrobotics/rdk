package utils

// Guard is a structure for managing special cleanup for when a function that returns an allocated
// resource (e.g: open file) fails. Programmers are often presented with two options when dealing
// with early-return error cases:
//   - Don't use defers. On success, the resource (e.g: file object) is returned, but on failure the
//     programmer must ensure the resource (e.g: file) is closed.
//   - Use defers to close the resource in anticipation of failure. But wrap the defer with an `if
//     !success` check. And when the function returns for the success case, it must flip a `bool
//     success` from false - > true.
//
// A Guard encapsulates the second technique. Correct usage of a Guard uses the following pattern:
//
//	guard := NewGuard(func() { f.Close() })
//	defer guard.OnFail()
//	if (error) { return error }
//	guard.Success()
//	return nil
type Guard struct {
	OnFail  func()
	success bool
}

// NewGuard returns a NewGuard.
func NewGuard(onFailCleanup func()) *Guard {
	ret := &Guard{}
	ret.OnFail = func() {
		if !ret.success {
			onFailCleanup()
		}
	}
	return ret
}

// Success declares the function succeeded and the "failure" cleanup code does not need to be
// executed.
func (guard *Guard) Success() {
	guard.success = true
}
