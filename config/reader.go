package config

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"
	"github.com/mitchellh/copystructure"

	"go.viam.com/utils"
)

// An AttributeConverter converts a single attribute into a possibly
// different representation.
type AttributeConverter func(val interface{}) (interface{}, error)

// An AttributeMapConverter converts an attribute map into a possibly
// different representation.
type AttributeMapConverter func(attributes AttributeMap) (interface{}, error)

// A ComponentAttributeConverterRegistration describes how to convert a specific attribute
// for a model of a type of component.
type ComponentAttributeConverterRegistration struct {
	CompType ComponentType
	Model    string
	Attr     string
	Conv     AttributeConverter
}

// A ComponentAttributeMapConverterRegistration describes how to convert all attributes
// for a model of a type of component.
type ComponentAttributeMapConverterRegistration struct {
	CompType ComponentType
	Model    string
	Conv     AttributeMapConverter
	RetType  interface{} // the shape of what is converted to
}

// A ServiceAttributeMapConverterRegistration describes how to convert all attributes
// for a model of a type of service.
type ServiceAttributeMapConverterRegistration struct {
	SvcType ServiceType
	Conv    AttributeMapConverter
	RetType interface{} // the shape of what is converted to
}

var (
	componentAttributeConverters    = []ComponentAttributeConverterRegistration{}
	componentAttributeMapConverters = []ComponentAttributeMapConverterRegistration{}
	serviceAttributeMapConverters   = []ServiceAttributeMapConverterRegistration{}
)

// RegisterComponentAttributeConverter associates a component type and model with a way to convert a
// particular attribute name.
func RegisterComponentAttributeConverter(CompType ComponentType, model, attr string, conv AttributeConverter) {
	componentAttributeConverters = append(componentAttributeConverters, ComponentAttributeConverterRegistration{CompType, model, attr, conv})
}

// RegisterComponentAttributeMapConverter associates a component type and model with a way to convert all attributes.
func RegisterComponentAttributeMapConverter(compType ComponentType, model string, conv AttributeMapConverter, retType interface{}) {
	if retType == nil {
		panic("retType should not be nil")
	}
	componentAttributeMapConverters = append(componentAttributeMapConverters, ComponentAttributeMapConverterRegistration{compType, model, conv, retType})
}

// RegisterServiceAttributeMapConverter associates a service type with a way to convert all attributes.
func RegisterServiceAttributeMapConverter(svcType ServiceType, conv AttributeMapConverter, retType interface{}) {
	if retType == nil {
		panic("retType should not be nil")
	}
	serviceAttributeMapConverters = append(serviceAttributeMapConverters, ServiceAttributeMapConverterRegistration{svcType, conv, retType})
}

// RegisteredComponentAttributeConverters returns a copy of the registered component attribute converters.
func RegisteredComponentAttributeConverters() []ComponentAttributeConverterRegistration {
	copied, err := copystructure.Copy(componentAttributeConverters)
	if err != nil {
		panic(err)
	}
	return copied.([]ComponentAttributeConverterRegistration)
}

// RegisteredComponentAttributeMapConverters returns a copy of the registered component attribute converters.
func RegisteredComponentAttributeMapConverters() []ComponentAttributeMapConverterRegistration {
	copied, err := copystructure.Copy(componentAttributeMapConverters)
	if err != nil {
		panic(err)
	}
	return copied.([]ComponentAttributeMapConverterRegistration)
}

// RegisteredServiceAttributeMapConverters returns a copy of the registered component attribute converters.
func RegisteredServiceAttributeMapConverters() []ServiceAttributeMapConverterRegistration {
	copied, err := copystructure.Copy(serviceAttributeMapConverters)
	if err != nil {
		panic(err)
	}
	return copied.([]ServiceAttributeMapConverterRegistration)
}

func findConverter(compType ComponentType, model, attr string) AttributeConverter {
	for _, r := range componentAttributeConverters {
		if r.CompType == compType && r.Model == model && r.Attr == attr {
			return r.Conv
		}
	}
	return nil
}

func findMapConverter(CompType ComponentType, model string) AttributeMapConverter {
	for _, r := range componentAttributeMapConverters {
		if r.CompType == CompType && r.Model == model {
			return r.Conv
		}
	}
	return nil
}

