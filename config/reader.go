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

	"github.com/a8m/envsubst"
	"github.com/pkg/errors"
	apppb "go.viam.com/api/app/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"golang.org/x/sys/cpu"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	rutils "go.viam.com/rdk/utils"
	"go.viam.com/rdk/utils/contextutils"
)

// RDK versioning variables which are replaced by LD flags.
var (
	Version      = ""
	GitRevision  = ""
	DateCompiled = ""
)

const (
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

// DefaultPackagesDir is the directory used to store packages locally: ~/.viam/packages.
// It is read fresh from [rutils.ViamDotDir] on each call so tests can redirect it.
func DefaultPackagesDir() string {
	return filepath.Join(rutils.ViamDotDir, PackagesDirName)
}

func getCloudCacheFilePath(id string) string {
	return filepath.Join(rutils.ViamDotDir, fmt.Sprintf("cached_cloud_config_%s.json", id))
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
		if runtime.GOOS == "windows" {
			utils.UncheckedErrorFunc(r.Close)
		}
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
	machineID string,
	conn rpc.ClientConn,
) (tlsConfig, error) {
	service := apppb.NewRobotServiceClient(conn)
	res, err := service.Certificate(ctx, &apppb.CertificateRequest{Id: machineID})
	if err != nil {
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
	diffFQDN := prevCloud.FQDN != cloud.FQDN
	diffLocalFQDN := prevCloud.LocalFQDN != cloud.LocalFQDN
	diffSignalingAddr := prevCloud.SignalingAddress != cloud.SignalingAddress
	diffSignalInsecure := prevCloud.SignalingInsecure != cloud.SignalingInsecure
	diffManagedBy := prevCloud.ManagedBy != cloud.ManagedBy
	diffLocSecret := prevCloud.LocationSecret != cloud.LocationSecret || !isLocationSecretsEqual(prevCloud, cloud)
	// certs are scoped to a location, so a location (or owning org) change means a new cert.
	diffLocID := prevCloud.LocationID != cloud.LocationID
	diffOrgID := prevCloud.PrimaryOrgID != cloud.PrimaryOrgID

	return diffFQDN || diffLocalFQDN || diffSignalingAddr ||
		diffSignalInsecure || diffManagedBy || diffLocSecret || diffLocID || diffOrgID
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

// cloudReadResult is what one read of the cloud config endpoint produces. The two configs are
// carried together in a struct rather than as a pair of *Config parameters so they cannot be
// silently transposed at a call site.
type cloudReadResult struct {
	// processed is the config the robot runs on.
	processed *Config
	// unprocessed is the same config before local processing; it is what gets written to the
	// cache, so that changes to how RDK processes configs do not invalidate an existing cache.
	unprocessed *Config
	// cached is true when the config came from the on-disk cache rather than the cloud.
	cached bool
}

// fetchAndProcessCloudConfig pulls the config for machineID and processes it.
func fetchAndProcessCloudConfig(
	ctx context.Context,
	machineID string,
	shouldReadFromCache bool,
	logger logging.Logger,
	conn rpc.ClientConn,
) (cloudReadResult, error) {
	unprocessedConfig, cached, err := getFromCloudOrCache(ctx, machineID, shouldReadFromCache, logger, conn)
	if err != nil {
		return cloudReadResult{}, err
	}

	processedCfg, err := processConfigFromCloud(unprocessedConfig, logger)
	if err != nil {
		if cached {
			// We could not process the cached config; clear it so we don't keep reusing a bad cache.
			logger.Warn("Detected failure to process the cached config, clearing cache.")
			clearCache(machineID)
			return cloudReadResult{}, err
		}
		// A freshly fetched cloud config we cannot process locally is malformed. The robot cannot
		// apply it, and it will fail identically on every refresh, so surface it loudly.
		return cloudReadResult{}, malformedConfigError{err}
	}
	if processedCfg.Cloud == nil {
		return cloudReadResult{}, errors.New("expected config to have cloud section")
	}

	return cloudReadResult{processed: processedCfg, unprocessed: unprocessedConfig, cached: cached}, nil
}

// applyCloudConfig finishes a cloud read: it stamps the resolved TLS cert, restores the local-only
// cloud fields, and stages the cache write.
func applyCloudConfig(
	res cloudReadResult,
	tls tlsConfig,
	localCloudCfg *Cloud,
	logger logging.Logger,
) {
	res.processed.Cloud.TLSCertificate = tls.certificate
	res.processed.Cloud.TLSPrivateKey = tls.privateKey

	// Set the cert data on the unprocessed config too, so it is saved as part of the cached config.
	res.unprocessed.Cloud.TLSCertificate = tls.certificate
	res.unprocessed.Cloud.TLSPrivateKey = tls.privateKey

	res.processed.Cloud.restoreLocalOnlyFields(localCloudCfg)

	// Never stage a cache write for a machine that needs a cert but has none. On the next boot a
	// cached config that fails ValidateTLS makes tlsConfig.readFromCache clear the whole cache,
	// taking the offline-boot fallback with it; and a machine that does boot from it comes up
	// serving plaintext, since weboptions.FromConfig skips its TLS and bind-address setup when
	// TLSCertificate is empty. Both callers error out before reaching here, so this is a backstop.
	if !res.processed.Cloud.SignalingInsecure && (tls.certificate == "" || tls.privateKey == "") {
		logger.Error("refusing to cache a cloud config that has no TLS certificate")
		return
	}

	if err := res.processed.SetToCache(res.unprocessed); err != nil {
		logger.Errorw("failed to set toCache on config", "error", err)
	}
}

// firstReadFromCloud fetches a robot config from the cloud on server startup, given the cloud
// section of the config read from disk. Unlike readFromCloud, it may fall back to the on-disk
// cache -- on startup there is nothing in memory to fall back to, and a machine that boots offline
// still needs to come up.
func firstReadFromCloud(
	ctx context.Context,
	originalCloudCfg *Cloud,
	logger logging.Logger,
	conn rpc.ClientConn,
) (*Config, error) {
	logger.Debug("reading first configuration from the cloud")
	machineID := originalCloudCfg.ID

	res, err := fetchAndProcessCloudConfig(ctx, machineID, true, logger, conn)
	if err != nil {
		return nil, err
	}

	var tls tlsConfig
	if res.processed.Cloud.SignalingInsecure {
		// No cert is expected. A cached config may still carry one, so pass it through rather than
		// blanking it; a cloud-sourced config never has one, and this is empty either way.
		tls = tlsFromCloud(res.processed.Cloud)
	} else {
		// Prefer the cloud's cert, but falling back to the cache here is not an error: a machine
		// that boots offline still has to come up with the cert it last had.
		logger.Debug("reading tlsCertificate from the cloud")

		ctxWithTimeout, cancel := contextutils.GetTimeoutCtx(ctx, true, machineID, logger)
		certData, certErr := readCertificateDataFromCloudGRPC(ctxWithTimeout, machineID, conn)
		cancel()
		switch {
		case certErr == nil:
			tls = certData
		case res.cached:
			// The config itself came from the cache, so its cert data is the cached cert data.
			tls = tlsFromCloud(res.processed.Cloud)
		default:
			// The config came from the cloud, so go read the cert out of the cache separately.
			if err := tls.readFromCache(machineID, logger); err != nil {
				return nil, err
			}
		}
		if certErr != nil {
			logger.Warnw("failed to fetch certificate data; using cached for now", "error", certErr)
		}

		// Signaling is secure, so this machine needs a cert. Neither the cloud nor the cache had
		// one, and coming up anyway would serve plaintext and poison the cache with an empty cert.
		// Fail loudly instead, matching readFromCloud.
		if tls.certificate == "" || tls.privateKey == "" {
			err := errors.New("no TLS certificate available from the cloud or the on-disk cache")
			if certErr != nil {
				err = errors.Wrap(certErr, err.Error())
			}
			return nil, err
		}
	}

	applyCloudConfig(res, tls, originalCloudCfg, logger)
	return res.processed, nil
}

// readFromCloud fetches a robot config from the cloud for the given machine ID. This is the
// steady-state path, driven by the config watcher.
//
// It never falls back to the on-disk cache: that cache is only written once reconfiguration
// completes, so during a slow reconfigure it still holds the previous cert, and reading it back
// would leave the machine reconfiguring forever (RSDK-11851). prevCloudCfg -- the cloud section of
// the previous successful read, held in memory -- is the only fallback.
func readFromCloud(
	ctx context.Context,
	machineID string,
	prevCloudCfg *Cloud,
	checkForNewCert bool,
	logger logging.Logger,
	conn rpc.ClientConn,
) (*Config, error) {
	if prevCloudCfg == nil {
		return nil, errors.New("expected prevCloudCfg to not be nil")
	}
	logger.Debug("reading configuration from the cloud")

	res, err := fetchAndProcessCloudConfig(ctx, machineID, false, logger, conn)
	if err != nil {
		return nil, err
	}

	// The cert always comes from the previous read, held in memory. Never the cache, and never
	// res.processed -- this path never reads the cache, so res.processed is always cloud-sourced,
	// and the cloud config endpoint does not carry cert data (see CloudConfigFromProto).
	tls := tlsFromCloud(prevCloudCfg)
	if !res.processed.Cloud.SignalingInsecure {
		hasPrevCert := tls.certificate != "" && tls.privateKey != ""

		// Beyond the periodic refresh the caller asks for, refetch if the cloud section changed in
		// a way that invalidates the cert (FQDN, location, ...), or if there is no cert to carry
		// forward -- e.g. the watcher was seeded from a config that predates the first cert fetch.
		refetch := checkForNewCert ||
			shouldCheckForCert(prevCloudCfg, res.processed.Cloud) ||
			!hasPrevCert

		if refetch {
			logger.Debug("reading tlsCertificate from the cloud")
			ctxWithTimeout, cancel := contextutils.GetTimeoutCtx(ctx, false, machineID, logger)
			certData, err := readCertificateDataFromCloudGRPC(ctxWithTimeout, machineID, conn)
			cancel()
			if err != nil {
				// A transient cert-endpoint failure should not cost us an otherwise good config,
				// so carry the previous cert forward. With no previous cert there is nothing to
				// fall back to and the server cannot come up securely.
				if !hasPrevCert {
					return nil, err
				}
				logger.Warnw("failed to refresh certificate data; using last fetched certificate for now", "error", err)
			} else {
				tls = certData
			}
		}
	}

	applyCloudConfig(res, tls, prevCloudCfg, logger)
	return res.processed, nil
}

// tlsConfig is the TLS cert data securing the server the viam-server spins up. It comes from a
// different endpoint than the config itself, so every cloud read has to decide where to source it.
// Whether a cert is needed at all is keyed off Cloud.SignalingInsecure; see the note there.
type tlsConfig struct {
	certificate string
	privateKey  string
}

// tlsFromCloud pulls the cert data already present on a cloud config.
func tlsFromCloud(cloud *Cloud) tlsConfig {
	return tlsConfig{certificate: cloud.TLSCertificate, privateKey: cloud.TLSPrivateKey}
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
	conn rpc.ClientConn,
) (*Config, error) {
	buf, err := envsubst.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	return FromReader(ctx, filePath, bytes.NewReader(buf), logger, conn)
}

// ReadLocalConfig reads a config from the given file but does not fetch any config from the remote servers.
func ReadLocalConfig(
	filePath string,
	logger logging.Logger,
) (*Config, error) {
	buf, err := envsubst.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	return fromReaderLocal(filePath, bytes.NewReader(buf), logger)
}

// FromReader reads a config from the given reader and specifies
// where, if applicable, the file the reader originated from.
func FromReader(
	ctx context.Context,
	originalPath string,
	r io.Reader,
	logger logging.Logger,
	conn rpc.ClientConn,
) (*Config, error) {
	return fromReader(ctx, originalPath, r, logger, conn)
}

// fromReaderLocal reads a config from the given reader and specifies
// where, if applicable, the file the reader originated from.
func fromReaderLocal(
	originalPath string,
	r io.Reader,
	logger logging.Logger,
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
	return cfgFromDisk, err
}

// fromReader reads a config from the given reader and specifies
// where, if applicable, the file the reader originated from.
func fromReader(
	ctx context.Context,
	originalPath string,
	r io.Reader,
	logger logging.Logger,
	conn rpc.ClientConn,
) (*Config, error) {
	// First read and process config from disk
	cfgFromDisk, err := fromReaderLocal(originalPath, r, logger)
	if err != nil {
		return nil, err
	}

	if conn != nil && cfgFromDisk.Cloud != nil {
		cfg, err := firstReadFromCloud(ctx, cfgFromDisk.Cloud, logger, conn)

		// Special case: DefaultBindAddress is set from Cloud, but user has specified a non-default BindAddress in local config.
		// Keep the BindAddress from local config, and use Cloud options for everything else.
		// Note: DefaultBindAddress "from Cloud" is actually set with a constant in rdk.
		if err == nil && !cfgFromDisk.Network.BindAddressDefaultSet {
			if cfg.Network.BindAddressDefaultSet {
				logger.CInfof(ctx, "Using cloud config, but BindAddress is specified in local config (%v) "+
					"and not cloud config (default = %v). Using local's.",
					cfgFromDisk.Network.BindAddress,
					cfg.Network.BindAddress)
				cfg.Network.BindAddress = cfgFromDisk.Network.BindAddress
				cfg.Network.BindAddressDefaultSet = false
			} else {
				logger.CInfof(ctx, "Using cloud config, and BindAddress specified in both cloud config (%v) "+
					"and local config (%v). Using cloud's. Remove BindAddress from cloud config to use local's.",
					cfg.Network.BindAddress,
					cfgFromDisk.Network.BindAddress)
			}
		}
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
func additionalModuleEnvVars(cloud *Cloud, auth AuthConfig, tracing TracingConfig) map[string]string {
	env := make(map[string]string)
	if cloud != nil {
		env[rutils.PrimaryOrgIDEnvVar] = cloud.PrimaryOrgID
		env[rutils.LocationIDEnvVar] = cloud.LocationID
		env[rutils.MachineFQDNEnvVar] = cloud.FQDN
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
	if tracing.IsEnabled() {
		env[rutils.ViamModuleTracingEnvVar] = "1"
	}
	return env
}

// ProcessLocalConfigForTesting invokes processConfig with fromCloud: false. To be used
// for testing that is not in this package but needs the side effects of processConfig.
func ProcessLocalConfigForTesting(unprocessedConfig *Config, logger logging.Logger) (*Config, error) {
	return processConfig(unprocessedConfig, false, logger)
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
				converted.UpdateResourceNames(func(oldName resource.Name) resource.Name {
					newName := oldName
					if resName != nil {
						newName = *resName
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
			if err := convertAndAssociateResourceConfigs(&resName, conf.AssociatedResourceConfigs); err != nil {
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
		if err := convertAndAssociateResourceConfigs(nil, c.AssociatedResourceConfigs); err != nil {
			return nil, errors.Wrapf(err, "error processing associated service configs for remote %q", c.Name)
		}
	}

	// add additional environment vars to modules
	// adding them here ensures that if the parsed API key changes, the module will be restarted with the updated environment.
	env := additionalModuleEnvVars(cfg.Cloud, cfg.Auth, cfg.Tracing)
	if len(env) > 0 {
		for idx := 0; idx < len(cfg.Modules); idx++ {
			cfg.Modules[idx].MergeEnvVars(env)
		}
	}

	// now that the attribute maps are converted, validate configs and get implicit dependencies for builtin resource models
	if err := cfg.Ensure(fromCloud, logger); err != nil {
		return nil, err
	}

	return cfg, nil
}

// malformedConfigError wraps an error indicating this robot's cloud config could not be applied, as
// opposed to a transient failure to reach the cloud. This happens either because the cloud served a
// config this robot could not decode from proto or process locally, or because the cloud responded with
// a status code indicating it could not produce a usable config for this robot. A malformed config fails
// identically on every refresh, so surface it loudly rather than retrying quietly.
type malformedConfigError struct {
	err error
}

func (e malformedConfigError) Error() string {
	return fmt.Sprintf("config was malformed: %s", e.err.Error())
}

func (e malformedConfigError) Unwrap() error { return e.err }

// IsMalformedConfigError reports whether err indicates this robot's cloud config could not be applied
// because it is malformed.
func IsMalformedConfigError(err error) bool {
	var malformedErr malformedConfigError
	return errors.As(err, &malformedErr)
}

// isCloudConfigMalformed reports whether a cloud config-fetch error means the cloud responded and
// could not return a usable config for this robot, as opposed to a transient failure to reach the cloud.
func isCloudConfigMalformed(err error) bool {
	st, ok := status.FromError(err)
	if !ok {
		return false
	}
	code := st.Code()
	return code == codes.Unknown || code == codes.InvalidArgument || code == codes.FailedPrecondition
}

// getFromCloudOrCache returns the config from the gRPC endpoint. If failures during cloud lookup fallback to the
// local cache if shouldReadFromCache is set.
func getFromCloudOrCache(
	ctx context.Context,
	machineID string,
	shouldReadFromCache bool,
	logger logging.Logger,
	conn rpc.ClientConn,
) (*Config, bool, error) {
	var cached bool
	ctxWithTimeout, cancel := contextutils.GetTimeoutCtx(ctx, shouldReadFromCache, machineID, logger)
	defer cancel()

	cfg, err := getFromCloudGRPC(ctxWithTimeout, machineID, logger, conn)
	if err != nil {
		malformed := IsMalformedConfigError(err)
		if shouldReadFromCache {
			cachedConfig, cacheErr := readFromCache(machineID)
			if cacheErr != nil {
				if os.IsNotExist(cacheErr) {
					// No cache to fall back to, return original error.
					return nil, cached, errors.Wrap(
						err,
						"error getting cloud config, cached config does not exist; returning error from cloud config attempt",
					)
				}
				// return cache err
				return nil, cached, errors.Wrap(cacheErr, "error reading cache after getting cloud config failed")
			}

			lastUpdated := "unknown"
			if fInfo, err := os.Stat(getCloudCacheFilePath(machineID)); err == nil {
				// Use logging.DefaultTimeFormatStr since this time will be logged.
				lastUpdated = fInfo.ModTime().Format(logging.DefaultTimeFormatStr)
			}
			// A malformed config is logged at Error since it will keep failing,
			// while a transient failure to reach the cloud stays at Warn. Same
			// message either way.
			logFunc := logger.Warnw
			if malformed {
				logFunc = logger.Errorw
			}
			logFunc("could not apply new cloud config; using cached version",
				"config last updated", lastUpdated, "error", err)
			cached = true
			return cachedConfig, cached, nil
		}

		return nil, cached, errors.Wrap(err, "error getting cloud config")
	}

	return cfg, cached, nil
}

// getFromCloudGRPC actually does the fetching of the robot config from the gRPC endpoint. A failure to
// reach the cloud is returned as a plain error; a config the cloud could not produce or that we cannot
// decode is returned as a malformedConfigError.
func getFromCloudGRPC(
	ctx context.Context,
	machineID string,
	logger logging.Logger,
	conn rpc.ClientConn,
) (*Config, error) {
	agentInfo, err := getAgentInfo(logger)
	if err != nil {
		return nil, errors.WithMessage(err, "error getting agent info")
	}

	service := apppb.NewRobotServiceClient(conn)
	res, err := service.Config(ctx, &apppb.ConfigRequest{Id: machineID, AgentInfo: agentInfo})
	if err != nil {
		// A status code indicating the cloud could not produce a usable config is treated as malformed.
		// Anything else (connectivity, timeout, auth, rate-limiting, etc) is transient.
		malformed := isCloudConfigMalformed(err)
		err = errors.WithMessage(err, "error getting config from config endpoint")
		if malformed {
			return nil, malformedConfigError{err}
		}
		return nil, err
	}
	cfg, err := FromProto(res.Config, logger)
	if err != nil {
		// The cloud served a config we could not decode from proto, so it is malformed.
		return nil, malformedConfigError{errors.WithMessage(err, "error converting config from proto")}
	}

	return cfg, nil
}

// CreateNewGRPCClient creates a new grpc cloud configured to communicate with the robot service based on the cloud config given.
func CreateNewGRPCClient(ctx context.Context, cloudCfg *Cloud, logger logging.Logger) (rpc.ClientConn, error) {
	u, err := url.Parse(cloudCfg.AppAddress)
	if err != nil {
		return nil, err
	}

	dialOpts := make([]rpc.DialOption, 0, 2)

	cloudCreds := cloudCfg.GetCloudCredsDialOpt()

	// Only add credentials when they are set.
	if cloudCreds != nil {
		dialOpts = append(dialOpts, cloudCreds)
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
