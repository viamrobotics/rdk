// Package picommon contains shared information for supported and non-supported pi boards.
package picommon

import (
	"go.viam.com/rdk/resource"
)

// Model is the name used refer to any implementation of a pi based component.
var Model = resource.DefaultModelFamily.WithModel("pi")
