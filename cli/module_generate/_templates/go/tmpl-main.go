package main

import (
	"{{.ModuleName}}/models"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/{{.ResourceType}}s/{{.ResourceSubtype}}"
)

func main() {
	// ModularMain can take multiple APIModel arguments, if your module implements multiple models.
	module.ModularMain(resource.APIModel{ {{.ResourceSubtype}}.API, models.{{.ModelPascal}}})
}
