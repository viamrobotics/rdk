// Package modmanageroptions provides Options for configuring a mod manager
package modmanageroptions

import (
	"context"

	"go.viam.com/rdk/resource"
)

// Options configures a modManager.
type Options struct {
	UntrustedEnv bool

	// Root of Viam server configuration. This is ~/.viam for now but will be migrating to /opt/viam as part of viam-agent
	ViamHomeDir string

	// Cloud ID of the robot. Empty if this is a local-only robot
	RobotCloudID string

	// RemoveOrphanedResources is a function that the module manager can call to
	// remove orphaned resources from the resource graph.
	RemoveOrphanedResources func(ctx context.Context, rNames []resource.Name)
}
