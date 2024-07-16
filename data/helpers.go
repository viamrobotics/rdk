package data

import "time"

// GetDurationFromHz gets time.Duration from hz.
func GetDurationFromHz(captureFrequencyHz float32) time.Duration {
	if captureFrequencyHz == 0 {
		return time.Duration(0)
	}
	return time.Duration(float32(time.Second) / captureFrequencyHz)
}
