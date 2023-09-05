package utils

// FlushChan is a function that takes a generic chanel and completely empties it
func FlushChan[T any](c chan T) {
	for i := 0; i < len(c); i++ {
		<-c
	}
}
