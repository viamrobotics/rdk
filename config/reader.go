package config

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"

	"github.com/a8m/envsubst"
	"github.com/edaniels/golog"
	"github.com/mitchellh/copystructure"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"go.viam.com/utils"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/resource"
)

// RDK versioning variables which are replaced by LD flags.
var (
	Version     = ""
	GitRevision = ""
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
	Subtype resource.SubtypeName
	Model   string
	Attr    string
	Conv    AttributeConverter
}

// A ComponentAttributeMapConverterRegistration describes how to convert all attributes
// for a model of a type of component.
type ComponentAttributeMapConverterRegistration struct {
	Subtype resource.SubtypeName
	Model   string
	Conv    AttributeMapConverter
	RetType interface{} // the shape of what is converted to
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
func RegisterComponentAttributeConverter(subtype resource.SubtypeName, model, attr string, conv AttributeConverter) {
	componentAttributeConverters = append(componentAttributeConverters, ComponentAttributeConverterRegistration{subtype, model, attr, conv})
}

// RegisterComponentAttributeMapConverter associates a component type and model with a way to convert all attributes.
func RegisterComponentAttributeMapConverter(subtype resource.SubtypeName, model string, conv AttributeMapConverter, retType interface{}) {
	if retType == nil {
		panic("retType should not be nil")
	}
	componentAttributeMapConverters = append(
		componentAttributeMapConverters,
		ComponentAttributeMapConverterRegistration{subtype, model, conv, retType},
	)
}

// TransformAttributeMapToStruct uses an attribute map to transform attributes to the perscribed format.
func TransformAttributeMapToStruct(to interface{}, attributes AttributeMap) (interface{}, error) {
	var md mapstructure.Metadata
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName:  "json",
		Result:   to,
		Metadata: &md,
	})
	if err != nil {
		return nil, err
	}
	if err := decoder.Decode(attributes); err != nil {
		return nil, err
	}
	if attributes.Has("attributes") || len(md.Unused) == 0 {
		return to, nil
	}
	// set as many unused attributes as possible
	toV := reflect.ValueOf(to)
	if toV.Kind() == reflect.Ptr {
		toV = toV.Elem()
	}
	if attrsV := toV.FieldByName("Attributes"); attrsV.IsValid() &&
		attrsV.Kind() == reflect.Map &&
		attrsV.Type().Key().Kind() == reflect.String {
		if attrsV.IsNil() {
			attrsV.Set(reflect.MakeMap(attrsV.Type()))
		}
		mapValueType := attrsV.Type().Elem()
		for _, key := range md.Unused {
			val := attributes[key]
			valV := reflect.ValueOf(val)
			if valV.Type().AssignableTo(mapValueType) {
				attrsV.SetMapIndex(reflect.ValueOf(key), valV)
			}
		}
	}
	return to, nil
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

func findConverter(subtype resource.SubtypeName, model, attr string) AttributeConverter {
	for _, r := range componentAttributeConverters {
		if r.Subtype == subtype && r.Model == model && r.Attr == attr {
			return r.Conv
		}
	}
	return nil
}

func findMapConverter(subtype resource.SubtypeName, model string) AttributeMapConverter {
	for _, r := range componentAttributeMapConverters {
		if r.Subtype == subtype && r.Model == model {
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

const (
	cloudConfigSecretField           = "Secret"
	cloudConfigUserInfoField         = "User-Info"
	cloudConfigUserInfoHostField     = "host"
	cloudConfigUserInfoOSField       = "os"
	cloudConfigUserInfoLocalIPsField = "ips"
	cloudConfigVersionField          = "version"
	cloudConfigGitRevisionField      = "gitRevision"
)

// CreateCloudRequest makes a request to fetch the robot config
// from a cloud endpoint.
func CreateCloudRequest(ctx context.Context, cloudCfg *Cloud) (*http.Request, error) {
	url := fmt.Sprintf("%s?id=%s", cloudCfg.Path, cloudCfg.ID)

	r, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "error creating request for %s", url)
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
	userInfo[cloudConfigVersionField] = Version
	userInfo[cloudConfigGitRevisionField] = GitRevision

	userInfoBytes, err := json.Marshal(userInfo)
	if err != nil {
		return nil, err
	}

	r.Header.Set(cloudConfigUserInfoField, string(userInfoBytes))

	return r, nil
}

// createCloudCertificateRequest makes a request to fetch the robot's TLS
// certificate from a cloud endpoint.
func createCloudCertificateRequest(ctx context.Context, cloudCfg *Cloud) (*http.Request, error) {
	url := fmt.Sprintf("%s?id=%s&cert=true", cloudCfg.Path, cloudCfg.ID)

	r, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "error creating request for %s", url)
	}
	r.Header.Set(cloudConfigSecretField, cloudCfg.Secret)

	return r, nil
}

