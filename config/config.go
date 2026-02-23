// Package config defines the structures to configure a robot and its connected parts.
package config

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/pkg/errors"
	"go.viam.com/utils/artifact"
	"go.viam.com/utils/jwks"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	rutils "go.viam.com/rdk/utils"
)

// A Config describes the configuration of a robot.
type Config struct {
	Cloud             *Cloud
	Modules           []Module
	Remotes           []Remote
	Components        []resource.Config
	Processes         []pexec.ProcessConfig
	Services          []resource.Config
	Packages          []PackageConfig
	Network           NetworkConfig
	Auth              AuthConfig
	Debug             bool
	LogConfig         []logging.LoggerPatternConfig
	MaintenanceConfig *MaintenanceConfig
	Jobs              []JobConfig
	Tracing           TracingConfig

	ConfigFilePath string

	// AllowInsecureCreds is used to have all connections allow insecure
	// downgrades and send credentials over plaintext. This is an option
	// a user must pass via command line arguments.
	AllowInsecureCreds bool

	// UntrustedEnv is used to disable Processes and shell for untrusted environments
	// where a process cannot be trusted. This is an option a user must pass via
	// command line arguments.
	UntrustedEnv bool

	// FromCommand indicates if this config was parsed via the web server command.
	// If false, it's for creating a robot via the RDK library. This is helpful for
	// error messages that can indicate flags/config fields to use.
	FromCommand bool

	// PackagePath sets the directory used to store packages locally. Defaults to ~/.viam/packages
	PackagePath string

	// EnableWebProfile turns pprof http server in localhost. Defaults to false.
	EnableWebProfile bool

	// Revision contains the current revision of the config.
	Revision string

	// Initial represents whether this is an "initial" config passed in by web
	// server entrypoint code. If true, the robot will continue to report a state
	// of initializing after applying this config. If false, the robot will
	// report a state of running after applying this config.
	Initial bool

	// DisableLogDeduplication controls whether deduplication of noisy logs
	// should be turned off. Defaults to false.
	DisableLogDeduplication bool

	// toCache stores the JSON marshalled version of the config to be cached. It should be a copy of
	// the config pulled from cloud with minor changes.
	// This version is kept because the config is changed as it moves through the system.
	toCache []byte
}

// A TracingConfig describes the tracing configuration for a robot
type TracingConfig struct {
	// Enabled globally enables or disables tracing.
	Enabled bool `json:"enabled"`

	// Disk enables saving trace data to disk in the VIAM_HOME directory. Data
	// recorded this way can later be retrieved with the viam cli.
	Disk bool `json:"disk,omitempty"`

	// Console enables printing traces to the console as they occur.
	Console bool `json:"console,omitempty"`

	// OTLPEndpoint specifies an endpoint that trace spans should be sent to
	// using OTLP over gRPC. Environment variables can be used to specify auth
	// headers or other options. See
	// https://pkg.go.dev/go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc
	// for more information.
	OTLPEndpoint string `json:"otlpendpoint,omitempty"`
}

// IsEnabled returns true if Enabled is true and at least one export location
// is enabled and false otherwise.
func (cfg TracingConfig) IsEnabled() bool {
	return cfg.Enabled && (cfg.Disk || cfg.Console || cfg.OTLPEndpoint != "")
}

// MaintenanceConfig specifies a sensor that the machine will check to determine if the machine should reconfigure.
// This Config is not validated during config processing but it will be validated during reconfiguration.
type MaintenanceConfig struct {
	SensorName            string `json:"sensor_name"`
	MaintenanceAllowedKey string `json:"maintenance_allowed_key"`
}

// NOTE: This data must be maintained with what is in [Config].
type configData struct {
	Cloud                   *Cloud                        `json:"cloud,omitempty"`
	Modules                 []Module                      `json:"modules,omitempty"`
	Remotes                 []Remote                      `json:"remotes,omitempty"`
	Components              []resource.Config             `json:"components,omitempty"`
	Processes               []pexec.ProcessConfig         `json:"processes,omitempty"`
	Services                []resource.Config             `json:"services,omitempty"`
	Packages                []PackageConfig               `json:"packages,omitempty"`
	Network                 NetworkConfig                 `json:"network"`
	Auth                    AuthConfig                    `json:"auth"`
	Debug                   bool                          `json:"debug,omitempty"`
	EnableWebProfile        bool                          `json:"enable_web_profile"`
	LogConfig               []logging.LoggerPatternConfig `json:"log,omitempty"`
	Revision                string                        `json:"revision,omitempty"`
	MaintenanceConfig       *MaintenanceConfig            `json:"maintenance,omitempty"`
	PackagePath             string                        `json:"package_path,omitempty"`
	DisableLogDeduplication bool                          `json:"disable_log_deduplication"`
	Jobs                    []JobConfig                   `json:"jobs,omitempty"`
	Tracing                 TracingConfig                 `json:"tracing,omitempty"`
}

// AppValidationStatus refers to the.
type AppValidationStatus struct {
	Error string `json:"error"`
}

