package {{ .ModuleLowercase }}

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"

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
	server *http.Server
	logger logging.Logger
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
	return newWebApp(rawConf.ResourceName(), conf, logger)
}

func newWebApp(name resource.Name, conf *Config, logger logging.Logger) (resource.Resource, error) {
	distFiles, err := distFS()
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServerFS(distFiles))

	handler := &cookieSetter{
		handler: mux,
		logger:  logger,
	}
	handler.addEnvCookie("MACHINE_FQDN", "host")
	handler.addEnvCookie("API_KEY_ID", "api-key-id")
	handler.addEnvCookie("API_KEY", "api-key")

	port := 8888
	if conf.Port != nil {
		port = *conf.Port
	}

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: handler,
	}

	logger.Infof("starting web app server on port %d", port)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Errorf("web app server error: %v", err)
		}
	}()

	return &webApp{
		name:   name,
		server: server,
		logger: logger,
	}, nil
}

func (w *webApp) Name() resource.Name {
	return w.name
}

func (w *webApp) DoCommand(_ context.Context, _ map[string]interface{}) (map[string]interface{}, error) {
	return nil, nil
}

func (w *webApp) Status(_ context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{"status": "running"}, nil
}

func (w *webApp) Close(_ context.Context) error {
	return w.server.Close()
}

// cookieSetter is an http.Handler that injects credential cookies into every response.
type cookieSetter struct {
	handler http.Handler
	cookies []*http.Cookie
	logger  logging.Logger
}

func (cs *cookieSetter) addEnvCookie(envVar, cookieName string) {
	v := os.Getenv(envVar)
	if v == "" {
		cs.logger.Warnf("no value for env var %s (cookie %s)", envVar, cookieName)
	}
	cs.cookies = append(cs.cookies, &http.Cookie{Name: cookieName, Value: v})
}

func (cs *cookieSetter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for _, c := range cs.cookies {
		http.SetCookie(w, c)
	}
	cs.handler.ServeHTTP(w, r)
}
