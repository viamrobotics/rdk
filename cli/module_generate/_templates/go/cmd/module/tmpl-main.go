package main

import (
	"{{.ModuleLowercase}}"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
	{{.ResourceSubtypeAlias}} "go.viam.com/rdk/{{.ResourceType}}s/{{.ResourceSubtype}}"
)

func main() {
	// ModularMain can take multiple APIModel arguments, if your module implements multiple models.
	module.ModularMain(resource.APIModel{ {{.ResourceSubtypeAlias}}.API, {{.ModuleLowercase}}.{{.ModelPascal}}})
}
