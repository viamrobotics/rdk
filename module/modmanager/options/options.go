// Package modmanageroptions provides Options for configuring a mod manager
package modmanageroptions

import (
	"context"

	"go.viam.com/rdk/ftdc"
	"go.viam.com/rdk/resource"
)

// Options configures a modManager.
type Options struct {
	UntrustedEnv bool
	// ViamHomeDir is the root of Viam server configuration. This is ~/.viam for now but will become /opt/viam as part of viam-agent
	ViamHomeDir string
	// RobotCloudID is the ID of the robot in app.viam.com. Empty if this is a local-only robot
	RobotCloudID string
	// RemoveOrphanedResources is a function that the module manager can call to
	// remove orphaned resources from the resource graph.
	RemoveOrphanedResources func(ctx context.Context, rNames []resource.Name)
	// PackagesDir is from Config.PackagesPath. It's used for resolving local tarball module paths.
	PackagesDir string
	// Passing in an FTDC object will let the mod manager add and remove pieces to track diagnostics
	// of. Such module process CPU/memory information as they are started and stopped.
	FTDC *ftdc.FTDC
}
