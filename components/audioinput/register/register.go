// Package register registers all relevant audio inputs and also subtype specific functions
package register

import (
	// for audio inputs.
	_ "go.viam.com/rdk/components/audioinput/fake"
	_ "go.viam.com/rdk/components/audioinput/microphone"
)
