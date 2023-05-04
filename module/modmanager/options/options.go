// Package modmanageroptions provides Options for configuring a mod manager
package modmanageroptions

import "go.viam.com/rdk/resource"

// Options configures a modManager.
type Options struct {
	UntrustedEnv bool

	// MarkResourcesRemoved is a function that the module manager can call to
	// mark orphaned resources as removed in the resource graph.
	MarkResourcesRemoved func(rNames []resource.Name,
		addNames func(names ...resource.Name)) []resource.Resource
}
