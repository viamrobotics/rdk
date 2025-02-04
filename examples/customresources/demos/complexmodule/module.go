// Package main is a module, which serves all four custom model types in the customresources examples, including both custom APIs.
package main

import (
	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	"go.viam.com/rdk/examples/customresources/apis/summationapi"
	"go.viam.com/rdk/examples/customresources/models/mybase"
	"go.viam.com/rdk/examples/customresources/models/mygizmo"
	"go.viam.com/rdk/examples/customresources/models/mynavigation"
	"go.viam.com/rdk/examples/customresources/models/mysum"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/navigation"
)

func main() {
	// ModularMain will stand up a module which will host 4 different models.
	module.ModularMain(
		resource.APIModel{gizmoapi.API, mygizmo.Model},
		resource.APIModel{summationapi.API, mysum.Model},
		resource.APIModel{base.API, mybase.Model},
		resource.APIModel{navigation.API, mynavigation.Model},
	)
}
