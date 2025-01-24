package config

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"github.com/a8m/envsubst"
	"github.com/pkg/errors"
	apppb "go.viam.com/api/app/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"golang.org/x/sys/cpu"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	rutils "go.viam.com/rdk/utils"
)

// RDK versioning variables which are replaced by LD flags.
var (
	Version      = ""
	GitRevision  = ""
	DateCompiled = ""
)

const (
	initialReadTimeout     = 1 * time.Second
	readTimeout            = 5 * time.Second
	readTimeoutBehindProxy = time.Minute
	// PackagesDirName is where packages go underneath viamDotDir.
	PackagesDirName = "packages"
	// LocalPackagesSuffix is used by the local package manager.
	LocalPackagesSuffix = "-local"
)

func getAgentInfo(logger logging.Logger) (*apppb.AgentInfo, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	ips, err := utils.GetAllLocalIPv4s()
	if err != nil {
		return nil, err
	}

	arch := runtime.GOARCH
	// "arm" is used for arm32. "arm64" is used for versions after v7
	if arch == "arm" {
		// armv7 added LPAE (Large Page Address Extension).
		// this is an official way to detect armv7
		// https://go-review.googlesource.com/c/go/+/525637/2/src/internal/cpu/cpu_arm.go#36
		if cpu.ARM.HasLPAE {
			arch = "arm32v7"
		} else {
			// fallback to armv6
			arch = "arm32v6"
		}
	}

	platform := fmt.Sprintf("%s/%s", runtime.GOOS, arch)

	return &apppb.AgentInfo{
		Host:         hostname,
		Ips:          ips,
		Os:           runtime.GOOS,
		Version:      Version,
		GitRevision:  GitRevision,
		Platform:     &platform,
		PlatformTags: readExtendedPlatformTags(logger, true),
	}, nil
}

var (
	// ViamDotDir is the directory for Viam's cached files.
	ViamDotDir      string
	viamPackagesDir string
)

func init() {
	home := rutils.PlatformHomeDir()
	ViamDotDir = filepath.Join(home, ".viam")
	viamPackagesDir = filepath.Join(ViamDotDir, PackagesDirName)
}

