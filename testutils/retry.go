package testutils

import "time"

type Condition func() bool

// Retry sleeps until a condition is met or a max of numRetries times
func Retry(condition Condition, numRetries int) {
	for i := 0; i < numRetries && !condition(); i++ {
		time.Sleep(time.Second)
	}
}
