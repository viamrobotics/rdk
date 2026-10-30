package main

import (
{{- if eq .ResourceType "component"}}
	{{.ResourceSubtypeAlias}} "go.viam.com/rdk/{{.ResourceType}}s/{{.ResourceSubtype}}"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
{{- else}}
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
	{{.ResourceSubtypeAlias}} "go.viam.com/rdk/{{.ResourceType}}s/{{.ResourceSubtype}}"
{{- end}}

	"{{.ModuleLowercase}}"
)

func main() {
	// ModularMain can take multiple APIModel arguments, if your module implements multiple models.
	module.ModularMain(resource.APIModel{API: {{.ResourceSubtypeAlias}}.API, Model: {{.ModuleLowercase}}.{{.ModelPascal}}})
}