// Ensure calls respective Validate methods on many subconfigs of the larger config. These
// Validate methods both cause side-effects and return validation errors. Those
// side-effects fill in default values, and, in the case of component and service configs,
// can fill in implicit dependencies. Some validation errors are "fatal" (such as
// malformed cloud, network, or auth subbconfigs), and some will only result in a logged
// error.
func (c *Config) Ensure(fromCloud bool, logger logging.Logger) error {
	if c.Cloud != nil {
		// Adds default for RefreshInterval if not set.
		if err := c.Cloud.Validate("cloud", fromCloud); err != nil {
			return err
		}
	}

	// Adds default BindAddress and HeartbeatWindow if not set.
	if err := c.Network.Validate("network"); err != nil {
		return err
	}

	// Updates ValidatedKeySet once validated.
	if err := c.Auth.Validate("auth"); err != nil {
		return err
	}

	// Validate jobs, modules, remotes, packages, and processes, and log errors for lack of
	// uniqueness within each category. Managers of each resource handle duplicates
	// differently, and behavior is undefined.
	seenJobs := make(map[string]struct{})
	for idx := range len(c.Jobs) {
		if err := c.Jobs[idx].Validate(fmt.Sprintf("%s.%d", "jobs", idx)); err != nil {
			logger.Errorw("Jobs config error; starting robot without job", "name", c.Jobs[idx].Name, "error", err.Error())
		}
		if _, exists := seenJobs[c.Jobs[idx].Name]; exists {
			logger.Errorf("Duplicate job %s in robot config; behavior undefined", c.Jobs[idx].Name)
		}
		seenJobs[c.Jobs[idx].Name] = struct{}{}
	}
	seenModules := make(map[string]struct{})
	for idx := range len(c.Modules) {
		if err := c.Modules[idx].Validate(fmt.Sprintf("%s.%d", "modules", idx)); err != nil {
			logger.Errorw("Module config error; starting robot without module", "name", c.Modules[idx].Name, "error", err.Error())
		}
		if _, exists := seenModules[c.Modules[idx].Name]; exists {
			logger.Errorf("Duplicate module %s in robot config; behavior undefined", c.Modules[idx].Name)
		}
		seenModules[c.Modules[idx].Name] = struct{}{}
	}
	seenRemotes := make(map[string]struct{})
	for idx := range len(c.Remotes) {
		if _, _, err := c.Remotes[idx].Validate(fmt.Sprintf("%s.%d", "remotes", idx)); err != nil {
			logger.Errorw("Remote config error; starting robot without remote", "name", c.Remotes[idx].Name, "error", err.Error())
		}
		if _, exists := seenRemotes[c.Remotes[idx].Name]; exists {
			logger.Errorf("Duplicate remote %s in robot config; behavior undefined", c.Remotes[idx].Name)
		}
		seenRemotes[c.Remotes[idx].Name] = struct{}{}
	}
	seenPackages := make(map[string]struct{})
	for idx := range len(c.Packages) {
		if err := c.Packages[idx].Validate(fmt.Sprintf("%s.%d", "packages", idx)); err != nil {
			logger.Errorw("Package config error; starting robot without package", "name", c.Packages[idx].Name, "error", err.Error())
		}
		if _, exists := seenPackages[c.Packages[idx].Name]; exists {
			logger.Errorf("Duplicate package %s in robot config; behavior undefined", c.Packages[idx].Name)
		}
		seenPackages[c.Packages[idx].Name] = struct{}{}
	}
	seenProcesses := make(map[string]struct{})
	for idx := range len(c.Processes) {
		if err := c.Processes[idx].Validate(fmt.Sprintf("%s.%d", "processes", idx)); err != nil {
			logger.Errorw("Process config error; starting robot without process", "name", c.Processes[idx].Name, "error", err.Error())
		}
		if _, exists := seenProcesses[c.Processes[idx].Name]; exists {
			logger.Errorf("Duplicate process %s in robot config; behavior undefined", c.Processes[idx].Name)
		}
		seenProcesses[c.Processes[idx].Name] = struct{}{}
	}

	// Validate components and services as above but also populate implicit dependencies. Do
	// not log any errors about duplicates. Duplicate components and services are caught by
	// the resource manager.
	for idx := range len(c.Components) {
		// requiredDeps and optionalDeps will only be populated if attributes have been converted, which does not happen in this function.
		// Attributes can be converted from an untyped, JSON-like object to a typed Go struct based on whether a converter/the typed struct
		// was registered during resource model registration. If no converter but a typed struct was registered, the RDK provides a
		// default converter. For modular resources, since lookup will fail as no converter or a typed struct is registered, implicit
		// dependencies are gathered during robot reconfiguration itself.
		requiredDeps, optionalDeps, err := c.Components[idx].Validate(fmt.Sprintf("%s.%d", "components", idx), resource.APITypeComponentName)
		if err != nil {
			resLogger := logger.Sublogger(c.Components[idx].ResourceName().String())
			resLogger.Errorw("Component config error; starting robot without component", "name", c.Components[idx].Name, "error", err.Error())
		} else {
			c.Components[idx].ImplicitDependsOn = requiredDeps
			c.Components[idx].ImplicitOptionalDependsOn = optionalDeps
		}
	}
	for idx := range len(c.Services) {
		requiredDeps, optionalDeps, err := c.Services[idx].Validate(fmt.Sprintf("%s.%d", "services", idx), resource.APITypeServiceName)
		if err != nil {
			resLogger := logger.Sublogger(c.Services[idx].ResourceName().String())
			resLogger.Errorw("Service config error; starting robot without service", "name", c.Services[idx].Name, "error", err.Error())
		} else {
			c.Services[idx].ImplicitDependsOn = requiredDeps
			c.Services[idx].ImplicitOptionalDependsOn = optionalDeps
		}
	}

	return nil
}

// FindComponent finds a particular component by name.
func (c Config) FindComponent(name string) *resource.Config {
	for _, cmp := range c.Components {
		if cmp.Name == name {
			return &cmp
		}
	}
	return nil
}

// SetToCache sets toCache with a marshalled copy of the config passed in.
func (c *Config) SetToCache(cfg *Config) error {
	md, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	c.toCache = md
	return nil
}

// StoreToCache caches the toCache.
func (c *Config) StoreToCache() error {
	if c.toCache == nil {
		return errors.New("no unprocessed config to cache")
	}
	if err := os.MkdirAll(rutils.ViamDotDir, 0o700); err != nil {
		return err
	}
	reader := bytes.NewReader(c.toCache)
	path := getCloudCacheFilePath(c.Cloud.ID)
	return artifact.AtomicStore(path, reader, c.Cloud.ID)
}

// UnmarshalJSON unmarshals JSON into the config and adjusts some
// names if they are not fully filled in.
func (c *Config) UnmarshalJSON(data []byte) error {
	var conf configData
	if err := json.Unmarshal(data, &conf); err != nil {
		return err
	}
	for idx := range conf.Components {
		conf.Components[idx].AdjustPartialNames(resource.APITypeComponentName)
	}
	for idx := range conf.Services {
		conf.Services[idx].AdjustPartialNames(resource.APITypeServiceName)
	}

	c.Cloud = conf.Cloud
	c.Modules = conf.Modules
	c.Remotes = conf.Remotes
	c.Components = conf.Components
	c.Processes = conf.Processes
	c.Services = conf.Services
	c.Packages = conf.Packages
	c.Network = conf.Network
	c.Auth = conf.Auth
	c.Debug = conf.Debug
	c.EnableWebProfile = conf.EnableWebProfile
	c.LogConfig = conf.LogConfig
	c.Revision = conf.Revision
	c.MaintenanceConfig = conf.MaintenanceConfig
	c.PackagePath = conf.PackagePath
	c.DisableLogDeduplication = conf.DisableLogDeduplication
	c.Jobs = conf.Jobs
	c.Tracing = conf.Tracing

	return nil
}