func findServiceMapConverter(svcType ServiceType) AttributeMapConverter {
	for _, r := range serviceAttributeMapConverters {
		if r.SvcType == svcType {
			return r.Conv
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

	return nil, errors.Errorf("cannot find file: %s", target)
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
	defer utils.UncheckedErrorFunc(subFile.Close)

	var sub map[string]interface{}
	decoder := json.NewDecoder(subFile)
	if err := decoder.Decode(&sub); err != nil {
		return nil, false, err
	}
	return sub, true, nil
}

var (
	defaultCloudBasePath = "https://app.viam.com"
	defaultCloudPath     = defaultCloudBasePath + "/api/json1/config"
	defaultCoudLogPath   = defaultCloudBasePath + "/api/json1/log"
)

const (
	cloudConfigSecretField           = "Secret"
	cloudConfigUserInfoField         = "User-Info"
	cloudConfigUserInfoHostField     = "host"
	cloudConfigUserInfoOSField       = "os"
	cloudConfigUserInfoLocalIPsField = "ips"
)

// CreateCloudRequest makes a request to fetch the robot config
// from a cloud endpoint.
func CreateCloudRequest(cloudCfg *Cloud) (*http.Request, error) {
	if cloudCfg.Path == "" {
		cloudCfg.Path = defaultCloudPath
	}
	if cloudCfg.LogPath == "" {
		cloudCfg.LogPath = defaultCoudLogPath
	}

	url := fmt.Sprintf("%s?id=%s", cloudCfg.Path, cloudCfg.ID)

	r, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.Errorf("error creating request for %s : %w", url, err)
	}
	r.Header.Set(cloudConfigSecretField, cloudCfg.Secret)

	userInfo := map[string]interface{}{}
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	userInfo[cloudConfigUserInfoHostField] = hostname
	userInfo[cloudConfigUserInfoOSField] = runtime.GOOS

	ips, err := utils.GetAllLocalIPv4s()
	if err != nil {
		return nil, err
	}
	userInfo[cloudConfigUserInfoLocalIPsField] = ips

	userInfoBytes, err := json.Marshal(userInfo)
	if err != nil {
		return nil, err
	}

	r.Header.Set(cloudConfigUserInfoField, string(userInfoBytes))

	return r, nil
}

var viamDotDir = filepath.Join(os.Getenv("HOME"), ".viam")

func getCloudCacheFilePath(id string) string {
	return filepath.Join(viamDotDir, fmt.Sprintf("cached_cloud_config_%s.json", id))
}

func openFromCache(id string) (io.ReadCloser, error) {
	return os.Open(getCloudCacheFilePath(id))
}

func storeToCache(id string, cfg *Config) error {
	if err := os.MkdirAll(viamDotDir, 0700); err != nil {
		return err
	}
	md, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(getCloudCacheFilePath(id), md, 0640)
}

// ReadFromCloud fetches a robot config from the cloud based
// on the given config.
func ReadFromCloud(cloudCfg *Cloud, readFromCache bool) (*Config, error) {
	cloudReq, err := CreateCloudRequest(cloudCfg)
	if err != nil {
		return nil, err
	}

	var client http.Client
	defer client.CloseIdleConnections()
	resp, err := client.Do(cloudReq)

	var configReader io.ReadCloser
	if err == nil {
		if resp.StatusCode != http.StatusOK {
			defer utils.UncheckedErrorFunc(resp.Body.Close)
			rd, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return nil, err
			}
			if len(rd) != 0 {
				return nil, errors.Errorf("unexpected status %d: %s", resp.StatusCode, string(rd))
			}
			return nil, errors.Errorf("unexpected status %d", resp.StatusCode)
		}
		configReader = resp.Body
	} else {
		if !readFromCache {
			return nil, err
		}
		var urlErr *url.Error
		if !errors.As(err, &urlErr) || urlErr.Temporary() {
			return nil, err
		}
		var cacheErr error
		configReader, cacheErr = openFromCache(cloudCfg.ID)
		if cacheErr != nil {
			if os.IsNotExist(cacheErr) {
				return nil, err
			}
			return nil, cacheErr
		}
	}
	defer utils.UncheckedErrorFunc(configReader.Close)

	// read the actual config and do not make a cloud request again to avoid
	// infinite recursion.
	cfg, err := fromReader("", configReader, true)
	if err != nil {
		return nil, err
	}
	if cfg.Cloud == nil {
		return nil, errors.New("expected config to have cloud section")
	}
	self := cfg.Cloud.Self
	signalingAddress := cfg.Cloud.SignalingAddress
	*cfg.Cloud = *cloudCfg
	cfg.Cloud.Self = self
	cfg.Cloud.SignalingAddress = signalingAddress

	if err := storeToCache(cloudCfg.ID, cfg); err != nil {
		golog.Global.Errorw("failed to cache config", "error", err)
	}

	return cfg, nil
}

// Read reads a config from the given file.
func Read(filePath string) (*Config, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer utils.UncheckedErrorFunc(file.Close)

	return FromReader(filePath, file)
}

// FromReader reads a config from the given reader and specifies
// where, if applicable, the file the reader originated from.
func FromReader(originalPath string, r io.Reader) (*Config, error) {
	return fromReader(originalPath, r, false)
}

func fromReader(originalPath string, r io.Reader, skipCloud bool) (*Config, error) {
	cfg := Config{
		ConfigFilePath: originalPath,
	}

	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, errors.Errorf("cannot parse config %w", err)
	}
	if err := cfg.Ensure(skipCloud); err != nil {
		return nil, err
	}

	if !skipCloud && cfg.Cloud != nil {
		return ReadFromCloud(cfg.Cloud, true)
	}

	for idx, c := range cfg.Components {
		conv := findMapConverter(c.Type, c.Model)
		if conv == nil {
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
					return nil, errors.Errorf("error converting attribute for (%s, %s, %s) %w", c.Type, c.Model, k, err)
				}
				cfg.Components[idx].Attributes[k] = n

			}
			continue
		}

		converted, err := conv(c.Attributes)
		if err != nil {
			return nil, errors.Errorf("error converting attributes for (%s, %s) %w", c.Type, c.Model, err)
		}
		cfg.Components[idx].Attributes = nil
		cfg.Components[idx].ConvertedAttributes = converted
	}

	for idx, c := range cfg.Services {
		conv := findServiceMapConverter(c.Type)
		if conv == nil {
			continue
		}

		converted, err := conv(c.Attributes)
		if err != nil {
			return nil, errors.Errorf("error converting attributes for %s %w", c.Type, err)
		}
		cfg.Services[idx].Attributes = nil
		cfg.Services[idx].ConvertedAttributes = converted
	}

	if err := cfg.Ensure(skipCloud); err != nil {
		return nil, err
	}
	return &cfg, nil
}
