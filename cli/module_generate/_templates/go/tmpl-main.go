package main

import (
	"{{.ModuleName}}/models"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/{{.ResourceType}}s/{{.ResourceSubtype}}"

)

func main() {
	module.ModularMain(resource.APIModel{ {{.ResourceSubtype}}.API, models.{{.ModelPascal}} },
	                   // If your module implements multiple models, add the rest here
				       )
}
