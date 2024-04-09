package utils

type Guard struct {
	success bool
	OnFail  func()
}

func NewGuard(onFailCleanup func()) *Guard {
	ret := &Guard{}
	ret.OnFail = func() {
		if !ret.success {
			onFailCleanup()
		}
	}
	return ret
}

func (guard *Guard) Success() {
	guard.success = true
}
