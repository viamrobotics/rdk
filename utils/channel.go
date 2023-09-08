package utils

// FlushChan is a function that takes a generic chanel and completely empties it.
func FlushChan[T any](c chan T) {
	for gotSomething := true; gotSomething; {
		select {
		case <-c:
		default:
			gotSomething = false
		}
	}
}
