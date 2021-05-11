package api

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"runtime"
	"strings"
)

// An AttributeConverter converts a single attribute into a possibly
// different representation.
type AttributeConverter func(val interface{}) (interface{}, error)

type attributeConverterRegistration struct {
	compType ComponentType
	model    string
	attr     string
	conv     AttributeConverter
}

var (
	attributeConverters = []attributeConverterRegistration{}
)

func Register(compType ComponentType, model, attr string, conv AttributeConverter) {
	attributeConverters = append(attributeConverters, attributeConverterRegistration{compType, model, attr, conv})
}

func findConverter(compType ComponentType, model, attr string) AttributeConverter {
	for _, r := range attributeConverters {
		if r.compType == compType && r.model == model && r.attr == attr {
			return r.conv
		}
	}
	return nil
}

func openSub(original, target string) (*os.File, error) {
	targetFile, err := os.Open(target)
	if err == nil {
		return targetFile, nil
	}

	// try finding it through the original path
	targetFile, err = os.Open(path.Join(path.Dir(original), target))
	if err == nil {
		return targetFile, nil
	}

	return nil, fmt.Errorf("cannot find file: %s", target)
}

func loadSubFromFile(original, cmd string) (interface{}, bool, error) {
	if !strings.HasPrefix(cmd, "$load{") {
		return cmd, false, nil
	}

	cmd = cmd[6:]             // [$load{|...]
	cmd = cmd[0 : len(cmd)-1] // [...|}]

	subFile, err := openSub(original, cmd)
	if err != nil {
		return cmd, false, err
	}
	defer subFile.Close()

	var sub map[string]interface{}
	decoder := json.NewDecoder(subFile)
	if err := decoder.Decode(&sub); err != nil {
		return nil, false, err
	}
	return sub, true, nil
}

const (
	defaultCloudConfigPath = "https://app.viam.com/api/json1/config"
	defaultCoudLogPath     = "https://app.viam.com/api/json1/log"

	cloudConfigSecretField       = "Secret"
	cloudConfigUserInfoField     = "User-Info"
	cloudConfigUserInfoHostField = "host"
	cloudConfigUserInfoOSField   = "os"
)

// createCloudConfigRequest makes a request to fetch the robot config
// from a cloud endpoint.
func createCloudConfigRequest(cloudCfg *CloudConfig) (*http.Request, error) {
	if cloudCfg.Path == "" {
		cloudCfg.Path = defaultCloudConfigPath
	}
	if cloudCfg.LogPath == "" {
		cloudCfg.LogPath = defaultCoudLogPath
	}

	url := fmt.Sprintf("%s?id=%s", cloudCfg.Path, cloudCfg.ID)

	r, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request for %s : %w", url, err)
	}
	r.Header.Set(cloudConfigSecretField, cloudCfg.Secret)

	userInfo := map[string]interface{}{}
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	userInfo[cloudConfigUserInfoHostField] = hostname
	userInfo[cloudConfigUserInfoOSField] = runtime.GOOS

	userInfoBytes, err := json.Marshal(userInfo)
	if err != nil {
		return nil, err
	}

	r.Header.Set(cloudConfigUserInfoField, string(userInfoBytes))

	return r, nil
}

// ReadConfigFromCloud fetches a robot config from the cloud based
// on the given config.
func ReadConfigFromCloud(cloudCfg *CloudConfig) (*Config, error) {
	cloudReq, err := createCloudConfigRequest(cloudCfg)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(cloudReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		rd, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		if len(rd) != 0 {
			return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(rd))
		}
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	// read the actual config and do not make a cloud request again to avoid
	// infinite recursion.
	cfg, err := readConfigFromReader("", resp.Body, true)
	if err != nil {
		return nil, err
	}
	cfg.Cloud = cloudCfg
	return cfg, err
}

// ReadConfig reads a config from the given file.
func ReadConfig(filePath string) (*Config, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return ReadConfigFromReader(filePath, file)
}

// ReadConfigFromReader reads a config from the given reader and specifies
// where, if applicable, the file the reader originated from.
func ReadConfigFromReader(originalPath string, r io.Reader) (*Config, error) {
	return readConfigFromReader(originalPath, r, false)
}

func readConfigFromReader(originalPath string, r io.Reader, skipCloud bool) (*Config, error) {
	cfg := Config{
		ConfigFilePath: originalPath,
	}

	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("cannot parse config %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	if !skipCloud && cfg.Cloud != nil {
		return ReadConfigFromCloud(cfg.Cloud)
	}

	for idx, c := range cfg.Components {
		for k, v := range c.Attributes {
			s, ok := v.(string)
			if ok {
				cfg.Components[idx].Attributes[k] = os.ExpandEnv(s)
				loaded := false
				var err error
				v, loaded, err = loadSubFromFile(originalPath, s)
				if err != nil {
					return nil, err
				}
				if loaded {
					cfg.Components[idx].Attributes[k] = v
				}
			}

			conv := findConverter(c.Type, c.Model, k)
			if conv == nil {
				continue
			}

			n, err := conv(v)
			if err != nil {
				return nil, fmt.Errorf("error converting attribute for (%s, %s, %s) %w", c.Type, c.Model, k, err)
			}
			cfg.Components[idx].Attributes[k] = n

		}
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}
