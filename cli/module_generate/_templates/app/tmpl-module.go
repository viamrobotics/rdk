package {{ .ModuleLowercase }}

import (
	"context"
	"embed"
	"io/fs"

	"github.com/erh/vmodutils"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

//go:embed dist/**
var staticFS embed.FS

func distFS() (fs.FS, error) {
	return fs.Sub(staticFS, "dist")
}

var Model = resource.NewModel("{{ .Namespace }}", "{{ .ModuleName }}", "webapp")

type Config struct {
	resource.TriviallyValidateConfig

	Port *int `json:"port,omitempty"`
}

func init() {
	resource.RegisterComponent(generic.API, Model,
		resource.Registration[resource.Resource, *Config]{
			Constructor: NewServer,
		},
	)
}

func NewServer(_ context.Context, _ resource.Dependencies, rawConf resource.Config, logger logging.Logger) (resource.Resource, error) {
	conf, err := resource.NativeConfig[*Config](rawConf)
	if err != nil {
		return nil, err
	}

	fs, err := distFS()
	if err != nil {
		return nil, err
	}

	port := 8888
	if conf.Port != nil {
		port = *conf.Port
	}

	return vmodutils.NewWebModuleAndStart(rawConf.ResourceName(), fs, logger, port)
}