var viamDotDir = filepath.Join(os.Getenv("HOME"), ".viam")

func getCloudCacheFilePath(id string) string {
	return filepath.Join(viamDotDir, fmt.Sprintf("cached_cloud_config_%s.json", id))
}

func readFromCache(id string) (*Config, error) {
	r, err := os.Open(getCloudCacheFilePath(id))
	if err != nil {
		return nil, err
	}

	unprocessedConfig := &Config{
		ConfigFilePath: "",
	}
	bytes, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(bytes, unprocessedConfig); err != nil {
		return nil, errors.Wrap(err, "cannot parse cloud config")
	}
	return unprocessedConfig, nil
}

func storeToCache(id string, cfg *Config) error {
	if err := os.MkdirAll(viamDotDir, 0o700); err != nil {
		return err
	}

	md, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	reader := bytes.NewReader(md)

	path := getCloudCacheFilePath(id)

	return artifact.AtomicStore(path, reader, id)
}

// getFromCloud actually does the fetching of the robot config and parses to an unprocessed Config struct.
func getFromCloud(ctx context.Context, cloudCfg *Cloud, shouldReadFromCache bool, logger golog.Logger) (*Config, bool, error) {
	cloudReq, err := CreateCloudRequest(ctx, cloudCfg)
	if err != nil {
		return nil, false, err
	}

	var cached bool
	unprocessedConfig := &Config{
		ConfigFilePath: "",
	}

	var client http.Client
	defer client.CloseIdleConnections()
	resp, err := client.Do(cloudReq)
	// Try to load from the cache
	if err != nil {
		if !shouldReadFromCache {
			return nil, cached, err
		}
		var urlErr *url.Error
		if !errors.Is(err, context.DeadlineExceeded) && (!errors.As(err, &urlErr) || urlErr.Temporary()) {
			return nil, cached, err
		}
		cachedConfig, cacheErr := readFromCache(cloudCfg.ID)
		if cacheErr != nil {
			if os.IsNotExist(cacheErr) {
				// Return original http error if failed to load from cache.
				return nil, cached, err
			}
			// return cache err
			cached = true
			return nil, cached, cacheErr
		}
		logger.Warnw("unable to get cloud config; using cached version", "error", err)
		cached = true
		return cachedConfig, cached, nil
	}

	defer utils.UncheckedErrorFunc(resp.Body.Close)

	rd, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, cached, err
	}

	if resp.StatusCode != http.StatusOK {
		if len(rd) != 0 {
			return nil, cached, errors.Errorf("unexpected status %d: %s", resp.StatusCode, string(rd))
		}
		return nil, cached, errors.Errorf("unexpected status %d", resp.StatusCode)
	}

	if err := json.Unmarshal(rd, unprocessedConfig); err != nil {
		return nil, cached, errors.Wrap(err, "cannot parse cloud config")
	}
	return unprocessedConfig, cached, nil
}

