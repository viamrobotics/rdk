// Package register registers all relevant vision models and also subtype specific functions
package register

import (
	// for vision models.
	_ "go.viam.com/rdk/services/vision/color_detector"
	_ "go.viam.com/rdk/services/vision/detections_to_3dsegments"
	_ "go.viam.com/rdk/services/vision/radius_clustering"
)
