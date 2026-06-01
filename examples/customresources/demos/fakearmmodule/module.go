// Package main is a test module that exposes a fake arm under a custom model.
// The component implementation is reused from the built-in RDK fake arm package.
package main

import (
	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/arm/fake"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
)

var myModel = resource.NewModel("acme", "demo", "fakearm")

func main() {
	resource.RegisterComponent(arm.API, myModel, resource.Registration[arm.Arm, *fake.Config]{
		Constructor: fake.NewArm,
	})

	module.ModularMain(resource.APIModel{API: arm.API, Model: myModel})
}
