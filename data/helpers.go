package data

import (
	"time"

	"go.viam.com/rdk/utils"
)

// GetDurationFromHz gets time.Duration from hz.
func GetDurationFromHz(captureFrequencyHz float32) time.Duration {
	// this would be once every ~11.5 days. This is close enough to 0
	if utils.Float32AlmostEqual(captureFrequencyHz, 0, 0.000001) {
		return time.Duration(0)
	}
	return time.Duration(float32(time.Second) / captureFrequencyHz)
}
