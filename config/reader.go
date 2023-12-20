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
	"runtime"

	"github.com/a8m/envsubst"
	"github.com/pkg/errors"
	apppb "go.viam.com/api/app/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/artifact"
	"go.viam.com/utils/rpc"
	"golang.org/x/sys/cpu"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	rutils "go.viam.com/rdk/utils"
)

// RDK versioning variables which are replaced by LD flags.
var (
	Version     = ""
	GitRevision = ""
)

func getAgentInfo() (*apppb.AgentInfo, error) {
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
		Host:        hostname,
		Ips:         ips,
		Os:          runtime.GOOS,
		Version:     Version,
		GitRevision: GitRevision,
		Platform:    &platform,
	}, nil
}

var (
	// ViamDotDir is the directory for Viam's cached files.
	ViamDotDir      string
	viamPackagesDir string
)

func init() {
	//nolint:errcheck
	home, _ := os.UserHomeDir()
	ViamDotDir = filepath.Join(home, ".viam")
	viamPackagesDir = filepath.Join(ViamDotDir, "packages")
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

func storeToCache(id string, cfg *Config) error {
	if err := os.MkdirAll(ViamDotDir, 0o700); err != nil {
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
	logger logging.Logger,
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
	logger logging.Logger,
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

// FromReader reads a config from the given reader and specifies
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

// processConfig processes the config passed in. The config can be either JSON or gRPC derived.
func processConfig(unprocessedConfig *Config, // Dan: We call this an `unprocessedConfig` but the
	// object used here is not annotated with the json field names. There's some conversion that
	// happens. I think `processConfig` is central enough where we should note anything exceptional
	// that happens between the raw json config input that everyone must be familiar with to work on
	// this code and what actually gets passed in here.
	fromCloud bool, logger logging.Logger) (*Config, error) {

	// Dan: Does `Ensure` have any side-effects? It can be interpreted as "validating" which implies
	// no side-effects, or it can also mean substituting in some defaults. If this has side-effects,
	// what would be the consequence of not ensuring?

	// Ensure validates the config but also substitutes in some defaults. Implicit dependencies for builtin resource
	// models will not be filled in until attributes are converted.
	if err := unprocessedConfig.Ensure(fromCloud, logger); err != nil {
		return nil, err
	}

	// Dan: The name mostly makes sense (maybe "CreateCopyWithOnlyPublicFields"), but why? If I were to
	// be adding fields to a `Config` how would I decide whether the field should be public or
	// private? Also, notably -- all fields on `Config` are* public. Private fields only exist
	// inside of members with types such as `Module`.

	// we cached the unprocessed config, so make a copy before changing it too much. Also ensures validation
	// happens again on resources, remotes, and modules.
	cfg, err := unprocessedConfig.CopyOnlyPublicFields()
	if err != nil {
		return nil, errors.Wrap(err, "error copying config")
	}

	// Dan: I think this is our template-y ${} substition. Given we've already parsed out keys and values
	// into their types, I expect this can modify any value of type `string`? Can users define
	// placeholder variables? Or is there a predefined set of viam substitutions? If the latter,
	// this comment should direct the reader to how they can find that list in the source code.

	// replacement can happen in resource attributes and in the module config. look at config/placeholder_replace.go
	// for available substitution types.
	if err := cfg.ReplacePlaceholders(); err != nil {
		logger.Errorw("error during placeholder replacement", "err", err)
	}

	// Dan: Three remarks. 1) Not sure what the consequence is of not having this? I think the
	// comment is saying that copying the config leaves an empty `ConfigFilePath` member in the
	// copy? 2) I see `CopyOnlyPublicFields` is only called by this method and some place in the
	// cloud watcher code. Could we have this assignment happen in `CopyOnlyPublicFields` and move
	// the complexity of "but `ConfigFilePath` is special" into the less complex cloud watcher code?
	// 3) Is it important this is done after `ReplacePlaceholders`?
	//

	// Copy does not preserve ConfigFilePath and we need to pass it along manually
	// ConfigFilePath needs to be preserved so the correct config watcher can be instantiated later in
	// the flow. We could move this into CopyOnlyPublicFields since this is a public field.
	// It is not important AFAIK
	// we do need this here because ConfigFilePath is not JSON-exported.
	cfg.ConfigFilePath = unprocessedConfig.ConfigFilePath

	// Dan: Huh? I know we run things that aren't necessarily part of the config. I guess this is where we add them. This code should declare its intent a bit better. Specifically, I would expect these three for-loops to be written as:
	//  for name in defaultServices:
	//    if name not in cfg.Services:
	//      cfg.Services.Append(thing)
	//
	// The multiple for-loops and temporary variables makes me feel:
	// - This code generates a different output than my above example
	// - `unconfiguredDefaultServices` is used further down in the function in some way.
	//
	// I've diffed in some scoping to also better help the communicate the lines of code covered by the existing comment.

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

	// Dan: Associations need to be introduced here. And the datastructure needs to be explained
	// such that a reader can predict the intended state after processing resources. Alternatively,
	// this can construct this data structure closer to where its consumed and maybe avoid some
	// complexity due to the increased lifetime of this variable.

	// We keep track of resource configs per API to facilitate linking resource configs to
	// its associated resource configs. Associated resource configs are configs that are
	// linked to and used by a different resource config.
	// can limit to APIs with Associated Config linkers registered
	resCfgsPerAPI := map[resource.API][]*resource.Config{}

	processResources := func(confs []resource.Config) error {
		for idx, conf := range confs {
			// Dan: ??. `conf` _is_ a "copy" of the element inside `confs`. If this is needed it
			// should be justified in documentation.

			// not sure about how needed this is.
			copied := conf

			// for resource to resource associations
			resCfgsPerAPI[copied.API] = append(resCfgsPerAPI[copied.API], &confs[idx])
			resName := copied.ResourceName()

			// Look up if a resource model registered an attribute map converter. Attribute conversion converts an untyped, JSON-like object to a typed
			// Go struct. There are no default converters, a converter will be automatically registered if the resource model registers a config struct alongside
			// its resource constructor.
			// Lookup will fail for non-builtin models (so lookup will fail for modular resources) but conversion will happen on the module-side.
			reg, ok := resource.LookupRegistration(resName.API, copied.Model)
			// Dan: ConvertedAttributes needs to be introduced here. Why are the `conf.Attributes`
			// not good enough? Is there a default `AttributeMapConverter`? When is an
			// `AttributeMapConverter` nil? If there are custom `AttributeMapConverter` what
			// properties do they rely on with respect to the `conf.Attributes` input?
			if !ok || reg.AttributeMapConverter == nil {
				continue
			}

			// if conversion errors, the robot will not learn of the new config until it is corrected.

			// conversion has to happen otherwise we can't populate implicit dependencies.
			converted, err := reg.AttributeMapConverter(conf.Attributes)
			if err != nil {
				// Dan: I assume this means the user has a bad config. We should document that here
				// and that a robot will not learn of the updated config until its been corrected.

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
			// Dan: Same remark as `ConvertedAttributes`. Is there a default converter? Can custom
			// ones be supplied?

			// there is no default converter for associated config converters. custom ones can be supplied through registering it on the API level.
			conv, ok := resource.LookupAssociatedConfigRegistration(associatedConf.API)
			if !ok {
				continue
			}

			// Dan: I'm see that `resource.Config` objects have an `Attributes` and
			// `ConvertedAttributes` members. `resource.Config` also has an
			// `AssociatedResourceConfigs` member which also contain an `Attributes` and
			// `ConvertedAttributes` (different type declaration, but presumably the same
			// thing?). What's the relationship here?

			// The relationship is that both Attributes are not structured, and we convert the attributes into
			// a Go struct through the use of converters and stick it in ConvertedAttributes.
			var convertedAttrs interface{} = associatedConf.Attributes
			if conv.AttributeMapConverter != nil {
				// Dan: What properties to do `AttributeMapConverter`s require of their input
				// `associatedConf.Attributes`?

				// associatedConf.Attributes is expected to be JSON-like.
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
				convertedAttrs = converted
			}

			// Dan: ??. I think I've lost context by now that I was in a bigger for-loop over
			// `associatedCfgs`. So we're doing some pairwise operation. The high-level algorithm
			// should be described in words outside the for-loop. And the reader should be reminded
			// where this part of the algorithm fits into the high level bit.

			// for APIs with an associated config linker, link the current associated config with
			// each resource config of that API.
			for _, assocConf := range resCfgsPerAPI[associatedConf.API] {
				reg, ok := resource.LookupRegistration(associatedConf.API, assocConf.Model)
				if !ok || reg.AssociatedConfigLinker == nil {
					continue
				}

				// Dan: ??. Any documentation that can tie what an `assocConf.ConvertedAttributes`
				// is in the raw json config and `convertedAttrs` would be useful here.

				// link the converted attributes for the current resource config (convertedAttrs) to the config that accepts
				// associated configs.
				if err := reg.AssociatedConfigLinker(assocConf.ConvertedAttributes, convertedAttrs); err != nil {
					return errors.Wrapf(err, "error associating resource association config to resource %q", assocConf.Model)
				}
			}
		}
		return nil
	}

	processAssociations := func(confs []resource.Config) error {
		for _, conf := range confs {
			copied := conf
			resName := copied.ResourceName()

			// Dan: Where do `AssociatedResourceConfigs` come from? User-written json or typically
			// part of auto-injected default services such as data capture?

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

	// Dan: How are remotes similar to components/services? What associations do we put on them?

	// associations can be put on resources on remotes
	for _, c := range cfg.Remotes {
		if err := convertAndAssociateResourceConfigs(nil, &c.Name, c.AssociatedResourceConfigs); err != nil {
			return nil, errors.Wrapf(err, "error processing associated service configs for remote %q", c.Name)
		}
	}

	// Dan: Again? Are we not confident the mutations we've made are legal? If this is defensive,
	// that should be called out. If there's a reason this is expected to happen, that should be
	// called out.

	// get implicit dependencies for builtin resource models
	if err := cfg.Ensure(fromCloud, logger); err != nil {
		return nil, err
	}

	return cfg, nil
}

// getFromCloudOrCache returns the config from the gRPC endpoint. If failures during cloud lookup fallback to the
// local cache if the error indicates it should.
func getFromCloudOrCache(ctx context.Context, cloudCfg *Cloud, shouldReadFromCache bool, logger logging.Logger) (*Config, bool, error) {
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
func getFromCloudGRPC(ctx context.Context, cloudCfg *Cloud, logger logging.Logger) (*Config, bool, error) {
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

	return rpc.DialDirectGRPC(ctx, u.Host, logger.AsZap(), dialOpts...)
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

	return rpc.DialDirectGRPC(ctx, u.Host, logger.AsZap(), dialOpts...)
}
