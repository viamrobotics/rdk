// Package main is a module that serves the acme:demo:combouser model. It is deliberately a
// separate module from the composite it depends on, so the dependency handle is a real composite
// client rather than an in-process Go handle.
package main

import (
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/examples/customresources/models/combouser"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
)

func main() {
	module.ModularMain(resource.APIModel{API: generic.API, Model: combouser.Model})
}