// MarshalJSON marshals JSON from the config.
func (c Config) MarshalJSON() ([]byte, error) {
	for idx := range c.Components {
		c.Components[idx].AdjustPartialNames(resource.APITypeComponentName)
	}
	for idx := range c.Services {
		c.Services[idx].AdjustPartialNames(resource.APITypeServiceName)
	}

	return json.Marshal(configData{
		Cloud:                   c.Cloud,
		Modules:                 c.Modules,
		Remotes:                 c.Remotes,
		Components:              c.Components,
		Processes:               c.Processes,
		Services:                c.Services,
		Packages:                c.Packages,
		Network:                 c.Network,
		Auth:                    c.Auth,
		Debug:                   c.Debug,
		EnableWebProfile:        c.EnableWebProfile,
		LogConfig:               c.LogConfig,
		Revision:                c.Revision,
		MaintenanceConfig:       c.MaintenanceConfig,
		PackagePath:             c.PackagePath,
		DisableLogDeduplication: c.DisableLogDeduplication,
		Jobs:                    c.Jobs,
		Tracing:                 c.Tracing,
	})
}

// CopyOnlyPublicFields returns a deep-copy of the current config only preserving JSON exported fields.
func (c *Config) CopyOnlyPublicFields() (*Config, error) {
	// We're using JSON as an intermediary to ensure only the json exported fields are
	// copied.
	tmpJSON, err := json.Marshal(c)
	if err != nil {
		return nil, errors.Wrap(err, "error marshaling config")
	}
	var cfg Config
	err = json.Unmarshal(tmpJSON, &cfg)
	if err != nil {
		return nil, errors.Wrap(err, "error unmarshaling config")
	}

	return &cfg, nil
}

// A Remote describes a remote robot that should be integrated.
// The Frame field defines how the "world" node of the remote robot should be reconciled with the "world" node of
// the current robot. All components of the remote robot who have Parent as "world" will be attached to the parent defined
// in Frame, and with the given offset as well.
type Remote struct {
	Name                      string
	Address                   string
	Frame                     *referenceframe.LinkConfig
	Auth                      RemoteAuth
	ManagedBy                 string
	Insecure                  bool
	ConnectionCheckInterval   time.Duration
	ReconnectInterval         time.Duration
	AssociatedResourceConfigs []resource.AssociatedResourceConfig

	// Secret is a helper for a robot location secret.
	Secret string
	Prefix string

	alreadyValidated bool
	cachedErr        error
}

// Note: keep this in sync with Remote.
type remoteData struct {
	Name                      string                              `json:"name"`
	Address                   string                              `json:"address"`
	Frame                     *referenceframe.LinkConfig          `json:"frame,omitempty"`
	Auth                      RemoteAuth                          `json:"auth"`
	ManagedBy                 string                              `json:"managed_by"`
	Insecure                  bool                                `json:"insecure"`
	ConnectionCheckInterval   string                              `json:"connection_check_interval,omitempty"`
	ReconnectInterval         string                              `json:"reconnect_interval,omitempty"`
	AssociatedResourceConfigs []resource.AssociatedResourceConfig `json:"service_configs"`

	// Secret is a helper for a robot location secret.
	Secret string `json:"secret"`
	Prefix string `json:"prefix,omitempty"`
}

// Equals checks if the two configs are deeply equal to each other.
func (conf Remote) Equals(other Remote) bool {
	conf.alreadyValidated = false
	conf.cachedErr = nil
	other.alreadyValidated = false
	other.cachedErr = nil
	//nolint:govet
	return reflect.DeepEqual(conf, other)
}

// UnmarshalJSON unmarshals JSON data into this config.
func (conf *Remote) UnmarshalJSON(data []byte) error {
	var temp remoteData
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}
	*conf = Remote{
		Name:                      temp.Name,
		Address:                   temp.Address,
		Frame:                     temp.Frame,
		Auth:                      temp.Auth,
		ManagedBy:                 temp.ManagedBy,
		Insecure:                  temp.Insecure,
		AssociatedResourceConfigs: temp.AssociatedResourceConfigs,
		Secret:                    temp.Secret,
		Prefix:                    temp.Prefix,
	}
	if temp.ConnectionCheckInterval != "" {
		dur, err := time.ParseDuration(temp.ConnectionCheckInterval)
		if err != nil {
			return err
		}
		conf.ConnectionCheckInterval = dur
	}
	if temp.ReconnectInterval != "" {
		dur, err := time.ParseDuration(temp.ReconnectInterval)
		if err != nil {
			return err
		}
		conf.ReconnectInterval = dur
	}
	return nil
}

// MarshalJSON marshals out this config.
func (conf Remote) MarshalJSON() ([]byte, error) {
	temp := remoteData{
		Name:                      conf.Name,
		Address:                   conf.Address,
		Prefix:                    conf.Prefix,
		Frame:                     conf.Frame,
		Auth:                      conf.Auth,
		ManagedBy:                 conf.ManagedBy,
		Insecure:                  conf.Insecure,
		AssociatedResourceConfigs: conf.AssociatedResourceConfigs,
		Secret:                    conf.Secret,
	}
	if conf.Prefix != "" {
		temp.Prefix = conf.Prefix
	}
	if conf.ConnectionCheckInterval != 0 {
		temp.ConnectionCheckInterval = conf.ConnectionCheckInterval.String()
	}
	if conf.ReconnectInterval != 0 {
		temp.ReconnectInterval = conf.ReconnectInterval.String()
	}
	return json.Marshal(temp)
}

// RemoteAuth specifies how to authenticate against a remote. If no credentials are
// specified, authentication does not happen. If an entity is specified, the
// authentication request will specify it.
type RemoteAuth struct {
	Credentials *rpc.Credentials `json:"credentials"`
	Entity      string           `json:"entity"`

	// only used internally right now
	ExternalAuthAddress    string           `json:"-"`
	ExternalAuthInsecure   bool             `json:"-"`
	ExternalAuthToEntity   string           `json:"-"`
	Managed                bool             `json:"-"`
	SignalingServerAddress string           `json:"-"`
	SignalingAuthEntity    string           `json:"-"`
	SignalingCreds         *rpc.Credentials `json:"-"`
}

// Validate ensures all parts of the config are valid.
func (conf *Remote) Validate(path string) ([]string, []string, error) {
	if conf.alreadyValidated {
		return nil, nil, conf.cachedErr
	}
	conf.cachedErr = conf.validate(path)
	conf.alreadyValidated = true
	return nil, nil, conf.cachedErr
}

