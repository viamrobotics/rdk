// Package register registers all relevant vision models and also subtype specific functions
package register

import (
	// for vision models.
	_ "go.viam.com/rdk/services/vision/colordetector"
	_ "go.viam.com/rdk/services/vision/detectionstosegments"
	_ "go.viam.com/rdk/services/vision/mlvision"
	_ "go.viam.com/rdk/services/vision/radiusclustering"
	_ "go.viam.com/rdk/services/vision/tflite"
)
