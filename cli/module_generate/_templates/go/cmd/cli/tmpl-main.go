package main

import (
	"context"

{{- if eq .ResourceType "component"}}

	{{.ResourceSubtypeAlias}} "go.viam.com/rdk/{{.ResourceType}}s/{{.ResourceSubtype}}"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
{{- else}}

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	{{.ResourceSubtypeAlias}} "go.viam.com/rdk/{{.ResourceType}}s/{{.ResourceSubtype}}"
{{- end}}

	"{{.ModuleLowercase}}"
)

func main() {
	err := realMain()
	if err != nil {
		panic(err)
	}
}

func realMain() error {
	ctx := context.Background()
	logger := logging.NewLogger("cli")

	deps := resource.Dependencies{}
	// can load these from a remote machine if you need

	cfg := {{.ModuleLowercase}}.Config{}

	thing, err := {{.ModuleLowercase}}.New{{.ModelPascal}}(ctx, deps, {{.ResourceSubtypeAlias}}.Named("foo"), cfg, logger)
	if err != nil {
		return err
	}
	defer func() {
		if err := thing.Close(ctx); err != nil {
			logger.Error(err)
		}
	}()

	return nil
}
