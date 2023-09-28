//go:build !no_cgo
package robotimpl

import "go.viam.com/rdk/services/vision"

var visionSubtypeName string = vision.API.SubtypeName
