//go:build !no_cgo || android

package register

import (
	// register services.
	_ "go.viam.com/rdk/services/mlmodel/register"
	_ "go.viam.com/rdk/services/vision/register"
)
