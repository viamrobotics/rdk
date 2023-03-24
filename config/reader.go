package config

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"

	"github.com/a8m/envsubst"
	"github.com/edaniels/golog"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	apppb "go.viam.com/api/app/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/artifact"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/resource"
	rutils "go.viam.com/rdk/utils"
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
type AttributeMapConverter func(attributes rutils.AttributeMap) (interface{}, error)

// A ResourceAttributeConverterRegistration describes how to convert a specific attribute
// for a model of a type of resource.
type ResourceAttributeConverterRegistration struct {
	Subtype resource.Subtype
	Model   resource.Model
	Attr    string
	Conv    AttributeConverter
}

// A ResourceAttributeMapConverterRegistration describes how to convert all attributes
// for a model of a type of resource.
type ResourceAttributeMapConverterRegistration struct {
	Subtype resource.Subtype
	Model   resource.Model
	Conv    AttributeMapConverter
}

// A ResourceAssociationConfigConverter describes how to convert all attributes
// for a type of resource associated with another resource (e.g. data capture on a resource).
type ResourceAssociationConfigConverter struct {
	Subtype  resource.Subtype
	Conv     AttributeMapConverter
	WithName ResourceToResourceAssociationWithName
}

// A ResourceAssocationConfigAssociator describes how to associate a
// resource association config to a specific resource model (e.g. builtin data capture).
type ResourceAssocationConfigAssociator struct {
	Subtype   resource.Subtype
	Model     resource.Model
	Associate ResourceToResourceAssociator
}

type (
	// ResourceToResourceAssociationWithName allows a resource to attach a name to a  subtype specific
	// association config. This is generally done by the subtype registration.
	ResourceToResourceAssociationWithName func(resName resource.Name, resAssociation interface{}) error

	// ResourceToResourceAssociator allows one resource to associate a specific association config
	// to its own config. This is generally done by a specific resource (e.g. data capture of many components).
	ResourceToResourceAssociator func(conf *resource.Config, resAssociation interface{}) error
)

var (
	resourceAttributeConverters        = []ResourceAttributeConverterRegistration{}
	resourceAttributeMapConverters     = []ResourceAttributeMapConverterRegistration{}
	resourceAssociationConfigConverter = []ResourceAssociationConfigConverter{}
	resourceAssocationConfigAssociator = []ResourceAssocationConfigAssociator{}
)

// RegisterComponentAttributeConverter associates a component type and model with a way to convert a
// particular attribute name.
func RegisterComponentAttributeConverter(subtype resource.Subtype, model resource.Model, attr string, conv AttributeConverter) {
	RegisterResourceAttributeConverter(subtype, model, attr, conv)
}

// RegisterComponentAttributeMapConverter associates a component type and model with a way to convert all attributes.
func RegisterComponentAttributeMapConverter(
	subtype resource.Subtype,
	model resource.Model,
	conv AttributeMapConverter,
) {
	RegisterResourceAttributeMapConverter(subtype, model, conv)
}

// RegisterServiceAttributeConverter associates a service type and model with a way to convert a
// particular attribute name. It is a helper for RegisterResourceAttributeConverter.
func RegisterServiceAttributeConverter(subtype resource.Subtype, model resource.Model, attr string, conv AttributeConverter) {
	RegisterResourceAttributeConverter(subtype, model, attr, conv)
}

// RegisterServiceAttributeMapConverter associates a service type and model with a way to convert all attributes.
// It is a helper for RegisterResourceAttributeMapConverter.
func RegisterServiceAttributeMapConverter(
	subtype resource.Subtype,
	model resource.Model,
	conv AttributeMapConverter,
) {
	RegisterResourceAttributeMapConverter(subtype, model, conv)
}

// RegisterResourceAttributeConverter associates a resource (component/service) type and model with a way to
// convert a particular attribute name.
func RegisterResourceAttributeConverter(subtype resource.Subtype, model resource.Model, attr string, conv AttributeConverter) {
	resourceAttributeConverters = append(resourceAttributeConverters, ResourceAttributeConverterRegistration{subtype, model, attr, conv})
}

