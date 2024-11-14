// Package baseremotecontrol implements a remote control for a base.
// For more information, see the [base remote control service docs].
//
// [base remote control service docs]: https://docs.viam.com/services/base-rc/
package baseremotecontrol

import (
	"context"

	"go.viam.com/rdk/components/input"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

// SubtypeName is the name of the type of service.
const SubtypeName = "base_remote_control"

// API is a variable that identifies the remote control resource API.
var API = resource.APINamespaceRDK.WithServiceType(SubtypeName)

// Named is a helper for getting the named base remote control service's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// FromRobot is a helper for getting the named base remote control service from the given Robot.
func FromRobot(r robot.Robot, name string) (Service, error) {
	return robot.ResourceFromRobot[Service](r, Named(name))
}

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[Service]{})
}

// A Service is the basis for the base remote control.
// For more information, see the [base remote control service docs].
//
// Close example:
//
//	// Close out of all remote control-related systems.
//	err := baseRCService.Close(context.Background())
//
// ControllerInputs example:
//
//	// Get the list of inputs from the controller that are being monitored for that control mode.
//	inputs := baseRCService.ControllerInputs()
//
// [base remote control service docs]: https://docs.viam.com/services/base-rc/
type Service interface {
	resource.Resource
	// Close out of all remote control related systems.
	Close(ctx context.Context) error
	// controllerInputs returns the list of inputs from the controller that are being monitored for that control mode.
	ControllerInputs() []input.Control
}