// readCertificateDataFromCloud returns the certificate from the app. It returns it as properties of a new Cloud config.
// The argument `cloudConfigFromDisk` represents the Cloud config from disk and only the Path parameters are used to
// generate the url. This is different from the Cloud config returned from the HTTP or gRPC API which do not have it.
//
// TODO(adam): The TLS certificate data should not be part of the Cloud portion of the config.
func readCertificateDataFromCloud(ctx context.Context, signalingInsecure bool, cloudConfigFromDisk *Cloud) (*Cloud, error) {
	certReq, err := createCloudCertificateRequest(ctx, cloudConfigFromDisk)
	if err != nil {
		return nil, err
	}

	var client http.Client
	defer client.CloseIdleConnections()
	resp, err := client.Do(certReq)
	if err != nil {
		return nil, err
	}
	defer utils.UncheckedErrorFunc(resp.Body.Close)

	dec := json.NewDecoder(resp.Body)
	var certData Cloud
	if err := dec.Decode(&certData); err != nil {
		return nil, errors.Wrap(err, "error decoding certificate data from cloud; try again later")
	}

	if !signalingInsecure {
		if certData.TLSCertificate == "" {
			return nil, errors.New("no TLS certificate yet from cloud; try again later")
		}
		if certData.TLSPrivateKey == "" {
			return nil, errors.New("no TLS private key yet from cloud; try again later")
		}
	}

	// TODO(adam): we might want to use an internal type here. The gRPC api will not return a Cloud json struct.
	return &Cloud{
		TLSCertificate: certData.TLSCertificate,
		TLSPrivateKey:  certData.TLSPrivateKey,
	}, nil
}

// readFromCloud fetches a robot config from the cloud based
// on the given config.
func readFromCloud(
	ctx context.Context,
	originalCfg *Config,
	shouldReadFromCache bool,
	checkForNewCert bool,
	logger golog.Logger,
) (*Config, *Config, error) {
	logger.Debug("reading configuration from the cloud")
	cloudCfg := originalCfg.Cloud
	unprocessedConfig, cached, err := getFromCloud(ctx, cloudCfg, shouldReadFromCache, logger)
	if err != nil {
		if !cached {
			err = errors.Wrapf(err, "error getting cloud config, please make sure the RDK config located in %v is valid", originalCfg.ConfigFilePath)
		}
		return nil, nil, err
	}

	// process the config
	cfg, err := processConfigFromCloud(unprocessedConfig)
	if err != nil {
		return nil, nil, err
	}
	if cfg.Cloud == nil {
		return nil, nil, errors.New("expected config to have cloud section")
	}

	// empty if not cached, since its a separate request, which we check next
	tlsCertificate := cfg.Cloud.TLSCertificate
	tlsPrivateKey := cfg.Cloud.TLSPrivateKey
	if !cached {
		// get cached certificate data
		// read cached config from fs.
		// process the config with fromReader() use processed config as cachedConfig to update the cert data.
		unproccessedCachedConfig, err := readFromCache(cloudCfg.ID)
		if err == nil {
			cachedConfig, err := processConfigFromCloud(unproccessedCachedConfig)
			if err != nil {
				return nil, nil, err
			}

			if cachedConfig.Cloud != nil {
				tlsCertificate = cachedConfig.Cloud.TLSCertificate
				tlsPrivateKey = cachedConfig.Cloud.TLSPrivateKey
			}
		} else if !os.IsNotExist(err) {
			return nil, nil, err
		}
	}

	if checkForNewCert || tlsCertificate == "" || tlsPrivateKey == "" {
		logger.Debug("reading tlsCertificate from the cloud")
		// Use the SignalingInsecure from the Cloud config returned from the app not the initial config.
		certData, err := readCertificateDataFromCloud(ctx, cfg.Cloud.SignalingInsecure, cloudCfg)
		if err != nil {
			var urlErr *url.Error
			if !errors.Is(err, context.DeadlineExceeded) && (!errors.As(err, &urlErr) || urlErr.Temporary()) {
				return nil, nil, err
			}
			if tlsCertificate == "" || tlsPrivateKey == "" {
				return nil, nil, errors.Wrap(err, "error getting certificate data from cloud; try again later")
			}
			logger.Warnw("failed to refresh certificate data; using cached for now", "error", err)
		} else {
			tlsCertificate = certData.TLSCertificate
			tlsPrivateKey = certData.TLSPrivateKey
		}
	}

	fqdn := cfg.Cloud.FQDN
	localFQDN := cfg.Cloud.LocalFQDN
	signalingAddress := cfg.Cloud.SignalingAddress
	signalingInsecure := cfg.Cloud.SignalingInsecure
	managedBy := cfg.Cloud.ManagedBy
	locationSecret := cfg.Cloud.LocationSecret

	mergeCloudConfig := func(to *Config) {
		*to.Cloud = *cloudCfg
		to.Cloud.FQDN = fqdn
		to.Cloud.LocalFQDN = localFQDN
		to.Cloud.SignalingAddress = signalingAddress
		to.Cloud.SignalingInsecure = signalingInsecure
		to.Cloud.ManagedBy = managedBy
		to.Cloud.LocationSecret = locationSecret
		to.Cloud.TLSCertificate = tlsCertificate
		to.Cloud.TLSPrivateKey = tlsPrivateKey
	}

	mergeCloudConfig(cfg)
	mergeCloudConfig(unprocessedConfig)

	if err := storeToCache(cloudCfg.ID, unprocessedConfig); err != nil {
		golog.Global.Errorw("failed to cache config", "error", err)
	}

	return cfg, unprocessedConfig, nil
}

