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
	"go.uber.org/multierr"
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

// CreateCloudCertificateRequest makes a request to fetch the robot's TLS
// certificate from a cloud endpoint.
func CreateCloudCertificateRequest(ctx context.Context, cloudCfg *Cloud) (*http.Request, error) {
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

func openFromCache(id string) (io.ReadCloser, error) {
	return os.Open(getCloudCacheFilePath(id))
}

func deleteCachedConfig(id string) error {
	return os.Remove(getCloudCacheFilePath(id))
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

// readFromCloud fetches a robot config from the cloud based
// on the given config.
func readFromCloud(
	ctx context.Context,
	cloudCfg *Cloud,
	readFromCache bool,
	checkForNewCert bool,
	logger golog.Logger,
) (*Config, *Config, error) {
	cloudReq, err := CreateCloudRequest(ctx, cloudCfg)
	if err != nil {
		return nil, nil, err
	}

	var client http.Client
	defer client.CloseIdleConnections()
	resp, err := client.Do(cloudReq)

	var configReader io.ReadCloser
	var cached bool
	if err == nil {
		if resp.StatusCode != http.StatusOK {
			defer func() {
				utils.UncheckedError(resp.Body.Close())
			}()
			rd, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return nil, nil, err
			}
			if len(rd) != 0 {
				return nil, nil, errors.Errorf("unexpected status %d: %s", resp.StatusCode, string(rd))
			}
			return nil, nil, errors.Errorf("unexpected status %d", resp.StatusCode)
		}
		configReader = resp.Body
	} else {
		if !readFromCache {
			return nil, nil, err
		}
		var urlErr *url.Error
		if !errors.Is(err, context.DeadlineExceeded) && (!errors.As(err, &urlErr) || urlErr.Temporary()) {
			return nil, nil, err
		}
		var cacheErr error
		configReader, cacheErr = openFromCache(cloudCfg.ID)
		if cacheErr != nil {
			if os.IsNotExist(cacheErr) {
				return nil, nil, err
			}
			return nil, nil, cacheErr
		}
		cached = true
		logger.Warnw("unable to get cloud config; using cached version", "error", err)
	}
	defer utils.UncheckedErrorFunc(configReader.Close)

	// read the actual config and do not make a cloud request again to avoid
	// infinite recursion.
	cfg, unprocessedConfig, err := fromReader(ctx, "", configReader, true, logger)

	var parsingErr *parsingError
	switch {
	case cached && errors.As(err, &parsingErr):
		if deleteErr := deleteCachedConfig(cloudCfg.ID); deleteErr != nil {
			return nil, nil, multierr.Combine(deleteErr, err)
		} else {
			logger.Warnw("deleted unparseable cached config", "error", err)
			return nil, nil, err
		}
	case err != nil:
		return nil, nil, err
	case cfg.Cloud == nil:
		return nil, nil, errors.New("expected config to have cloud section")
	}

	// empty if not cached, since its a separate request, which we check next
	tlsCertificate := cfg.Cloud.TLSCertificate
	tlsPrivateKey := cfg.Cloud.TLSPrivateKey
	if !cached {
		// get cached certificate data
		cachedConfigReader, err := openFromCache(cloudCfg.ID)
		if err == nil {
			cachedConfig, _, err := fromReader(ctx, "", cachedConfigReader, true, logger)

			switch {
			case errors.As(err, &parsingErr):
				if deleteErr := deleteCachedConfig(cloudCfg.ID); deleteErr != nil {
					return nil, nil, multierr.Combine(deleteErr, err)
				} else {
					logger.Warnw("deleted unparseable cached config", "error", err)
					return nil, nil, err
				}
			case err != nil:
				return nil, nil, err
			case cachedConfig.Cloud == nil:
				logger.Warn("expected cached config to have cloud section; need to get a new certificate")
			default:
				tlsCertificate = cachedConfig.Cloud.TLSCertificate
				tlsPrivateKey = cachedConfig.Cloud.TLSPrivateKey
			}
		} else if !os.IsNotExist(err) {
			return nil, nil, err
		}
	}

	if checkForNewCert || tlsCertificate == "" || tlsPrivateKey == "" {
		certReq, err := CreateCloudCertificateRequest(ctx, cloudCfg)
		if err != nil {
			return nil, nil, err
		}

		var client http.Client
		defer client.CloseIdleConnections()
		resp, err := client.Do(certReq)
		if err == nil {
			defer func() {
				utils.UncheckedError(resp.Body.Close())
			}()

			dec := json.NewDecoder(resp.Body)
			var certData Cloud
			if err := dec.Decode(&certData); err != nil {
				return nil, nil, errors.Wrap(err, "error decoding certificate data from cloud; try again later")
			}

			if !certData.SignalingInsecure {
				if certData.TLSCertificate == "" {
					return nil, nil, errors.New("no TLS certificate yet from cloud; try again later")
				}
				if certData.TLSPrivateKey == "" {
					return nil, nil, errors.New("no TLS private key yet from cloud; try again later")
				}

				tlsCertificate = certData.TLSCertificate
				tlsPrivateKey = certData.TLSPrivateKey
			}
		} else {
			var urlErr *url.Error
			if !errors.Is(err, context.DeadlineExceeded) && (!errors.As(err, &urlErr) || urlErr.Temporary()) {
				return nil, nil, err
			}
			if tlsCertificate == "" || tlsPrivateKey == "" {
				return nil, nil, errors.Wrap(err, "error getting certificate data from cloud; try again later")
			}
			logger.Warnw("failed to refresh certificate data; using cached for now", "error", err)
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

// ParsingError is used when a configuration file cannot be parsed.
type parsingError struct {
	wrapped error
}

func (e parsingError) Error() string {
	return e.wrapped.Error()
}

// FromReader reads a config from the given reader and specifies
// where, if applicable, the file the reader originated from.
func FromReader(
	ctx context.Context,
	originalPath string,
	r io.Reader,
	logger golog.Logger,
) (*Config, error) {
	cfg, _, err := fromReader(ctx, originalPath, r, false, logger)
	return cfg, err
}

func fromReader(
	ctx context.Context,
	originalPath string,
	r io.Reader,
	skipCloud bool,
	logger golog.Logger,
) (*Config, *Config, error) {
	cfg := Config{
		ConfigFilePath: originalPath,
	}
	unprocessedConfig := Config{
		ConfigFilePath: originalPath,
	}

	rd, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, nil, err
	}
	if err := json.Unmarshal(rd, &cfg); err != nil {
		return nil, nil, &parsingError{errors.Wrap(err, "cannot parse config")}
	}
	if err := json.Unmarshal(rd, &unprocessedConfig); err != nil {
		return nil, nil, &parsingError{errors.Wrap(err, "cannot parse config")}
	}
	if err := cfg.Ensure(skipCloud); err != nil {
		return nil, nil, err
	}
	if err := unprocessedConfig.Ensure(skipCloud); err != nil {
		return nil, nil, err
	}

	if !skipCloud && cfg.Cloud != nil {
		return readFromCloud(ctx, cfg.Cloud, true, true, logger)
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
				return nil, nil, errors.Wrapf(err, "error converting attribute for (%s, %s, %s)", c.Type, c.Model, k)
			}
			cfg.Components[idx].Attributes[k] = n
		}
		if conv == nil {
			continue
		}

		converted, err := conv(c.Attributes)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "error converting attributes for (%s, %s)", c.Type, c.Model)
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
			return nil, nil, errors.Wrapf(err, "error converting attributes for %s", c.Type)
		}
		cfg.Services[idx].Attributes = nil
		cfg.Services[idx].ConvertedAttributes = converted
	}

	if err := cfg.Ensure(skipCloud); err != nil {
		return nil, nil, err
	}
	return &cfg, &unprocessedConfig, nil
}
