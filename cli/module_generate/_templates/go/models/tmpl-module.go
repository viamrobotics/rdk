package models

import (
    "go.viam.com/rdk/{{ .ResourceType }}s/{{ .ResourceSubtype}}"
)

var (
	{{.ModelPascal}} = resource.NewModel("{{ .Namespace}}", "{{ .ModuleName}}", "{{.ModelName}}")
	errUnimplemented = errors.New("unimplemented")
)

func init() {
	resource.Register{{ .ResourceType}}({{.ResourceSubtype}}.API, {{.ModelPascal}},
		resource.Registration[{{.ResourceSubtype}}.{{if eq .ResourceType "component"}}{{.ResourceSubtypePascal}}{{else}}Service{{end}}, *Config]{
			Constructor: new{{.ModulePascal}}{{.ModelPascal}},
		},
	)
}


type Config struct {
	// Put config attributes here

	/* if your model  does not need a config,
	   replace *Config in the init function with resource.NoNativeConfig */

	/* Uncomment this if your model does not need to be validated
	    and has no implicit dependecies. */
	// resource.TriviallyValidateConfig

}

func (cfg *Config) Validate(path string) ([]string, error) {
	// Add config validation code here
	 return nil, nil
}

type {{.ModuleCamel}}{{.ModelPascal}} struct {
	name   resource.Name

	logger logging.Logger
	cfg    *Config

	cancelCtx  context.Context
	cancelFunc func()


	/* Uncomment this if your model does not need to reconfigure. */
	// resource.TriviallyReconfigurable

	// Uncomment this if the model does not have any goroutines that
	// need to be shut down while closing.
	// resource.TriviallyCloseable

}


func new{{.ModulePascal}}{{.ModelPascal}}(ctx context.Context, deps resource.Dependencies, rawConf resource.Config, logger logging.Logger) ({{.ResourceSubtype}}.{{if eq .ResourceType "component"}}{{.ResourceSubtypePascal}}{{else}}Service{{end}}, error) {
	conf, err := resource.NativeConfig[*Config](rawConf)
	if err != nil {
		return nil, err
	}

	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	s := &{{.ModuleCamel}}{{.ModelPascal}}{
		name:       rawConf.ResourceName(),
		logger:     logger,
		cfg:        conf,
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
	}
	return s, nil
}


func (s *{{.ModuleCamel}}{{.ModelPascal}}) Name() resource.Name {
	return s.name
}

func (s *{{.ModuleCamel}}{{.ModelPascal}}) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	// Put reconfigure code here
	return nil
}


func (s *{{.ModuleCamel}}{{.ModelPascal}}) Close(context.Context) error {
	return nil
}