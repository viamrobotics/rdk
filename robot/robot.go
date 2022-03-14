// Package robot defines the robot which is the root of all robotic parts.
package robot

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/pexec"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
)

// A Robot encompasses all functionality of some robot comprised
// of parts, local and remote.
type Robot interface {
	// RemoteByName returns a remote robot by name.
	RemoteByName(name string) (Robot, bool)

	// ResourceByName returns a resource by name
	ResourceByName(name resource.Name) (interface{}, error)

	// RemoteNames returns the name of all known remote robots.
	RemoteNames() []string

	// FunctionNames returns the name of all known functions.
	FunctionNames() []string

	// ResourceNames returns a list of all known resource names
	ResourceNames() []resource.Name

	// ProcessManager returns the process manager for the robot.
	ProcessManager() pexec.ProcessManager

	// Config returns the config used to construct the robot.
	// This is allowed to be partial or empty.
	Config(ctx context.Context) (*config.Config, error)

	// FrameSystem returns a FrameSystem suitable for doing reference frame lookups
	// and then computing relative offsets of pieces.
	// The frame system will be given a name, and its parts given a prefix (both optional).
	FrameSystem(ctx context.Context, name, prefix string) (referenceframe.FrameSystem, error)

	// Logger returns the logger the robot is using.
	Logger() golog.Logger

	// Close attempts to cleanly close down all constituent parts of the robot.
	Close(ctx context.Context) error
}

// A Refresher can refresh the contents of a robot.
type Refresher interface {
	// Refresh instructs the Robot to manually refresh the contents of itself.
	Refresh(ctx context.Context) error
}

// A LocalRobot is a Robot that can have its parts modified.
type LocalRobot interface {
	Robot

	// Reconfigure instructs the robot to safely reconfigure itself based
	// on the given new config.
	Reconfigure(ctx context.Context, newConfig *config.Config) error
}

// AllResourcesByName returns an array of all resources that have this simple name.
func AllResourcesByName(r Robot, name string) []interface{} {
	all := []interface{}{}

	for _, n := range r.ResourceNames() {
		if n.Name == name {
			r, err := r.ResourceByName(n)
			if err != nil {
				panic("this should be impossible")
			}
			all = append(all, r)
		}
	}

	return all
}

// NamesBySubtype is a helper for getting all names from the given Robot given the subtype.
func NamesBySubtype(r Robot, subtype resource.Subtype) []string {
	names := []string{}
	for _, n := range r.ResourceNames() {
		if n.Subtype == subtype {
			names = append(names, n.Name)
		}
	}
	return names
}
