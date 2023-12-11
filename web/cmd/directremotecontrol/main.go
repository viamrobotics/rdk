/*
Package main provides a utility to run a remote control only web server. It is configured with
a config file that looks like the following:

	{
		"webrtc_enabled": true,
		"host": "something.0000000000.viam.cloud",
		"webrtc_signaling_address": "https://app.viam.com:443",
		"webrtc_additional_ice_servers": [{"credential":"cred","urls":"turn:global.turn.twilio.com:3478","username":"cred"}],
		"baked_auth": {
			"authEntity": "something.0000000000.viam.cloud",
			"creds": {
				"type": "some-key",
				"payload": "payload"
			}
		}
	}

# Usage

# Run with development server

	ENV=development go run go.viam.com/rdk/web/cmd/directremotecontrol --config ~/some/config_like_above.json
*/
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/Masterminds/sprig"
	"github.com/NYTimes/gziphandler"
	"go.viam.com/utils"
	"goji.io"
	"goji.io/pat"

	"go.viam.com/rdk/logging"
	robotweb "go.viam.com/rdk/robot/web"
	"go.viam.com/rdk/web"
)

// Arguments for the command.
type Arguments struct {
	Config string            `flag:"config,required"`
	Port   utils.NetPortFlag `flag:"port,default=8080"`
}

var logger = logging.NewDebugLogger("robot_server")

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

func mainWithArgs(ctx context.Context, args []string, logger logging.Logger) error {
	var argsParsed Arguments
	if err := utils.ParseFlags(args, &argsParsed); err != nil {
		return err
	}

	configData, err := os.ReadFile(argsParsed.Config)
	if err != nil {
		return err
	}

	var data robotweb.AppTemplateData
	if err := json.Unmarshal(configData, &data); err != nil {
		return err
	}
	if data.Host == "" {
		return errors.New("must set host in config")
	}

	if data.Env == "" {
		if os.Getenv("ENV") == "development" {
			data.Env = "development"
		} else {
			data.Env = "production"
		}
	}

	listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%s", argsParsed.Port.String()))
	if err != nil {
		return err
	}

	mux := goji.NewMux()

	embedFS, err := fs.Sub(web.AppFS, "runtime-shared/static")
	if err != nil {
		return err
	}
	staticDir := http.FS(embedFS)

	t := template.New("foo").Funcs(template.FuncMap{
		//nolint:gosec
		"jsSafe": func(js string) template.JS {
			return template.JS(js)
		},
		//nolint:gosec
		"htmlSafe": func(html string) template.HTML {
			return template.HTML(html)
		},
	}).Funcs(sprig.FuncMap())

	t, err = t.ParseFS(web.AppFS, "runtime-shared/templates/*.html")
	if err != nil {
		return err
	}
	template := t.Lookup("webappindex.html")

	mux.Handle(pat.Get("/static/*"), gziphandler.GzipHandler(http.StripPrefix("/static", http.FileServer(staticDir))))
	mux.Handle(pat.Get("/"), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dataCopy := data

		if err := r.ParseForm(); err != nil {
			logger.CDebugw(ctx, "failed to parse form", "error", err)
		}
		if r.Form.Get("grpc") == "true" {
			data.WebRTCEnabled = false
		}

		if err := template.Execute(w, dataCopy); err != nil {
			logger.CDebugw(ctx, "couldn't execute web page", "error", err)
		}
	}))

	httpServer := http.Server{
		Handler:           mux,
		ReadHeaderTimeout: time.Second * 5,
	}

	utils.PanicCapturingGo(func() {
		<-ctx.Done()
		defer func() {
			if err := httpServer.Shutdown(context.Background()); err != nil {
				logger.Errorw("error shutting down", "error", err)
			}
		}()
	})

	logger.Infow("serving", "address", listener.Addr().String())
	return httpServer.Serve(listener)
}