// RegisterResourceAttributeMapConverter associates a resource (component/service) type and model with a way to
// convert all attributes.
func RegisterResourceAttributeMapConverter(
	subtype resource.Subtype,
	model resource.Model,
	conv AttributeMapConverter,
) {
	resourceAttributeMapConverters = append(
		resourceAttributeMapConverters,
		ResourceAttributeMapConverterRegistration{subtype, model, conv},
	)
}

// TransformAttributeMapToStruct uses an attribute map to transform attributes to the prescribed format.
func TransformAttributeMapToStruct(to interface{}, attributes rutils.AttributeMap) (interface{}, error) {
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

// RegisterResourceAssociationConfigConverter registers a converter for a resource's resource association config
// to the given resource subtype that will consume it (e.g. data capture on a component). Additionally, a way
// to attach a resource name to the converted config must be supplied.
func RegisterResourceAssociationConfigConverter(
	subtype resource.Subtype,
	conv AttributeMapConverter,
	withResourceName ResourceToResourceAssociationWithName,
) {
	resourceAssociationConfigConverter = append(
		resourceAssociationConfigConverter,
		ResourceAssociationConfigConverter{subtype, conv, withResourceName},
	)
}

// RegisterResourceAssocationConfigAssociator registers a resource's association config type to a specific
// subtype model that will consume it (e.g. builtin data capture on a component).
func RegisterResourceAssocationConfigAssociator(
	subtype resource.Subtype,
	model resource.Model,
	associate ResourceToResourceAssociator,
) {
	resourceAssocationConfigAssociator = append(
		resourceAssocationConfigAssociator,
		ResourceAssocationConfigAssociator{subtype, model, associate},
	)
}

func findConverter(subtype resource.Subtype, model resource.Model, attr string) AttributeConverter {
	for _, r := range resourceAttributeConverters {
		if r.Subtype == subtype && r.Model == model && r.Attr == attr {
			return r.Conv
		}
	}
	return nil
}

// FindResourceAssociationConfigConverter finds the resource association config AttributeMapConverter for the given subtype.
func FindResourceAssociationConfigConverter(subtype resource.Subtype) (AttributeMapConverter, ResourceToResourceAssociationWithName, bool) {
	for _, r := range resourceAssociationConfigConverter {
		if r.Subtype == subtype {
			return r.Conv, r.WithName, true
		}
	}
	return nil, nil, false
}

// FindResourceAssocationConfigAssociator finds the resource association to model associator for the given subtype and model.
func FindResourceAssocationConfigAssociator(subtype resource.Subtype, model resource.Model) (ResourceToResourceAssociator, bool) {
	for _, r := range resourceAssocationConfigAssociator {
		if r.Subtype == subtype && r.Model == model {
			return r.Associate, true
		}
	}
	return nil, false
}

// FindMapConverter finds the resource AttributeMapConverter for the given subtype and model.
func FindMapConverter(subtype resource.Subtype, model resource.Model) AttributeMapConverter {
	for _, r := range resourceAttributeMapConverters {
		if r.Subtype == subtype && r.Model == model {
			return r.Conv
		}
	}
	return nil
}

func getAgentInfo() (*apppb.AgentInfo, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	ips, err := utils.GetAllLocalIPv4s()
	if err != nil {
		return nil, err
	}

	return &apppb.AgentInfo{
		Host:        hostname,
		Ips:         ips,
		Os:          runtime.GOOS,
		Version:     Version,
		GitRevision: GitRevision,
	}, nil
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
	defer utils.UncheckedErrorFunc(r.Close)

	unprocessedConfig := &Config{
		ConfigFilePath: "",
	}

	if err := json.NewDecoder(r).Decode(unprocessedConfig); err != nil {
		// clear the cache if we cannot parse the file.
		clearCache(id)
		return nil, errors.Wrap(err, "cannot parse the cached config as json")
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

func clearCache(id string) {
	utils.UncheckedErrorFunc(func() error {
		return os.Remove(getCloudCacheFilePath(id))
	})
}

func readCertificateDataFromCloudGRPC(ctx context.Context,
	signalingInsecure bool,
	cloudConfigFromDisk *Cloud,
	logger golog.Logger,
) (*Cloud, error) {
	conn, err := CreateNewGRPCClient(ctx, cloudConfigFromDisk, logger)
	if err != nil {
		return nil, err
	}
	defer utils.UncheckedErrorFunc(conn.Close)

	service := apppb.NewRobotServiceClient(conn)
	res, err := service.Certificate(ctx, &apppb.CertificateRequest{Id: cloudConfigFromDisk.ID})
	if err != nil {
		// Check cache?
		return nil, err
	}

	if !signalingInsecure {
		if res.TlsCertificate == "" {
			return nil, errors.New("no TLS certificate yet from cloud; try again later")
		}
		if res.TlsPrivateKey == "" {
			return nil, errors.New("no TLS private key yet from cloud; try again later")
		}
	}

	// TODO(RSDK-539): we might want to use an internal type here. The gRPC api will not return a Cloud json struct.
	return &Cloud{
		TLSCertificate: res.TlsCertificate,
		TLSPrivateKey:  res.TlsPrivateKey,
	}, nil
}

// shouldCheckForCert checks the Cloud config to see if the TLS cert should be refetched.
func shouldCheckForCert(prevCloud, cloud *Cloud) bool {
	// only checking the same fields as the ones that are explicitly overwritten in mergeCloudConfig
	diffFQDN := prevCloud.FQDN != cloud.FQDN
	diffLocalFQDN := prevCloud.LocalFQDN != cloud.LocalFQDN
	diffSignalingAddr := prevCloud.SignalingAddress != cloud.SignalingAddress
	diffSignalInsecure := prevCloud.SignalingInsecure != cloud.SignalingInsecure
	diffManagedBy := prevCloud.ManagedBy != cloud.ManagedBy
	diffLocSecret := prevCloud.LocationSecret != cloud.LocationSecret || !isLocationSecretsEqual(prevCloud, cloud)

	return diffFQDN || diffLocalFQDN || diffSignalingAddr || diffSignalInsecure || diffManagedBy || diffLocSecret
}

func isLocationSecretsEqual(prevCloud, cloud *Cloud) bool {
	if len(prevCloud.LocationSecrets) != len(cloud.LocationSecrets) {
		return false
	}

	for i := range cloud.LocationSecrets {
		if cloud.LocationSecrets[i].Secret != prevCloud.LocationSecrets[i].Secret {
			return false
		}

		if cloud.LocationSecrets[i].ID != prevCloud.LocationSecrets[i].ID {
			return false
		}
	}

	return true
}

// readFromCloud fetches a robot config from the cloud based
// on the given config.
func readFromCloud(
	ctx context.Context,
	originalCfg,
	prevCfg *Config,
	shouldReadFromCache bool,
	checkForNewCert bool,
	logger golog.Logger,
) (*Config, error) {
	logger.Debug("reading configuration from the cloud")
	cloudCfg := originalCfg.Cloud
	unprocessedConfig, cached, err := getFromCloudOrCache(ctx, cloudCfg, shouldReadFromCache, logger)
	if err != nil {
		if !cached {
			err = errors.Wrap(err, "error getting cloud config")
		}
		return nil, err
	}

	// process the config
	cfg, err := processConfigFromCloud(unprocessedConfig, logger)
	if err != nil {
		// If we cannot process the config from the cache we should clear it.
		if cached {
			// clear cache
			logger.Warn("Detected failure to process the cached config, clearing cache.")
			clearCache(cloudCfg.ID)
		}
		return nil, err
	}
	if cfg.Cloud == nil {
		return nil, errors.New("expected config to have cloud section")
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
			cachedConfig, err := processConfigFromCloud(unproccessedCachedConfig, logger)
			if err != nil {
				// clear cache
				logger.Warn("Detected failure to process the cached config when retrieving TLS config, clearing cache.")
				clearCache(cloudCfg.ID)
				return nil, err
			}

			if cachedConfig.Cloud != nil {
				tlsCertificate = cachedConfig.Cloud.TLSCertificate
				tlsPrivateKey = cachedConfig.Cloud.TLSPrivateKey
			}
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}

	if prevCfg != nil && shouldCheckForCert(prevCfg.Cloud, cfg.Cloud) {
		checkForNewCert = true
	}

	if checkForNewCert || tlsCertificate == "" || tlsPrivateKey == "" {
		logger.Debug("reading tlsCertificate from the cloud")
		// Use the SignalingInsecure from the Cloud config returned from the app not the initial config.

		certData, err := readCertificateDataFromCloudGRPC(ctx, cfg.Cloud.SignalingInsecure, cloudCfg, logger)
		if err != nil {
			if !errors.Is(err, context.DeadlineExceeded) {
				return nil, err
			}
			if tlsCertificate == "" || tlsPrivateKey == "" {
				return nil, errors.Wrap(err, "error getting certificate data from cloud; try again later")
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
	locationSecrets := cfg.Cloud.LocationSecrets

	mergeCloudConfig := func(to *Config) {
		*to.Cloud = *cloudCfg
		to.Cloud.FQDN = fqdn
		to.Cloud.LocalFQDN = localFQDN
		to.Cloud.SignalingAddress = signalingAddress
		to.Cloud.SignalingInsecure = signalingInsecure
		to.Cloud.ManagedBy = managedBy
		to.Cloud.LocationSecret = locationSecret
		to.Cloud.LocationSecrets = locationSecrets
		to.Cloud.TLSCertificate = tlsCertificate
		to.Cloud.TLSPrivateKey = tlsPrivateKey
	}

	mergeCloudConfig(cfg)
	// TODO(RSDK-1960): add more tests around config caching
	unprocessedConfig.Cloud.TLSCertificate = tlsCertificate
	unprocessedConfig.Cloud.TLSPrivateKey = tlsPrivateKey

	if err := storeToCache(cloudCfg.ID, unprocessedConfig); err != nil {
		logger.Errorw("failed to cache config", "error", err)
	}

	return cfg, nil
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

// ReadLocalConfig reads a config from the given file but does not fetch any config from the remote servers.
func ReadLocalConfig(
	ctx context.Context,
	filePath string,
	logger golog.Logger,
) (*Config, error) {
	buf, err := envsubst.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	return fromReader(ctx, filePath, bytes.NewReader(buf), logger, false)
}

// FromReader reads a config from the given reader and specifies
// where, if applicable, the file the reader originated from.
func FromReader(
	ctx context.Context,
	originalPath string,
	r io.Reader,
	logger golog.Logger,
) (*Config, error) {
	return fromReader(ctx, originalPath, r, logger, true)
}

// FromReader reads a config from the given reader and specifies
// where, if applicable, the file the reader originated from.
func fromReader(
	ctx context.Context,
	originalPath string,
	r io.Reader,
	logger golog.Logger,
	shouldReadFromCloud bool,
) (*Config, error) {
	// First read and processes config from disk
	unprocessedConfig := Config{
		ConfigFilePath: originalPath,
	}
	err := json.NewDecoder(r).Decode(&unprocessedConfig)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to decode Config from json")
	}
	cfgFromDisk, err := processConfigLocalConfig(&unprocessedConfig, logger)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to process Config")
	}

	if shouldReadFromCloud && cfgFromDisk.Cloud != nil {
		cfg, err := readFromCloud(ctx, cfgFromDisk, nil, true, true, logger)
		return cfg, err
	}

	return cfgFromDisk, err
}

// processConfigFromCloud returns a copy of the current config with all attributes parsed
// and config validated with the assumption the config came from the cloud.
// Returns an error if the unprocessedConfig is non-valid.
func processConfigFromCloud(unprocessedConfig *Config, logger golog.Logger) (*Config, error) {
	return processConfig(unprocessedConfig, true, logger)
}

// processConfigLocalConfig returns a copy of the current config with all attributes parsed
// and config validated with the assumption the config came from a local file.
// Returns an error if the unprocessedConfig is non-valid.
func processConfigLocalConfig(unprocessedConfig *Config, logger golog.Logger) (*Config, error) {
	return processConfig(unprocessedConfig, false, logger)
}

func processConfig(unprocessedConfig *Config, fromCloud bool, logger golog.Logger) (*Config, error) {
	if err := unprocessedConfig.Ensure(fromCloud, logger); err != nil {
		return nil, err
	}

	cfg, err := unprocessedConfig.CopyOnlyPublicFields()
	if err != nil {
		return nil, errors.Wrap(err, "error copying config")
	}

	// Copy does not presve ConfigFilePath and we need to pass it along manually
	cfg.ConfigFilePath = unprocessedConfig.ConfigFilePath

	// See if default service already exists in the config
	unconfiguredDefaultServices := make(map[resource.Subtype]resource.Name, len(resource.DefaultServices))
	for _, name := range resource.DefaultServices {
		unconfiguredDefaultServices[name.Subtype] = name
	}
	for _, c := range cfg.Services {
		copied := c
		// TODO(PRODUCT-266): Remove when API replaces namespace/subtype; otherwise this fixes it up
		if _, err := copied.Validate("", resource.ResourceTypeService); err != nil {
			return nil, errors.Wrapf(err, "error validating service")
		}
		delete(unconfiguredDefaultServices, copied.API)
	}

	for _, defaultServiceName := range unconfiguredDefaultServices {
		cfg.Services = append(cfg.Services, resource.Config{
			Name:                   defaultServiceName.Name,
			Model:                  resource.DefaultServiceModel,
			DeprecatedNamespace:    defaultServiceName.Namespace,
			DeprecatedSubtype:      defaultServiceName.ResourceSubtype,
			DeprecatedResourceType: resource.ResourceTypeService,
			API:                    defaultServiceName.Subtype,
		})
	}

	// for assocations
	resCfgsPerSubtype := map[resource.Subtype][]*resource.Config{}

	processResources := func(resType resource.TypeName, confs []resource.Config) error {
		for idx, conf := range confs {
			copied := conf

			// TODO(PRODUCT-266): Remove when API replaces namespace/subtype; otherwise this fixes it up
			if _, err := copied.Validate("", resType); err != nil {
				return errors.Wrapf(err, "error validating resource")
			}
			// for resource to resource assocations
			resCfgsPerSubtype[copied.API] = append(resCfgsPerSubtype[copied.API], &confs[idx])
			resName := copied.ResourceName()

			conv := FindMapConverter(resName.Subtype, conf.Model)
			// inner attributes may have their own converters
			for k, v := range conf.Attributes {
				attrConv := findConverter(resName.Subtype, conf.Model, k)
				if attrConv == nil {
					continue
				}

				n, err := attrConv(v)
				if err != nil {
					return errors.Wrapf(err, "error converting attribute for (%s, %s, %s)", resName.Subtype, conf.Model, k)
				}
				confs[idx].Attributes[k] = n
			}
			if conv == nil {
				continue
			}

			converted, err := conv(conf.Attributes)
			if err != nil {
				return errors.Wrapf(err, "error converting attributes for (%s, %s)", resName.Subtype, conf.Model)
			}
			confs[idx].Attributes = nil
			confs[idx].ConvertedAttributes = converted
		}
		return nil
	}

	if err := processResources(resource.ResourceTypeComponent, cfg.Components); err != nil {
		return nil, err
	}
	if err := processResources(resource.ResourceTypeService, cfg.Services); err != nil {
		return nil, err
	}

	convertAndAssociateResourceConfigs := func(resName *resource.Name, associatedCfgs []resource.AssociatedResourceConfig) error {
		for subIdx, associatedConf := range associatedCfgs {
			assocSubtype := associatedConf.AssociatedSubtype()
			conv, attachName, ok := FindResourceAssociationConfigConverter(assocSubtype)
			if !ok {
				continue
			}

			var converted interface{} = associatedConf.Attributes
			if conv != nil {
				var err error
				converted, err = conv(associatedConf.Attributes)
				if err != nil {
					return errors.Wrap(err, "error converting associated resource config attributes")
				}
				associatedCfgs[subIdx].Attributes = nil
				associatedCfgs[subIdx].ConvertedAttributes = converted
			}

			if resName != nil {
				if err := attachName(*resName, converted); err != nil {
					return errors.Wrap(err, "error attaching resource name to associated resource config")
				}
			} // otherwise we assume the resource name is already in the associated config

			// always associate
			for _, assocConf := range resCfgsPerSubtype[assocSubtype] {
				associate, ok := FindResourceAssocationConfigAssociator(assocSubtype, assocConf.Model)
				if !ok {
					continue
				}
				if err := associate(assocConf, converted); err != nil {
					return errors.Wrapf(err, "error associating resource association config to resource %q", assocConf.Model)
				}
			}
		}
		return nil
	}

	processAssocations := func(resType resource.TypeName, confs []resource.Config) error {
		for _, conf := range confs {
			copied := conf
			// TODO(PRODUCT-266): Remove when API replaces namespace/subtype; otherwise this fixes it up
			if _, err := copied.Validate("", resType); err != nil {
				return errors.Wrapf(err, "error validating resource")
			}
			resName := copied.ResourceName()

			if err := convertAndAssociateResourceConfigs(&resName, conf.AssociatedResourceConfigs); err != nil {
				return errors.Wrapf(err, "error processing associated service configs for %q", resName)
			}
		}
		return nil
	}

	if err := processAssocations(resource.ResourceTypeComponent, cfg.Components); err != nil {
		return nil, err
	}
	if err := processAssocations(resource.ResourceTypeService, cfg.Services); err != nil {
		return nil, err
	}

	for _, c := range cfg.Remotes {
		if err := convertAndAssociateResourceConfigs(nil, c.AssociatedResourceConfigs); err != nil {
			return nil, errors.Wrapf(err, "error processing associated service configs for remote %q", c.Name)
		}
	}

	if err := cfg.Ensure(fromCloud, logger); err != nil {
		return nil, err
	}

	return cfg, nil
}

// getFromCloudOrCache returns the config from the gRPC endpoint. If failures during cloud lookup fallback to the
// local cache if the error indicates it should.
func getFromCloudOrCache(ctx context.Context, cloudCfg *Cloud, shouldReadFromCache bool, logger golog.Logger) (*Config, bool, error) {
	var cached bool
	cfg, errorShouldCheckCache, err := getFromCloudGRPC(ctx, cloudCfg, logger)
	if err != nil {
		if shouldReadFromCache && errorShouldCheckCache {
			logger.Warnw("failed to read config from cloud, checking cache", "error", err)
			cachedConfig, cacheErr := readFromCache(cloudCfg.ID)
			if cacheErr != nil {
				if os.IsNotExist(cacheErr) {
					// Return original http error if failed to load from cache.
					return nil, cached, err
				}
				// return cache err
				return nil, cached, cacheErr
			}
			logger.Warnw("unable to get cloud config; using cached version", "error", err)
			cached = true
			return cachedConfig, cached, nil
		}

		return nil, cached, err
	}

	return cfg, cached, nil
}

// getFromCloudGRPC actually does the fetching of the robot config from the gRPC endpoint.
func getFromCloudGRPC(ctx context.Context, cloudCfg *Cloud, logger golog.Logger) (*Config, bool, error) {
	shouldCheckCacheOnFailure := true

	conn, err := CreateNewGRPCClient(ctx, cloudCfg, logger)
	if err != nil {
		return nil, shouldCheckCacheOnFailure, err
	}
	defer utils.UncheckedErrorFunc(conn.Close)

	agentInfo, err := getAgentInfo()
	if err != nil {
		return nil, shouldCheckCacheOnFailure, err
	}

	service := apppb.NewRobotServiceClient(conn)
	res, err := service.Config(ctx, &apppb.ConfigRequest{Id: cloudCfg.ID, AgentInfo: agentInfo})
	if err != nil {
		// Check cache?
		return nil, shouldCheckCacheOnFailure, err
	}

	cfg, err := FromProto(res.Config, logger)
	if err != nil {
		// Check cache?
		return nil, shouldCheckCacheOnFailure, err
	}

	return cfg, false, nil
}

// CreateNewGRPCClient creates a new grpc cloud configured to communicate with the robot service based on the cloud config given.
func CreateNewGRPCClient(ctx context.Context, cloudCfg *Cloud, logger golog.Logger) (rpc.ClientConn, error) {
	u, err := url.Parse(cloudCfg.AppAddress)
	if err != nil {
		return nil, err
	}

	dialOpts := make([]rpc.DialOption, 0, 2)
	// Only add credentials when secret is set.
	if cloudCfg.Secret != "" {
		dialOpts = append(dialOpts, rpc.WithEntityCredentials(cloudCfg.ID,
			rpc.Credentials{
				Type:    rutils.CredentialsTypeRobotSecret,
				Payload: cloudCfg.Secret,
			},
		))
	}

	if u.Scheme == "http" {
		dialOpts = append(dialOpts, rpc.WithInsecure())
	}

	return rpc.DialDirectGRPC(ctx, u.Host, logger, dialOpts...)
}