func (conf *Remote) validate(path string) error {
	if conf.Name == "" {
		return resource.NewConfigValidationFieldRequiredError(path, "name")
	}
	if err := rutils.ValidateRemoteName(conf.Name); err != nil {
		return resource.NewConfigValidationError(path, err)
	}
	if conf.Address == "" {
		return resource.NewConfigValidationFieldRequiredError(path, "address")
	}
	if conf.Frame != nil {
		if conf.Frame.Parent == "" {
			return resource.NewConfigValidationFieldRequiredError(path, "frame.parent")
		}
	}

	if conf.Secret != "" {
		conf.Auth = RemoteAuth{
			Credentials: &rpc.Credentials{
				Type:    rutils.CredentialsTypeRobotLocationSecret,
				Payload: conf.Secret,
			},
		}
	}
	return nil
}

// A Cloud describes how to configure a robot controlled by the
// cloud.
type Cloud struct {
	ID                string
	Secret            string
	LocationSecret    string // Deprecated: Use LocationSecrets
	LocationSecrets   []LocationSecret
	APIKey            APIKey
	LocationID        string
	PrimaryOrgID      string
	MachineID         string
	ManagedBy         string
	FQDN              string
	LocalFQDN         string
	SignalingAddress  string
	SignalingInsecure bool
	AppAddress        string
	RefreshInterval   time.Duration

	// cached by us and fetched from a non-config endpoint.
	TLSCertificate string
	TLSPrivateKey  string
}

// Note: keep this in sync with Cloud.
type cloudData struct {
	// For a working cloud managed robot, these three fields have to be set
	// within the config passed to the robot through the --config argument.
	// Cloud configs are not expected to return these fields, and these fields
	// will be ignored if they are returned.
	ID         string `json:"id"`
	Secret     string `json:"secret,omitempty"`
	AppAddress string `json:"app_address,omitempty"`

	LocationSecret    string           `json:"location_secret"`
	LocationSecrets   []LocationSecret `json:"location_secrets"`
	APIKey            APIKey           `json:"api_key"`
	LocationID        string           `json:"location_id"`
	PrimaryOrgID      string           `json:"primary_org_id"`
	MachineID         string           `json:"machine_id"`
	ManagedBy         string           `json:"managed_by"`
	FQDN              string           `json:"fqdn"`
	LocalFQDN         string           `json:"local_fqdn"`
	SignalingAddress  string           `json:"signaling_address"`
	SignalingInsecure bool             `json:"signaling_insecure,omitempty"`
	RefreshInterval   string           `json:"refresh_interval,omitempty"`

	// cached by us and fetched from a non-config endpoint.
	TLSCertificate string `json:"tls_certificate"`
	TLSPrivateKey  string `json:"tls_private_key"`
}

// APIKey is the cloud app authentication credential
type APIKey struct {
	ID  string `json:"id"`
	Key string `json:"key"`
}

// IsFullySet returns true if an APIKey has both the ID and Key fields set.
func (a APIKey) IsFullySet() bool {
	return a.ID != "" && a.Key != ""
}

// IsPartiallySet returns true if only one of the ID or Key fields are set.
func (a APIKey) IsPartiallySet() bool {
	return (a.ID == "" && a.Key != "") || (a.ID != "" && a.Key == "")
}

// GetCloudCredsDialOpt returns a dial option with the cloud credentials for this cloud config.
// API keys are always preferred over robot secrets. If neither are set, nil is returned.
func (config *Cloud) GetCloudCredsDialOpt() rpc.DialOption {
	if config.APIKey.IsFullySet() {
		return rpc.WithEntityCredentials(config.APIKey.ID, rpc.Credentials{rutils.CredentialsTypeAPIKey, config.APIKey.Key})
	} else if config.Secret != "" {
		return rpc.WithEntityCredentials(config.ID, rpc.Credentials{rutils.CredentialsTypeRobotSecret, config.Secret})
	}
	return nil
}

// UnmarshalJSON unmarshals JSON data into this config.
func (config *Cloud) UnmarshalJSON(data []byte) error {
	var temp cloudData
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}
	*config = Cloud{
		ID:                temp.ID,
		Secret:            temp.Secret,
		LocationSecret:    temp.LocationSecret,
		LocationSecrets:   temp.LocationSecrets,
		APIKey:            temp.APIKey,
		LocationID:        temp.LocationID,
		PrimaryOrgID:      temp.PrimaryOrgID,
		MachineID:         temp.MachineID,
		ManagedBy:         temp.ManagedBy,
		FQDN:              temp.FQDN,
		LocalFQDN:         temp.LocalFQDN,
		SignalingAddress:  temp.SignalingAddress,
		SignalingInsecure: temp.SignalingInsecure,
		AppAddress:        temp.AppAddress,
		TLSCertificate:    temp.TLSCertificate,
		TLSPrivateKey:     temp.TLSPrivateKey,
	}
	if temp.RefreshInterval != "" {
		dur, err := time.ParseDuration(temp.RefreshInterval)
		if err != nil {
			return err
		}
		config.RefreshInterval = dur
	}
	return nil
}

// MarshalJSON marshals out this config.
func (config Cloud) MarshalJSON() ([]byte, error) {
	temp := cloudData{
		ID:                config.ID,
		Secret:            config.Secret,
		LocationSecret:    config.LocationSecret,
		LocationSecrets:   config.LocationSecrets,
		APIKey:            config.APIKey,
		LocationID:        config.LocationID,
		PrimaryOrgID:      config.PrimaryOrgID,
		MachineID:         config.MachineID,
		ManagedBy:         config.ManagedBy,
		FQDN:              config.FQDN,
		LocalFQDN:         config.LocalFQDN,
		SignalingAddress:  config.SignalingAddress,
		SignalingInsecure: config.SignalingInsecure,
		AppAddress:        config.AppAddress,
		TLSCertificate:    config.TLSCertificate,
		TLSPrivateKey:     config.TLSPrivateKey,
	}
	if config.RefreshInterval != 0 {
		temp.RefreshInterval = config.RefreshInterval.String()
	}
	return json.Marshal(temp)
}

// Validate ensures all parts of the config are valid. Adds default for RefreshInterval if not set.
func (config *Cloud) Validate(path string, fromCloud bool) error {
	if config.ID == "" {
		return resource.NewConfigValidationFieldRequiredError(path, "id")
	}
	if fromCloud {
		if config.FQDN == "" {
			return resource.NewConfigValidationFieldRequiredError(path, "fqdn")
		}
		if config.LocalFQDN == "" {
			return resource.NewConfigValidationFieldRequiredError(path, "local_fqdn")
		}
	} else if config.APIKey.IsPartiallySet() {
		return resource.NewConfigValidationFieldRequiredError(path, "api_key")
	} else if config.Secret == "" && !config.APIKey.IsFullySet() {
		return resource.NewConfigValidationFieldRequiredError(path, "api_key")
	}
	if config.RefreshInterval == 0 {
		config.RefreshInterval = 10 * time.Second
	}
	return nil
}

