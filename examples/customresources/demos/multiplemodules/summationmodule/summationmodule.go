// Package main is a module, which serves the mysum custom model type in the customresources examples.
package main

import (
	"go.viam.com/rdk/examples/customresources/apis/summationapi"
	"go.viam.com/rdk/examples/customresources/models/mysum"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
)

func main() {
	// ModularMain will create a logger named "summationmodule" and will then host a module with
	// our mysum model in it.
	module.ModularMain("summationmodule", resource.APIModel{summationapi.API, mysum.Model})
}
