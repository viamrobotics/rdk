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
	"github.com/edaniels/golog"
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

func getAgentInfo() (*apppb.AgentInfo, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	ips, err := utils.GetAllLocalIPv4s()
	if err != nil {
		return nil, err
	}
	platform := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)

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

	if err := cfg.ReplacePlaceholders(); err != nil {
		logger.Errorw("error during placeholder replacement", "err", err)
	}

	// Copy does not presve ConfigFilePath and we need to pass it along manually
	cfg.ConfigFilePath = unprocessedConfig.ConfigFilePath

	// See if default service already exists in the config
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

	// for assocations
	resCfgsPerAPI := map[resource.API][]*resource.Config{}

	processResources := func(confs []resource.Config) error {
		for idx, conf := range confs {
			copied := conf

			// for resource to resource assocations
			resCfgsPerAPI[copied.API] = append(resCfgsPerAPI[copied.API], &confs[idx])
			resName := copied.ResourceName()

			reg, ok := resource.LookupRegistration(resName.API, copied.Model)
			if !ok || reg.AttributeMapConverter == nil {
				continue
			}

			converted, err := reg.AttributeMapConverter(conf.Attributes)
			if err != nil {
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

	convertAndAssociateResourceConfigs := func(
		resName *resource.Name,
		remoteName *string,
		associatedCfgs []resource.AssociatedResourceConfig,
	) error {
		for subIdx, associatedConf := range associatedCfgs {
			conv, ok := resource.LookupAssociatedConfigRegistration(associatedConf.API)
			if !ok {
				continue
			}

			var convertedAttrs interface{} = associatedConf.Attributes
			if conv.AttributeMapConverter != nil {
				converted, err := conv.AttributeMapConverter(associatedConf.Attributes)
				if err != nil {
					return errors.Wrap(err, "error converting associated resource config attributes")
				}
				if resName != nil || remoteName != nil {
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
				}
				associatedCfgs[subIdx].Attributes = nil
				associatedCfgs[subIdx].ConvertedAttributes = converted
				convertedAttrs = converted
			}

			// always associate
			for _, assocConf := range resCfgsPerAPI[associatedConf.API] {
				reg, ok := resource.LookupRegistration(associatedConf.API, assocConf.Model)
				if !ok || reg.AssociatedConfigLinker == nil {
					continue
				}
				if err := reg.AssociatedConfigLinker(assocConf.ConvertedAttributes, convertedAttrs); err != nil {
					return errors.Wrapf(err, "error associating resource association config to resource %q", assocConf.Model)
				}
			}
		}
		return nil
	}

	processAssocations := func(confs []resource.Config) error {
		for _, conf := range confs {
			copied := conf
			resName := copied.ResourceName()

			if err := convertAndAssociateResourceConfigs(&resName, nil, conf.AssociatedResourceConfigs); err != nil {
				return errors.Wrapf(err, "error processing associated service configs for %q", resName)
			}
		}
		return nil
	}

	if err := processAssocations(cfg.Components); err != nil {
		return nil, err
	}
	if err := processAssocations(cfg.Services); err != nil {
		return nil, err
	}

	for _, c := range cfg.Remotes {
		if err := convertAndAssociateResourceConfigs(nil, &c.Name, c.AssociatedResourceConfigs); err != nil {
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

// CreateNewGRPCClientWithAPIKey creates a new grpc cloud configured to communicate with the robot service
// based on the cloud config and API key given.
func CreateNewGRPCClientWithAPIKey(ctx context.Context, cloudCfg *Cloud,
	apiKey, apiKeyID string, logger golog.Logger,
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

	return rpc.DialDirectGRPC(ctx, u.Host, logger, dialOpts...)
}