// ValidateTLS ensures TLS fields are valid.
func (config *Cloud) ValidateTLS(path string) error {
	if config.TLSCertificate == "" {
		return resource.NewConfigValidationFieldRequiredError(path, "tls_certificate")
	}
	if config.TLSPrivateKey == "" {
		return resource.NewConfigValidationFieldRequiredError(path, "tls_private_key")
	}
	return nil
}

// LocationSecret describes a location secret that can be used to authenticate to the rdk.
type LocationSecret struct {
	ID string `json:"id"`
	// Payload of the secret
	Secret string `json:"secret"`
}

// NetworkConfig describes networking settings for the web server.
type NetworkConfig struct {
	NetworkConfigData
}

// NetworkConfigData is the network config data that gets marshaled/unmarshaled.
type NetworkConfigData struct {
	// FQDN is the unique name of this server.
	FQDN string `json:"fqdn,omitempty"`

	// Listener is the listener that the web server will use. This is mutually
	// exclusive with BindAddress.
	Listener net.Listener `json:"-"`

	// BindAddress is the address that the web server will bind to.
	// The default behavior is to bind to localhost:8080. This is mutually
	// exclusive with Listener.
	BindAddress string `json:"bind_address,omitempty"`

	BindAddressDefaultSet bool `json:"-"`

	// TLSCertFile is used to enable secure communications on the hosted HTTP server.
	// This is mutually exclusive with TLSCertPEM and TLSKeyPEM.
	TLSCertFile string `json:"tls_cert_file,omitempty"`

	// TLSKeyFile is used to enable secure communications on the hosted HTTP server.
	// This is mutually exclusive with TLSCertPEM and TLSKeyPEM.
	TLSKeyFile string `json:"tls_key_file,omitempty"`

	// TLSConfig is used to enable secure communications on the hosted HTTP server.
	// This is mutually exclusive with TLSCertFile and TLSKeyFile.
	TLSConfig *tls.Config `json:"-"`

	// NoTLS disables the use of TLS on the hosted HTTP server.
	NoTLS bool `json:"no_tls,omitempty"`

	// Sessions configures session management.
	Sessions SessionsConfig `json:"sessions"`

	// TrafficTunnelEndpoints are the allowed ports and options for tunneling.
	TrafficTunnelEndpoints []TrafficTunnelEndpoint `json:"traffic_tunnel_endpoints"`
}

// MarshalJSON marshals out this config.
func (nc NetworkConfig) MarshalJSON() ([]byte, error) {
	if nc.BindAddressDefaultSet {
		nc.BindAddress = ""
	}
	return json.Marshal(nc.NetworkConfigData)
}

// DefaultBindAddress is the default address that will be listened on. This default may
// not be used in managed cases when no bind address is explicitly set. In those cases
// the server will bind to all interfaces.
const DefaultBindAddress = "localhost:8080"

// Validate ensures all parts of the config are valid. Adds default BindAddress and HeartbeatWindow if not set.
func (nc *NetworkConfig) Validate(path string) error {
	if nc.BindAddress != "" && nc.Listener != nil {
		return resource.NewConfigValidationError(path, errors.New("may only set one of bind_address or listener"))
	}
	if nc.BindAddress == "" {
		nc.BindAddress = DefaultBindAddress
		nc.BindAddressDefaultSet = true
	}
	if _, _, err := net.SplitHostPort(nc.BindAddress); err != nil {
		return resource.NewConfigValidationError(path, errors.Wrap(err, "error validating bind_address"))
	}
	if (nc.TLSCertFile == "") != (nc.TLSKeyFile == "") {
		return resource.NewConfigValidationError(path, errors.New("must provide both tls_cert_file and tls_key_file"))
	}

	return nc.Sessions.Validate(path + ".sessions")
}

// SessionsConfig configures various parameters used in session management.
type SessionsConfig struct {
	// HeartbeatWindow is the window within which clients must send at least one
	// heartbeat in order to keep a session alive.
	HeartbeatWindow time.Duration
}

// Note: keep this in sync with SessionsConfig.
type sessionsConfigData struct {
	HeartbeatWindow string `json:"heartbeat_window,omitempty"`
}

// UnmarshalJSON unmarshals JSON data into this config.
func (sc *SessionsConfig) UnmarshalJSON(data []byte) error {
	var temp sessionsConfigData
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}
	if temp.HeartbeatWindow != "" {
		dur, err := time.ParseDuration(temp.HeartbeatWindow)
		if err != nil {
			return err
		}
		sc.HeartbeatWindow = dur
	}
	return nil
}

// MarshalJSON marshals out this config.
func (sc SessionsConfig) MarshalJSON() ([]byte, error) {
	var temp sessionsConfigData
	if sc.HeartbeatWindow != 0 {
		temp.HeartbeatWindow = sc.HeartbeatWindow.String()
	}
	return json.Marshal(temp)
}

// DefaultSessionHeartbeatWindow is the default session heartbeat window to use when not specified.
// It can be set with network.sessions.heartbeat_window.
const DefaultSessionHeartbeatWindow = 2 * time.Second

// Validate ensures all parts of the config are valid. Sets default HeartbeatWindow if not set.
func (sc *SessionsConfig) Validate(path string) error {
	if sc.HeartbeatWindow == 0 {
		sc.HeartbeatWindow = DefaultSessionHeartbeatWindow
	} else if sc.HeartbeatWindow < 30*time.Millisecond ||
		sc.HeartbeatWindow > time.Minute {
		return resource.NewConfigValidationError(path, errors.New("heartbeat_window must be between [30ms, 1m]"))
	}

	return nil
}

// TrafficTunnelEndpoint is an endpoint for tunneling traffic.
type TrafficTunnelEndpoint struct {
	// Port is the port which can be tunneled to/from.
	Port int
	// ConnectionTimeout is the timeout with which we will attempt to connect to the port.
	// If set to 0 or not specified, a default connection timeout of 10 seconds will be used.
	ConnectionTimeout time.Duration
}

// Note: keep this in sync with TrafficTunnelEndpoint.
type trafficTunnelEndpointData struct {
	Port              int    `json:"port"`
	ConnectionTimeout string `json:"connection_timeout,omitempty"`
}

