// Package action defines a registry for high-level actions to perform on any Robot.
//
// For example, an action might be to walk around for a few minutes.
package action

import (
	"context"

	"github.com/pkg/errors"

	"go.viam.com/rdk/robot"
)

// An Action is some high-level process that is performed on a robot.
type Action func(ctx context.Context, r robot.Robot)

var actionRegistry = map[string]Action{}

// RegisterAction associates a name to an action.
func RegisterAction(name string, action Action) {
	_, old := actionRegistry[name]
	if old {
		panic(errors.Errorf("trying to register 2 actions with the same name (%s)", name))
	}
	actionRegistry[name] = action
}

// LookupAction return the action associated with the given name,
// if it exists; nil is returned otherwise.
func LookupAction(name string) Action {
	return actionRegistry[name]
}

// AllActionNames returns all registered action names.
func AllActionNames() []string {
	names := []string{}
	for k := range actionRegistry {
		names = append(names, k)
	}
	return names
}
