// Package modmanageroptions provides Options for configuring a mod manager
package modmanageroptions

import (
	"context"

	"go.viam.com/rdk/resource"
)

// Options configures a modManager.
type Options struct {
	UntrustedEnv bool

	// Root of Viam server configuration. This is generally in either /opt/viam or ~/.viam
	ViamHomeDir string

	// Cloud ID of the robot. Empty if this is a local-only robot
	RobotCloudID string

	// RemoveOrphanedResources is a function that the module manager can call to
	// remove orphaned resources from the resource graph.
	RemoveOrphanedResources func(ctx context.Context, rNames []resource.Name)
}
