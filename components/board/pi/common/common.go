// Package picommon contains shared information for supported and non-supported pi boards.
package picommon

import (
	"go.viam.com/rdk/resource"
)

// ModelName is the name used refer to any implementation of a pi based component.
var ModelName = resource.NewDefaultModel("pi")