// UnmarshalJSON unmarshals JSON data into this traffic tunnel endpoint.
func (tte *TrafficTunnelEndpoint) UnmarshalJSON(data []byte) error {
	var temp trafficTunnelEndpointData
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	tte.Port = temp.Port

	if temp.ConnectionTimeout != "" {
		dur, err := time.ParseDuration(temp.ConnectionTimeout)
		if err != nil {
			return err
		}
		tte.ConnectionTimeout = dur
	}

	return nil
}

// MarshalJSON marshals out this traffic tunnel endpoint.
func (tte *TrafficTunnelEndpoint) MarshalJSON() ([]byte, error) {
	var temp trafficTunnelEndpointData

	temp.Port = tte.Port

	if tte.ConnectionTimeout != 0 {
		temp.ConnectionTimeout = tte.ConnectionTimeout.String()
	}
	return json.Marshal(temp)
}

// AuthConfig describes authentication and authorization settings for the web server.
type AuthConfig struct {
	Handlers           []AuthHandlerConfig `json:"handlers,omitempty"`
	TLSAuthEntities    []string            `json:"tls_auth_entities,omitempty"`
	ExternalAuthConfig *ExternalAuthConfig `json:"external_auth_config,omitempty"`
}

// ExternalAuthConfig contains information needed to verify externally authenticated tokens.
type ExternalAuthConfig struct {
	// contains the raw jwks json.
	JSONKeySet rutils.AttributeMap `json:"jwks"`

	// on validation the JSONKeySet is validated and parsed into this field and can be used.
	ValidatedKeySet jwks.KeySet `json:"-"`
}

var (
	allowedKeyTypesForExternalAuth = map[string]bool{
		"RSA": true,
	}

	allowedAlgsForExternalAuth = map[string]bool{
		"RS256": true,
		"RS384": true,
		"RS512": true,
	}
)

// Validate returns true if the config is valid. Ensures each key is valid and meets the required constraints.
// Updates ValidatedKeySet once validated. A sample ExternalAuthConfig in JSON form is shown below, where "keys"
// contains a list of JSON Web Keys as defined in https://datatracker.ietf.org/doc/html/rfc7517.
//
//	"external_auth_config": {
//		"jwks": {
//			"keys": [
//				{
//					"alg": "XXXX",
//					"e": "XXXX",
//					"kid": "XXXX",
//					"kty": "XXXX",
//					"n": "XXXX"
//				}
//			]
//		}
//	}
func (c *ExternalAuthConfig) Validate(path string) error {
	jwksPath := fmt.Sprintf("%s.jwks", path)
	jsonJWKs, err := json.Marshal(c.JSONKeySet)
	if err != nil {
		return resource.NewConfigValidationError(jwksPath, errors.Wrap(err, "failed to marshal jwks"))
	}

	keyset, err := jwks.ParseKeySet(string(jsonJWKs))
	if err != nil {
		return resource.NewConfigValidationError(jwksPath, errors.Wrap(err, "failed to parse jwks"))
	}

	if keyset.Len() == 0 {
		return resource.NewConfigValidationError(jwksPath, errors.New("must contain at least 1 key"))
	}

	for i := 0; i < keyset.Len(); i++ {
		// validate keys
		key, ok := keyset.Get(i)
		if !ok {
			return resource.NewConfigValidationError(fmt.Sprintf("%s.%d", jwksPath, i), errors.New("failed to parse jwks, missing index"))
		}

		if _, ok := allowedKeyTypesForExternalAuth[key.KeyType().String()]; !ok {
			return resource.NewConfigValidationError(fmt.Sprintf("%s.%d", jwksPath, i),
				errors.Errorf("failed to parse jwks, invalid key type (%s) only (RSA) supported", key.KeyType().String()))
		}

		if _, ok := allowedAlgsForExternalAuth[key.Algorithm()]; !ok {
			return resource.NewConfigValidationError(fmt.Sprintf("%s.%d", jwksPath, i),
				errors.Errorf("failed to parse jwks, invalid alg (%s) type only (RS256, RS384, RS512) supported", key.Algorithm()))
		}
	}

	c.ValidatedKeySet = keyset

	return nil
}

// AuthHandlerConfig describes the configuration for a particular auth handler.
type AuthHandlerConfig struct {
	Type   rpc.CredentialsType `json:"type"`
	Config rutils.AttributeMap `json:"config"`
}

// Validate ensures all parts of the config are valid. If it exists, updates ExternalAuthConfig's ValidatedKeySet once validated.
// A sample AuthConfig in JSON form is shown below, where "handlers" contains a list of auth handlers. The only accepted credential
// type for the RDK in the config is "api-key" currently. An auth handler for utils.CredentialsTypeRobotLocationSecret may be added
// later by the RDK during processing.
//
//	"auth": {
//			"handlers": [
//				{
//					"type": "api-key",
//					"config": {
//						"API_KEY_ID": "API_KEY",
//						"API_KEY_ID_2": "API_KEY_2",
//						"keys": ["API_KEY_ID", "API_KEY_ID_2"]
//					}
//				}
//			],
//		    "external_auth_config": {}
//	}
func (config *AuthConfig) Validate(path string) error {
	seenTypes := make(map[string]struct{}, len(config.Handlers))
	for idx, handler := range config.Handlers {
		handlerPath := fmt.Sprintf("%s.%s.%d", path, "handlers", idx)
		if _, ok := seenTypes[string(handler.Type)]; ok {
			return resource.NewConfigValidationError(handlerPath, errors.Errorf("duplicate handler type %q", handler.Type))
		}
		seenTypes[string(handler.Type)] = struct{}{}
		if err := handler.Validate(handlerPath); err != nil {
			return err
		}
	}
	if config.ExternalAuthConfig != nil {
		if err := config.ExternalAuthConfig.Validate(fmt.Sprintf("%s.%s", path, "external_auth_config")); err != nil {
			return err
		}
	}
	return nil
}

// Validate ensures all parts of the config are valid.
func (config *AuthHandlerConfig) Validate(path string) error {
	if config.Type == "" {
		return resource.NewConfigValidationError(path, errors.New("handler must have type"))
	}
	switch config.Type {
	case rpc.CredentialsTypeAPIKey:
		if len(config.Config.StringSlice("keys")) == 0 {
			return resource.NewConfigValidationError(fmt.Sprintf("%s.config", path), errors.New("keys is required"))
		}
	case rpc.CredentialsTypeExternal:
		return errors.New("robot cannot issue external auth tokens")
	default:
		return resource.NewConfigValidationError(path, errors.Errorf("do not know how to handle auth for %q", config.Type))
	}
	return nil
}

