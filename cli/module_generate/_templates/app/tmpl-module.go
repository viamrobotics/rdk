package {{ .ModuleLowercase }}

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net/http"

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

type webApp struct {
	resource.AlwaysRebuild

	name   resource.Name
	srv    resource.Resource
	logger logging.Logger
	cfg    *Config
}

func init() {
	resource.RegisterComponent(generic.API, Model,
		resource.Registration[resource.Resource, *Config]{
			Constructor: NewServer,
		},
	)
}

func NewServer(ctx context.Context, deps resource.Dependencies, rawConf resource.Config, logger logging.Logger) (resource.Resource, error) {
	conf, err := resource.NativeConfig[*Config](rawConf)
	if err != nil {
		return nil, err
	}
	return newWebApp(ctx, deps, rawConf.ResourceName(), conf, logger)
}

func newWebApp(ctx context.Context, deps resource.Dependencies, name resource.Name, conf *Config, logger logging.Logger) (resource.Resource, error) {
	fs, err := distFS()
	if err != nil {
		return nil, err
	}

	server, err := vmodutils.NewWebModuleWithCookies(name, fs, logger, nil)
	if err != nil {
		return nil, err
	}

	port := 8888
	if conf.Port != nil {
		port = *conf.Port
	}
	if err := server.Start(port); err != nil {
		return nil, err
	}

	return &webApp{
		name:   name,
		srv:    server,
		logger: logger,
		cfg:    conf,
	}, nil
}

func (w *webApp) Name() resource.Name {
	return w.name
}

func (w *webApp) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return nil, nil
}

func (w *webApp) Close(ctx context.Context) error {
	return w.srv.Close(ctx)
}
