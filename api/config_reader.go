package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"runtime"
	"strings"

	"github.com/edaniels/golog"
)

type AttributeConverter func(val interface{}) (interface{}, error)

type registered struct {
	comptype ComponentType
	model    string
	attr     string
	f        AttributeConverter
}

var (
	special = []registered{}
)

func Register(comptype ComponentType, model, attr string, f AttributeConverter) {
	special = append(special, registered{comptype, model, attr, f})
}

func findRegisterd(comptype ComponentType, model, attr string) *registered {
	for _, r := range special {
		if r.comptype == comptype && r.model == model && r.attr == attr {
			return &r
		}
	}
	return nil
}

func findPath(original, fn string) (*os.File, error) {
	f, err := os.Open(fn)
	if err == nil {
		return f, nil
	}

	fn2 := path.Join(path.Dir(original), fn)
	f, err = os.Open(fn2)
	if err == nil {
		return f, nil
	}

	return nil, fmt.Errorf("cannot find file: %s", fn)
}

func loadSubFromFile(original, cmd string) (interface{}, bool, error) {
	if !strings.HasPrefix(cmd, "$load{") {
		return cmd, false, nil
	}

	cmd = cmd[6:]
	cmd = cmd[0 : len(cmd)-1]

	f, err := findPath(original, cmd)
	if err != nil {
		return cmd, false, err
	}
	defer f.Close()

	sub := map[string]interface{}{}
	decoder := json.NewDecoder(f)
	err = decoder.Decode(&sub)

	return sub, true, err
}

func createRequest(cloudCfg *CloudConfig) (*http.Request, error) {
	if cloudCfg.Path == "" {
		cloudCfg.Path = "https://app.viam.com/api/json1/config"
	}
	if cloudCfg.LogPath == "" {
		cloudCfg.LogPath = "https://app.viam.com/api/json1/log"
	}

	url := fmt.Sprintf("%s?id=%s", cloudCfg.Path, cloudCfg.ID)
	golog.Global.Debugf("reading config from %s", url)

	r, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request for %s : %w", url, err)
	}
	r.Header.Set("Secret", cloudCfg.Secret)

	userInfo := map[string]interface{}{}
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	userInfo["host"] = hostname
	userInfo["os"] = runtime.GOOS

	userInfoBytes, err := json.Marshal(userInfo)
	if err != nil {
		return nil, err
	}

	r.Header.Set("User-Info", string(userInfoBytes))

	return r, nil
}

func ReadConfigFromCloud(cloudCfg CloudConfig) (Config, error) {
	cfg := Config{}

	r, err := createRequest(&cloudCfg)
	if err != nil {
		return cfg, err
	}

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return cfg, err
	}
	defer resp.Body.Close()

	cfg, err = ReadConfigFromReader("", resp.Body)
	cfg.Cloud = cloudCfg
	return cfg, err
}

// TODO(erh): should make this return a *Config (and all downstream)
func ReadConfig(fn string) (Config, error) {
	cfg := Config{}

	file, err := os.Open(fn)
	if err != nil {
		return cfg, err
	}
	defer file.Close()

	return ReadConfigFromReader(fn, file)
}

func ReadConfigFromReader(originalPath string, r io.Reader) (Config, error) {
	cfg := Config{}

	decoder := json.NewDecoder(r)
	err := decoder.Decode(&cfg)
	if err != nil {
		return cfg, fmt.Errorf("cannot parse config %w", err)
	}

	if cfg.Cloud.ID != "" {
		return ReadConfigFromCloud(cfg.Cloud)
	}

	for idx, c := range cfg.Components {
		for k, v := range c.Attributes {
			s, ok := v.(string)
			if ok {
				cfg.Components[idx].Attributes[k] = os.ExpandEnv(s)
				loaded := false
				v, loaded, err = loadSubFromFile(originalPath, s)
				if err != nil {
					return cfg, err
				}
				if loaded {
					cfg.Components[idx].Attributes[k] = v
				}
			}

			r := findRegisterd(c.Type, c.Model, k)
			if r != nil {
				n, err := r.f(v)
				if err != nil {
					return cfg, fmt.Errorf("error fixing type for (%s, %s, %s) %w", c.Type, c.Model, k, err)
				}
				cfg.Components[idx].Attributes[k] = n
			}

		}
	}

	return cfg, nil
}