// ParseAPIKeys parses API keys from the handler config. It will return an empty map
// if the credential type is not [rpc.CredentialsTypeAPIKey].
func ParseAPIKeys(handler AuthHandlerConfig) map[string]string {
	apiKeys := map[string]string{}
	if handler.Type == rpc.CredentialsTypeAPIKey {
		for k := range handler.Config {
			// if it is not a legacy api key indicated by "key(s)" key
			// current api keys will follow format { [keyId]: [key] }
			if k != "keys" && k != "key" {
				apiKeys[k] = handler.Config.String(k)
			}
		}
	}
	return apiKeys
}

// CreateTLSWithCert creates a tls.Config with the TLS certificate to be returned.
func CreateTLSWithCert(cfg *Config) (*tls.Config, error) {
	cert, err := tls.X509KeyPair([]byte(cfg.Cloud.TLSCertificate), []byte(cfg.Cloud.TLSPrivateKey))
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		GetCertificate: func(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
			// always return same cert
			return &cert, nil
		},
		GetClientCertificate: func(_ *tls.CertificateRequestInfo) (*tls.Certificate, error) {
			// always return same cert
			return &cert, nil
		},
	}, nil
}

// ProcessConfig processes robot configs.
func ProcessConfig(in *Config) (*Config, error) {
	out := *in
	var selfCreds *rpc.Credentials
	var selfAuthEntity string
	if in.Cloud != nil {
		// We expect a cloud config from app to always contain a non-empty `TLSCertificate` field.
		// We do this empty string check just to cope with unexpected input, such as cached configs
		// that are hand altered to have their `TLSCertificate` removed.
		if in.Cloud.TLSCertificate != "" {
			tlsConfig, err := CreateTLSWithCert(in)
			if err != nil {
				return nil, err
			}
			out.Network.TLSConfig = tlsConfig
		}
		if in.Cloud.APIKey.IsFullySet() {
			selfCreds = &rpc.Credentials{rutils.CredentialsTypeAPIKey, in.Cloud.APIKey.Key}
			selfAuthEntity = in.Cloud.APIKey.ID
		} else {
			selfCreds = &rpc.Credentials{rutils.CredentialsTypeRobotSecret, in.Cloud.Secret}
			selfAuthEntity = in.Cloud.ID
		}
	}

	out.Remotes = make([]Remote, len(in.Remotes))
	copy(out.Remotes, in.Remotes)
	for idx, remote := range out.Remotes {
		remoteCopy := remote
		if in.Cloud == nil {
			remoteCopy.Auth.SignalingCreds = remoteCopy.Auth.Credentials
		} else {
			if remote.ManagedBy != in.Cloud.ManagedBy {
				continue
			}
			remoteCopy.Auth.Managed = true
			remoteCopy.Auth.SignalingServerAddress = in.Cloud.SignalingAddress
			remoteCopy.Auth.SignalingAuthEntity = selfAuthEntity
			remoteCopy.Auth.SignalingCreds = selfCreds
		}
		out.Remotes[idx] = remoteCopy
	}
	return &out, nil
}

// DefaultPackageVersionValue default value of the package version used when empty.
const DefaultPackageVersionValue = "latest"

// PackageType indicates the type of the package
// This is used to replace placeholder strings in the config.
type PackageType string

const (
	// PackageTypeMlModel represents an ML model.
	PackageTypeMlModel PackageType = "ml_model"
	// PackageTypeModule represents a module type.
	PackageTypeModule PackageType = "module"
	// PackageTypeSlamMap represents a slam internal state.
	PackageTypeSlamMap PackageType = "slam_map"
)

// SupportedPackageTypes is a list of all of the valid package types.
var SupportedPackageTypes = []PackageType{PackageTypeMlModel, PackageTypeModule, PackageTypeSlamMap}

// A PackageConfig describes the configuration of a Package.
type PackageConfig struct {
	// Name is the local name of the package on the RDK. Must be unique across Packages. Must not be empty.
	Name string `json:"name"`
	// Package is the unqiue package name hosted by a remote PackageService. Must not be empty.
	Package string `json:"package"`
	// Version of the package ID hosted by a remote PackageService. If not specified "latest" is assumed.
	Version string `json:"version,omitempty"`
	// Types of the Package.
	Type PackageType `json:"type"`

	Status *AppValidationStatus `json:"status,omitempty"`

	alreadyValidated bool
	cachedErr        error
}

// Validate package config is valid.
func (p *PackageConfig) Validate(path string) error {
	if p.alreadyValidated {
		return p.cachedErr
	}

	if p.Status != nil {
		p.alreadyValidated = true
		p.cachedErr = resource.NewConfigValidationError(path, errors.New(p.Status.Error))
		return p.cachedErr
	}

	p.cachedErr = p.validate(path)
	p.alreadyValidated = true
	return p.cachedErr
}

func (p *PackageConfig) validate(path string) error {
	if p.Name == "" {
		return resource.NewConfigValidationError(path, errors.New("empty package name"))
	}

	if p.Package == "" {
		return resource.NewConfigValidationError(path, errors.New("empty package id"))
	}

	if !slices.Contains(SupportedPackageTypes, p.Type) {
		return resource.NewConfigValidationError(path, errors.Errorf("unsupported package type %q. Must be one of: %v",
			p.Type, SupportedPackageTypes))
	}

	if err := rutils.ValidatePackageName(p.Name); err != nil {
		return resource.NewConfigValidationError(path, err)
	}

	return nil
}

// Equals checks if the two configs are deeply equal to each other.
func (p PackageConfig) Equals(other PackageConfig) bool {
	p.alreadyValidated = false
	p.cachedErr = nil
	p.Status = nil
	other.alreadyValidated = false
	other.cachedErr = nil
	other.Status = nil
	//nolint:govet
	return reflect.DeepEqual(p, other)
}

// LocalDataDirectory returns the folder where the package should be extracted.
// Ex: /home/user/.viam/packages/data/ml_model/orgid_ballClassifier_0.1.2.
func (p *PackageConfig) LocalDataDirectory(packagesDir string) string {
	return filepath.Join(p.LocalDataParentDirectory(packagesDir), p.SanitizedName())
}

// LocalDownloadPath returns the file where the archive should be downloaded before extraction.
func (p *PackageConfig) LocalDownloadPath(packagesDir string) string {
	return filepath.Join(p.LocalDataParentDirectory(packagesDir), fmt.Sprintf("%s.download", p.SanitizedName()))
}

