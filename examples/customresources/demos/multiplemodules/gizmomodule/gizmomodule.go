// Package main is a module, which serves the mygizmosummer custom model type in the customresources examples.
package main

import (
	"go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	"go.viam.com/rdk/examples/customresources/models/mygizmosummer"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
)

func main() {
	// ModularMain will stand up a module which will host our gizmo.
	module.ModularMain(resource.APIModel{gizmoapi.API, mygizmosummer.Model})
}
