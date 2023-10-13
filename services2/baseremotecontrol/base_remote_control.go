// Package baseremotecontrol implements a remote control for a base.
package baseremotecontrol

import (
	"context"

	"go.viam.com/rdk/components/input"
	"go.viam.com/rdk/resource"
)

// SubtypeName is the name of the type of service.
const SubtypeName = "base_remote_control"

// API is a variable that identifies the remote control resource API.
var API = resource.APINamespaceRDK.WithServiceType(SubtypeName)

// Named is a helper for getting the named base remote control service's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[Service]{})
}

// A Service is the basis for the base remote control.
type Service interface {
	resource.Resource
	// Close out of all remote control related systems.
	Close(ctx context.Context) error
	// controllerInputs returns the list of inputs from the controller that are being monitored for that control mode.
	ControllerInputs() []input.Control
}