// LocalDataParentDirectory returns the folder that will contain the all packages of this type.
// Ex: /home/user/.viam/packages/data/ml_model.
func (p *PackageConfig) LocalDataParentDirectory(packagesDir string) string {
	return filepath.Join(packagesDir, "data", string(p.Type))
}

// SanitizedName returns the package name for the symlink/filepath of the package on the system.
func (p *PackageConfig) SanitizedName() string {
	// p.Package is set by the PackageServiceClient as "{org_id}/{package_name}"
	// see https://github.com/viamrobotics/app/blob/e0d693d80ae6f308e5b3a6bddb69991521127928/packages/packages.go#L1257
	return fmt.Sprintf("%s-%s", strings.ReplaceAll(p.Package, "/", "-"), p.sanitizedVersion())
}

// sanitizedVersion returns a cleaned version of the version so it is file-system-safe.
func (p *PackageConfig) sanitizedVersion() string {
	// replaces all the . if they exist with _
	return strings.ReplaceAll(p.Version, ".", "_")
}

// Revision encapsulates the revision of the latest config ingested by the robot along with
// a timestamp.
type Revision struct {
	Revision    string
	LastUpdated time.Time
}

// UpdateLoggerRegistryFromConfig will update the passed in registry with all log patterns
// in `cfg.LogConfig` and each resource's `LogConfiguration` field if present. It will
// also turn on or off log deduplication on the registry as necessary.
func UpdateLoggerRegistryFromConfig(registry *logging.Registry, cfg *Config, logger logging.Logger) {
	var combinedLogCfg []logging.LoggerPatternConfig
	if cfg.LogConfig != nil {
		combinedLogCfg = append(combinedLogCfg, cfg.LogConfig...)
	}

	for _, serv := range cfg.Services {
		if serv.LogConfiguration != nil {
			resLogCfg := logging.LoggerPatternConfig{
				Pattern: "rdk.resource_manager." + serv.ResourceName().String(),
				Level:   serv.LogConfiguration.Level.String(),
			}
			combinedLogCfg = append(combinedLogCfg, resLogCfg)
		}
	}
	for _, comp := range cfg.Components {
		if comp.LogConfiguration != nil {
			resLogCfg := logging.LoggerPatternConfig{
				Pattern: "rdk.resource_manager." + comp.ResourceName().String(),
				Level:   comp.LogConfiguration.Level.String(),
			}
			combinedLogCfg = append(combinedLogCfg, resLogCfg)
		}
	}

	registry.Update(combinedLogCfg, logger)

	// Check incoming disable log deduplication value for any diff. Note that config value
	// is a "disable" while registry is an "enable". This is by design to make configuration
	// easier for users and predicates easier for developers respectively. Due to this, the
	// conditional to check for diff below looks odd (== instead of !=.)
	if cfg.DisableLogDeduplication == registry.DeduplicateLogs.Load() {
		state := "enabled"
		if cfg.DisableLogDeduplication {
			state = "disabled"
		}
		registry.DeduplicateLogs.Store(!cfg.DisableLogDeduplication)
		logger.Infof("Noisy log deduplication is now %s", state)
	}

	// If a user-specified log pattern regex-matches the name of a module, update that
	// module's log level to be "debug." This will cause a restart of the module with
	// `--log-level=debug`. Only do this if the global log level is not already debug.
	//
	// NOTE(benji): This is hacky and simply a best-effort to honor user-specified log
	// patterns. We already emit a warning log in web/server/entrypoint.go#configWatcher
	// that points users to the 'log_level' and 'log_configuration' fields instead of 'log'
	// when trying to change levels of modular logs.
	var nonDebugGlobalLogger bool
	func() {
		globalLogger.mu.Lock()
		defer globalLogger.mu.Unlock()
		nonDebugGlobalLogger = globalLogger.actualGlobalLogger != nil && globalLogger.actualGlobalLogger.GetLevel() != logging.DEBUG
	}()
	if nonDebugGlobalLogger {
		for _, lpc := range cfg.LogConfig {
			// Only examine log patterns that have an associated level of "debug."
			if lpc.Level == moduleLogLevelDebug {
				for i, module := range cfg.Modules {
					// Only set a log level of "debug" if the pattern regex-matches the name of the
					// module, and the module does not already have a log level set
					r, err := regexp.Compile(logging.BuildRegexFromPattern(lpc.Pattern))
					if err != nil {
						// No need to log a warning here. The call to `registry.Update` above will
						// have logged one already.
						continue
					}

					if r.MatchString(module.Name) && module.LogLevel == "" {
						cfg.Modules[i].LogLevel = moduleLogLevelDebug
					}
				}
			}
		}
	}
}

// JobConfig describes regular job settings for the robot from the client.
type JobConfig struct {
	JobConfigData
}

// JobConfigData is the job config data that gets marshaled/unmarshaled.
type JobConfigData struct {
	Name             string              `json:"name"`
	Schedule         string              `json:"schedule"`
	Resource         string              `json:"resource"`
	Method           string              `json:"method"`
	Command          map[string]any      `json:"command,omitempty"`
	LogConfiguration *resource.LogConfig `json:"log_configuration,omitempty"`
}

// MarshalJSON marshals out this config.
func (jc JobConfig) MarshalJSON() ([]byte, error) {
	return json.Marshal(jc.JobConfigData)
}

// UnmarshalJSON unmarshals JSON data into this config.
func (jc *JobConfig) UnmarshalJSON(data []byte) error {
	var temp JobConfigData
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	*jc = JobConfig{
		temp,
	}
	return nil
}

// Validate checks that every required field is present.
func (jc *JobConfig) Validate(path string) error {
	if jc.Name == "" {
		return resource.NewConfigValidationFieldRequiredError(path, "name")
	}
	if jc.Method == "" {
		return resource.NewConfigValidationFieldRequiredError(path, "method")
	}
	if jc.Resource == "" {
		return resource.NewConfigValidationFieldRequiredError(path, "resource")
	}
	if jc.Schedule == "" {
		return resource.NewConfigValidationFieldRequiredError(path, "schedule")
	}
	// At this point, the schedule could still be invalid (not a golang duration string or a
	// cron expression). Such errors will be caught later, when the job manager will try to
	// schedule the job and parse this field. The error will be displayed to the user.
	return nil
}

// Equals checks if the two configs are deeply equal to each other.
func (jc JobConfig) Equals(other JobConfig) bool {
	return reflect.DeepEqual(jc, other)
}