func getCloudCacheFilePath(id string) string {
	return filepath.Join(ViamDotDir, fmt.Sprintf("cached_cloud_config_%s.json", id))
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

func clearCache(id string) {
	utils.UncheckedErrorFunc(func() error {
		return os.Remove(getCloudCacheFilePath(id))
	})
}

func readCertificateDataFromCloudGRPC(ctx context.Context,
	cloudConfigFromDisk *Cloud,
	logger logging.Logger,
) (tlsConfig, error) {
	conn, err := CreateNewGRPCClient(ctx, cloudConfigFromDisk, logger)
	if err != nil {
		return tlsConfig{}, err
	}
	defer utils.UncheckedErrorFunc(conn.Close)

	service := apppb.NewRobotServiceClient(conn)
	res, err := service.Certificate(ctx, &apppb.CertificateRequest{Id: cloudConfigFromDisk.ID})
	if err != nil {
		// Check cache?
		return tlsConfig{}, err
	}
	if res.TlsCertificate == "" {
		return tlsConfig{}, errors.New("no TLS certificate yet from cloud; try again later")
	}
	if res.TlsPrivateKey == "" {
		return tlsConfig{}, errors.New("no TLS private key yet from cloud; try again later")
	}

	return tlsConfig{
		certificate: res.TlsCertificate,
		privateKey:  res.TlsPrivateKey,
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

func getTimeoutCtx(ctx context.Context, shouldReadFromCache bool, id string) (context.Context, func()) {
	timeout := readTimeout
	// When environment indicates we are behind a proxy, bump timeout. Network
	// operations tend to take longer when behind a proxy.
	if proxyAddr := os.Getenv(rpc.SocksProxyEnvVar); proxyAddr != "" {
		timeout = readTimeoutBehindProxy
	}

	// use shouldReadFromCache to determine whether this is part of initial read or not, but only shorten timeout
	// if cached config exists
	cachedConfigExists := false
	if _, err := os.Stat(getCloudCacheFilePath(id)); err == nil {
		cachedConfigExists = true
	}
	if shouldReadFromCache && cachedConfigExists {
		timeout = initialReadTimeout
	}
	return context.WithTimeout(ctx, timeout)
}

// readFromCloud fetches a robot config from the cloud based
// on the given config.
func readFromCloud(
	ctx context.Context,
	originalCfg,
	prevCfg *Config,
	shouldReadFromCache bool,
	checkForNewCert bool,
	logger logging.Logger,
) (*Config, error) {
	logger.Debug("reading configuration from the cloud")
	cloudCfg := originalCfg.Cloud
	unprocessedConfig, cached, err := getFromCloudOrCache(ctx, cloudCfg, shouldReadFromCache, logger)
	if err != nil {
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

	tls := tlsConfig{
		// both fields are empty if not cached, since its a separate request, which we
		// check next
		certificate: cfg.Cloud.TLSCertificate,
		privateKey:  cfg.Cloud.TLSPrivateKey,
	}

	if !cached {
		// Try to get TLS information from the cached config (if it exists) even if we
		// got a new config from the cloud.
		if err := tls.readFromCache(cloudCfg.ID, logger); err != nil {
			return nil, err
		}
	}

	if prevCfg != nil && shouldCheckForCert(prevCfg.Cloud, cfg.Cloud) {
		checkForNewCert = true
	}

	// It is expected to have empty certificate and private key if we are using insecure signaling
	// Use the SignalingInsecure from the Cloud config returned from the app not the initial config.
	if !cfg.Cloud.SignalingInsecure && (checkForNewCert || tls.certificate == "" || tls.privateKey == "") {
		logger.Debug("reading tlsCertificate from the cloud")

		ctxWithTimeout, cancel := getTimeoutCtx(ctx, shouldReadFromCache, cloudCfg.ID)
		certData, err := readCertificateDataFromCloudGRPC(ctxWithTimeout, cloudCfg, logger)
		if err != nil {
			cancel()
			if !errors.As(err, &context.DeadlineExceeded) {
				return nil, err
			}
			logger.Warnw("failed to refresh certificate data; using cached for now", "error", err)
		} else {
			tls = certData
			cancel()
		}
	}

	fqdn := cfg.Cloud.FQDN
	localFQDN := cfg.Cloud.LocalFQDN
	signalingAddress := cfg.Cloud.SignalingAddress
	signalingInsecure := cfg.Cloud.SignalingInsecure
	managedBy := cfg.Cloud.ManagedBy
	locationSecret := cfg.Cloud.LocationSecret
	locationSecrets := cfg.Cloud.LocationSecrets
	primaryOrgID := cfg.Cloud.PrimaryOrgID
	locationID := cfg.Cloud.LocationID
	machineID := cfg.Cloud.MachineID

	// This resets the new config's Cloud section to the original we loaded from file,
	// but allows several fields to be updated, and merges the TLS certs which come
	// from a different endpoint.
	mergeCloudConfig := func(to *Config) {
		*to.Cloud = *cloudCfg
		to.Cloud.FQDN = fqdn
		to.Cloud.LocalFQDN = localFQDN
		to.Cloud.SignalingAddress = signalingAddress
		to.Cloud.SignalingInsecure = signalingInsecure
		to.Cloud.ManagedBy = managedBy
		to.Cloud.LocationSecret = locationSecret
		to.Cloud.LocationSecrets = locationSecrets
		to.Cloud.TLSCertificate = tls.certificate
		to.Cloud.TLSPrivateKey = tls.privateKey
		to.Cloud.PrimaryOrgID = primaryOrgID
		to.Cloud.LocationID = locationID
		to.Cloud.MachineID = machineID
	}

	mergeCloudConfig(cfg)
	unprocessedConfig.Cloud.TLSCertificate = tls.certificate
	unprocessedConfig.Cloud.TLSPrivateKey = tls.privateKey

	if err := cfg.SetToCache(unprocessedConfig); err != nil {
		logger.Errorw("failed to set toCache on config", "error", err)
	}
	return cfg, nil
}

type tlsConfig struct {
	certificate string
	privateKey  string
}

func (tls *tlsConfig) readFromCache(id string, logger logging.Logger) error {
	cachedCfg, err := readFromCache(id)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		logger.Warn("No cached config, using cloud TLS config.")
	case err != nil:
		return err
	case cachedCfg.Cloud == nil:
		logger.Warn("Cached config is not a cloud config, using cloud TLS config.")
	default:
		// In secure signaling mode, we need to ensure the cache is populated with a valid TLS entry
		// however, empty TLS creds are allowed when we have insecure signaling
		if !cachedCfg.Cloud.SignalingInsecure {
			if err := cachedCfg.Cloud.ValidateTLS("cloud"); err != nil {
				logger.Warn("Detected failure to process the cached config when retrieving TLS config, clearing cache.")
				clearCache(id)
				return err
			}
		}

		tls.certificate = cachedCfg.Cloud.TLSCertificate
		tls.privateKey = cachedCfg.Cloud.TLSPrivateKey
	}
	return nil
}

// Read reads a config from the given file.
func Read(
	ctx context.Context,
	filePath string,
	logger logging.Logger,
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
	logger logging.Logger,
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
	logger logging.Logger,
) (*Config, error) {
	return fromReader(ctx, originalPath, r, logger, true)
}

// fromReader reads a config from the given reader and specifies
// where, if applicable, the file the reader originated from.
func fromReader(
	ctx context.Context,
	originalPath string,
	r io.Reader,
	logger logging.Logger,
	shouldReadFromCloud bool,
) (*Config, error) {
	// First read and process config from disk
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

// ProcessLocal validates the current config assuming it came from a local file and
// updates it with all derived fields. Returns an error if the unprocessedConfig is
// non-valid.
func (c *Config) ProcessLocal(logger logging.Logger) error {
	processed, err := processConfigLocalConfig(c, logger)
	if err != nil {
		return err
	}
	*c = *processed
	return nil
}

// processConfigFromCloud returns a copy of the current config with all attributes parsed
// and config validated with the assumption the config came from the cloud.
// Returns an error if the unprocessedConfig is non-valid.
func processConfigFromCloud(unprocessedConfig *Config, logger logging.Logger) (*Config, error) {
	return processConfig(unprocessedConfig, true, logger)
}

// processConfigLocalConfig returns a copy of the current config with all attributes parsed
// and config validated with the assumption the config came from a local file.
// Returns an error if the unprocessedConfig is non-valid.
func processConfigLocalConfig(unprocessedConfig *Config, logger logging.Logger) (*Config, error) {
	return processConfig(unprocessedConfig, false, logger)
}

// additionalModuleEnvVars will get additional environment variables for modules using other parts of the config.
func additionalModuleEnvVars(cloud *Cloud, auth AuthConfig) map[string]string {
	env := make(map[string]string)
	if cloud != nil {
		env[rutils.PrimaryOrgIDEnvVar] = cloud.PrimaryOrgID
		env[rutils.LocationIDEnvVar] = cloud.LocationID
		env[rutils.MachineIDEnvVar] = cloud.MachineID
		env[rutils.MachinePartIDEnvVar] = cloud.ID
	}
	for _, handler := range auth.Handlers {
		if handler.Type != rpc.CredentialsTypeAPIKey {
			continue
		}
		apiKeys := ParseAPIKeys(handler)
		if len(apiKeys) == 0 {
			continue
		}
		// the keys come in unsorted, so sort the keys so we'll always get the same API key
		// if there are no changes
		keyIDs := make([]string, 0, len(apiKeys))
		for k := range apiKeys {
			keyIDs = append(keyIDs, k)
		}
		sort.Strings(keyIDs)
		env[rutils.APIKeyIDEnvVar] = keyIDs[0]
		env[rutils.APIKeyEnvVar] = apiKeys[keyIDs[0]]
	}
	return env
}

// processConfig processes the config passed in. The config can be either JSON or gRPC derived.
// If any part of this function errors, the function will exit and no part of the new config will be returned
// until it is corrected.
func processConfig(unprocessedConfig *Config, fromCloud bool, logger logging.Logger) (*Config, error) {
	// Ensure validates the config but also substitutes in some defaults. Implicit dependencies for builtin resource
	// models are not filled in until attributes are converted.
	if err := unprocessedConfig.Ensure(fromCloud, logger); err != nil {
		return nil, err
	}

	// The unprocessed config is cached, so make a deep copy before continuing. By caching a relatively
	// unchanged config, changes to the way RDK processes configs between versions will not cause a cache to be broken.
	// Also ensures validation happens again on resources, remotes, and modules since the cached validation fields are not public.
	cfg, err := unprocessedConfig.CopyOnlyPublicFields()
	if err != nil {
		return nil, errors.Wrap(err, "error copying config")
	}

	// Copy does not preserve ConfigFilePath since it preserves only JSON-exported fields and so we need
	// to pass it along manually. ConfigFilePath needs to be preserved so the correct config watcher can
	// be instantiated later in the flow.
	cfg.ConfigFilePath = unprocessedConfig.ConfigFilePath

	// replacement can happen in resource attributes and in the module config. look at config/placeholder_replace.go
	// for available substitution types.
	if err := cfg.ReplacePlaceholders(); err != nil {
		logger.Errorw("error during placeholder replacement", "err", err)
	}

	// See if default service already exists in the config and add them in if not. This code allows for default services to be
	// defined under a name other than "builtin".
	defaultServices := resource.DefaultServices()
	unconfiguredDefaultServices := make(map[resource.API]resource.Name, len(defaultServices))
	for _, name := range defaultServices {
		unconfiguredDefaultServices[name.API] = name
	}
	for _, c := range cfg.Services {
		delete(unconfiguredDefaultServices, c.API)
	}

	for _, defaultServiceName := range unconfiguredDefaultServices {
		cfg.Services = append(cfg.Services, resource.Config{
			Name:  defaultServiceName.Name,
			Model: resource.DefaultServiceModel,
			API:   defaultServiceName.API,
		})
	}

	// We keep track of resource configs per API to facilitate linking resource configs to
	// its associated resource configs. Associated resource configs are configs that are
	// linked to and used by a different resource config. See the data manager
	// service for an example of a resource that uses associated resource configs.
	resCfgsPerAPI := map[resource.API][]*resource.Config{}

	processResources := func(confs []resource.Config) error {
		for idx, conf := range confs {
			copied := conf

			// for resource to resource associations
			resCfgsPerAPI[copied.API] = append(resCfgsPerAPI[copied.API], &confs[idx])
			resName := copied.ResourceName()

			// Look up if a resource model registered an attribute map converter. Attribute conversion converts
			// an untyped, JSON-like object to a typed Go struct. There is a default converter if no
			// AttributeMapConverter is registered during resource model registration. Lookup will fail for
			// non-builtin models (so lookup will fail for modular resources) but conversion will happen on the module-side.
			reg, ok := resource.LookupRegistration(resName.API, copied.Model)
			if !ok || reg.AttributeMapConverter == nil {
				continue
			}

			converted, err := reg.AttributeMapConverter(conf.Attributes)
			if err != nil {
				// if any of the conversion errors, the function will exit and no part of the new config will be returned
				// until it is corrected.
				return errors.Wrapf(err, "error converting attributes for (%s, %s)", resName.API, copied.Model)
			}
			confs[idx].ConvertedAttributes = converted
		}
		return nil
	}

	if err := processResources(cfg.Components); err != nil {
		return nil, err
	}
	if err := processResources(cfg.Services); err != nil {
		return nil, err
	}

	// Look through all associated configs for a resource config and link it to the configs that each associated config is linked to
	convertAndAssociateResourceConfigs := func(
		resName *resource.Name,
		remoteName *string,
		associatedCfgs []resource.AssociatedResourceConfig,
	) error {
		for subIdx, associatedConf := range associatedCfgs {
			// there is no default converter for associated config converters. custom ones can be supplied through registering it on the API level.
			conv, ok := resource.LookupAssociatedConfigRegistration(associatedConf.API)
			if !ok {
				continue
			}

			if conv.AttributeMapConverter != nil {
				converted, err := conv.AttributeMapConverter(associatedConf.Attributes)
				if err != nil {
					return errors.Wrap(err, "error converting associated resource config attributes")
				}
				// associated resource configs for local resources might be missing the resource name,
				// which can be inferred from its resource config.
				// associated resource configs for remote resources might be missing the remote name for the resource,
				// which can be inferred from its remote config.
				converted.UpdateResourceNames(func(oldName resource.Name) resource.Name {
					newName := oldName
					if resName != nil {
						newName = *resName
					}
					if remoteName != nil {
						newName = newName.PrependRemote(*remoteName)
					}
					return newName
				})
				associatedCfgs[subIdx].ConvertedAttributes = converted

				// for APIs with an associated config linker, link the current associated config with each resource config of that API.
				for _, assocConf := range resCfgsPerAPI[associatedConf.API] {
					converted.Link(assocConf)
				}
			}
		}
		return nil
	}

	processAssociations := func(confs []resource.Config) error {
		for _, conf := range confs {
			copied := conf
			resName := copied.ResourceName()

			// convert and associate user-written associated resource configs here.
			if err := convertAndAssociateResourceConfigs(&resName, nil, conf.AssociatedResourceConfigs); err != nil {
				return errors.Wrapf(err, "error processing associated service configs for %q", resName)
			}
		}
		return nil
	}

	if err := processAssociations(cfg.Components); err != nil {
		return nil, err
	}
	if err := processAssociations(cfg.Services); err != nil {
		return nil, err
	}

	// associated configs can be put on resources in remotes as well, so check remote configs
	for _, c := range cfg.Remotes {
		if err := convertAndAssociateResourceConfigs(nil, &c.Name, c.AssociatedResourceConfigs); err != nil {
			return nil, errors.Wrapf(err, "error processing associated service configs for remote %q", c.Name)
		}
	}

	// add additional environment vars to modules
	// adding them here ensures that if the parsed API key changes, the module will be restarted with the updated environment.
	env := additionalModuleEnvVars(cfg.Cloud, cfg.Auth)
	if len(env) > 0 {
		for _, m := range cfg.Modules {
			m.MergeEnvVars(env)
		}
	}

	// now that the attribute maps are converted, validate configs and get implicit dependencies for builtin resource models
	if err := cfg.Ensure(fromCloud, logger); err != nil {
		return nil, err
	}

	return cfg, nil
}

// getFromCloudOrCache returns the config from the gRPC endpoint. If failures during cloud lookup fallback to the
// local cache if the error indicates it should.
func getFromCloudOrCache(ctx context.Context, cloudCfg *Cloud, shouldReadFromCache bool, logger logging.Logger) (*Config, bool, error) {
	var cached bool

	ctxWithTimeout, cancel := getTimeoutCtx(ctx, shouldReadFromCache, cloudCfg.ID)
	defer cancel()

	cfg, errorShouldCheckCache, err := getFromCloudGRPC(ctxWithTimeout, cloudCfg, logger)
	if err != nil {
		if shouldReadFromCache && errorShouldCheckCache {
			cachedConfig, cacheErr := readFromCache(cloudCfg.ID)
			if cacheErr != nil {
				if os.IsNotExist(cacheErr) {
					// Return original http error if failed to load from cache.
					return nil, cached, errors.Wrap(
						err,
						"error getting cloud config, cached config does not exist; returning error from cloud config attempt",
					)
				}
				// return cache err
				return nil, cached, errors.Wrap(cacheErr, "error reading cache after getting cloud config failed")
			}

			lastUpdated := "unknown"
			if fInfo, err := os.Stat(getCloudCacheFilePath(cloudCfg.ID)); err == nil {
				// Use logging.DefaultTimeFormatStr since this time will be logged.
				lastUpdated = fInfo.ModTime().Format(logging.DefaultTimeFormatStr)
			}
			logger.Warnw("unable to get cloud config; using cached version", "config last updated", lastUpdated, "error", err)
			cached = true
			return cachedConfig, cached, nil
		}

		return nil, cached, errors.Wrap(err, "error getting cloud config")
	}

	return cfg, cached, nil
}

// getFromCloudGRPC actually does the fetching of the robot config from the gRPC endpoint.
func getFromCloudGRPC(ctx context.Context, cloudCfg *Cloud, logger logging.Logger) (*Config, bool, error) {
	shouldCheckCacheOnFailure := true

	conn, err := CreateNewGRPCClient(ctx, cloudCfg, logger)
	if err != nil {
		return nil, shouldCheckCacheOnFailure, errors.WithMessage(err, "error creating cloud grpc client")
	}
	defer utils.UncheckedErrorFunc(conn.Close)

	agentInfo, err := getAgentInfo(logger)
	if err != nil {
		return nil, shouldCheckCacheOnFailure, errors.WithMessage(err, "error getting agent info")
	}

	service := apppb.NewRobotServiceClient(conn)
	res, err := service.Config(ctx, &apppb.ConfigRequest{Id: cloudCfg.ID, AgentInfo: agentInfo})
	if err != nil {
		// Check cache?
		return nil, shouldCheckCacheOnFailure, errors.WithMessage(err, "error getting config from config endpoint")
	}
	cfg, err := FromProto(res.Config, logger)
	if err != nil {
		// Check cache?
		return nil, shouldCheckCacheOnFailure, errors.WithMessage(err, "error converting config from proto")
	}

	return cfg, false, nil
}

// CreateNewGRPCClient creates a new grpc cloud configured to communicate with the robot service based on the cloud config given.
func CreateNewGRPCClient(ctx context.Context, cloudCfg *Cloud, logger logging.Logger) (rpc.ClientConn, error) {
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

	return rpc.DialDirectGRPC(ctx, u.Host, logger.Sublogger("networking"), dialOpts...)
}

// CreateNewGRPCClientWithAPIKey creates a new grpc cloud configured to communicate with the robot service
// based on the cloud config and API key given.
func CreateNewGRPCClientWithAPIKey(ctx context.Context, cloudCfg *Cloud,
	apiKey, apiKeyID string, logger logging.Logger,
) (rpc.ClientConn, error) {
	u, err := url.Parse(cloudCfg.AppAddress)
	if err != nil {
		return nil, err
	}

	dialOpts := make([]rpc.DialOption, 0, 2)

	dialOpts = append(dialOpts, rpc.WithEntityCredentials(apiKeyID,
		rpc.Credentials{
			Type:    rpc.CredentialsTypeAPIKey,
			Payload: apiKey,
		},
	))

	if u.Scheme == "http" {
		dialOpts = append(dialOpts, rpc.WithInsecure())
	}

	return rpc.DialDirectGRPC(ctx, u.Host, logger.Sublogger("networking"), dialOpts...)
}