// Read reads a config from the given file.
func Read(
	ctx context.Context,
	filePath string,
	logger golog.Logger,
) (*Config, error) {
	buf, err := envsubst.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	return FromReader(ctx, filePath, bytes.NewReader(buf), logger)
}

// FromReader reads a config from the given reader and specifies
// where, if applicable, the file the reader originated from.
func FromReader(
	ctx context.Context,
	originalPath string,
	r io.Reader,
	logger golog.Logger,
) (*Config, error) {
	// First read and processes config from disk
	unprocessedConfig := Config{
		ConfigFilePath: originalPath,
	}
	err := json.NewDecoder(r).Decode(&unprocessedConfig)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to decode Config from json")
	}
	cfgFromDisk, err := processConfigLocalConfig(&unprocessedConfig)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to process Config")
	}

	if cfgFromDisk.Cloud != nil {
		cfg, _, err := readFromCloud(ctx, cfgFromDisk, true /* shouldReadFromCache */, true /* checkForCert*/, logger)
		return cfg, err
	}

	return cfgFromDisk, err
}

// processConfigFromCloud returns a copy of the current config with all attributes parsed
// and config validated with the assumption the config came from the cloud.
// Returns an error if the unprocessedConfig is non-valid.
func processConfigFromCloud(unprocessedConfig *Config) (*Config, error) {
	return processConfig(unprocessedConfig, true)
}

// processConfigLocalConfig returns a copy of the current config with all attributes parsed
// and config validated with the assumption the config came from a local file.
// Returns an error if the unprocessedConfig is non-valid.
func processConfigLocalConfig(unprocessedConfig *Config) (*Config, error) {
	return processConfig(unprocessedConfig, false)
}

func processConfig(unprocessedConfig *Config, fromCloud bool) (*Config, error) {
	if err := unprocessedConfig.Ensure(fromCloud); err != nil {
		return nil, err
	}

	cfg, err := unprocessedConfig.Copy()
	if err != nil {
		return nil, errors.Wrap(err, "error copying config")
	}

	for idx, c := range cfg.Components {
		conv := findMapConverter(c.Type, c.Model)
		// inner attributes may have their own converters
		for k, v := range c.Attributes {
			attrConv := findConverter(c.Type, c.Model, k)
			if attrConv == nil {
				continue
			}

			n, err := attrConv(v)
			if err != nil {
				return nil, errors.Wrapf(err, "error converting attribute for (%s, %s, %s)", c.Type, c.Model, k)
			}
			cfg.Components[idx].Attributes[k] = n
		}
		if conv == nil {
			continue
		}

		converted, err := conv(c.Attributes)
		if err != nil {
			return nil, errors.Wrapf(err, "error converting attributes for (%s, %s)", c.Type, c.Model)
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
			return nil, errors.Wrapf(err, "error converting attributes for %s", c.Type)
		}
		cfg.Services[idx].Attributes = nil
		cfg.Services[idx].ConvertedAttributes = converted
	}

	if err := cfg.Ensure(fromCloud); err != nil {
		return nil, err
	}

	return cfg, nil
}
