// Package main is a module that serves the acme:demo:combodevice composite model. The single
// model is advertised under all three of its APIs, so one instance shows up as one resource
// reachable as a gizmo, a summation, and a sensor.
package main

import (
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	"go.viam.com/rdk/examples/customresources/apis/summationapi"
	"go.viam.com/rdk/examples/customresources/models/combodevice"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
)

func main() {
	module.ModularMain(
		resource.APIModel{API: gizmoapi.API, Model: combodevice.Model},
		resource.APIModel{API: summationapi.API, Model: combodevice.Model},
		resource.APIModel{API: sensor.API, Model: combodevice.Model},
	)
}
