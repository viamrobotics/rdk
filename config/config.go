// Package config defines the structures to configure a robot and its connected parts.
package config

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.viam.com/utils"
	"go.viam.com/utils/jwks"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"
	"golang.org/x/exp/slices"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	rutils "go.viam.com/rdk/utils"
)

// A Config describes the configuration of a robot.
type Config struct {
	Cloud           *Cloud
	Modules         []Module
	Remotes         []Remote
	Components      []resource.Config
	Processes       []pexec.ProcessConfig
	Services        []resource.Config
	Packages        []PackageConfig
	Network         NetworkConfig
	Auth            AuthConfig
	Debug           bool
	GlobalLogConfig []GlobalLogConfig

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

	// DisablePartialStart ensures that a robot will only start when all the components,
	// services, and remotes pass config validation. This value is false by default
	DisablePartialStart bool

	// PackagePath sets the directory used to store packages locally. Defaults to ~/.viam/packages
	PackagePath string
}

// NOTE: This data must be maintained with what is in Config.
type configData struct {
	Cloud               *Cloud                `json:"cloud,omitempty"`
	Modules             []Module              `json:"modules,omitempty"`
	Remotes             []Remote              `json:"remotes,omitempty"`
	Components          []resource.Config     `json:"components,omitempty"`
	Processes           []pexec.ProcessConfig `json:"processes,omitempty"`
	Services            []resource.Config     `json:"services,omitempty"`
	Packages            []PackageConfig       `json:"packages,omitempty"`
	Network             NetworkConfig         `json:"network"`
	Auth                AuthConfig            `json:"auth"`
	Debug               bool                  `json:"debug,omitempty"`
	DisablePartialStart bool                  `json:"disable_partial_start"`
	GlobalLogConfig     []GlobalLogConfig     `json:"global_log_configuration"`
}

type appValidationStatus struct {
	Error string `bson:"error"`
}

func (c *Config) validateUniqueResource(logger logging.Logger, seenResources map[string]bool, name string) error {
	if _, exists := seenResources[name]; exists {
		errString := errors.Errorf("duplicate resource %s in robot config", name)
		if c.DisablePartialStart {
			return errString
		}
		logger.Error(errString)
	}
	seenResources[name] = true
	return nil
}

// Ensure ensures all parts of the config are valid.
func (c *Config) Ensure(fromCloud bool, logger logging.Logger) error {
	seenResources := make(map[string]bool)

	if c.Cloud != nil {
		if err := c.Cloud.Validate("cloud", fromCloud); err != nil {
			return err
		}
	}

	if err := c.Network.Validate("network"); err != nil {
		return err
	}

	if err := c.Auth.Validate("auth"); err != nil {
		return err
	}

	for idx := 0; idx < len(c.Modules); idx++ {
		if err := c.Modules[idx].Validate(fmt.Sprintf("%s.%d", "modules", idx)); err != nil {
			if c.DisablePartialStart {
				return err
			}
			logger.Errorw("module config error; starting robot without module", "name", c.Modules[idx].Name, "error", err)
		}
		if err := c.validateUniqueResource(logger, seenResources, c.Modules[idx].Name); err != nil {
			return err
		}
	}

	for idx := 0; idx < len(c.Remotes); idx++ {
		if _, err := c.Remotes[idx].Validate(fmt.Sprintf("%s.%d", "remotes", idx)); err != nil {
			if c.DisablePartialStart {
				return err
			}
			logger.Errorw("remote config error; starting robot without remote", "name", c.Remotes[idx].Name, "error", err)
		}
		// we need to figure out how to make it so that the remote is tied to the API
		resourceRemoteName := resource.NewName(resource.APINamespaceRDK.WithType("remote").WithSubtype(""), c.Remotes[idx].Name)
		if err := c.validateUniqueResource(logger, seenResources, resourceRemoteName.String()); err != nil {
			return err
		}
	}

	for idx := 0; idx < len(c.Components); idx++ {
		dependsOn, err := c.Components[idx].Validate(fmt.Sprintf("%s.%d", "components", idx), resource.APITypeComponentName)
		if err != nil {
			fullErr := errors.Errorf("error validating component %s: %s", c.Components[idx].Name, err)
			if c.DisablePartialStart {
				return fullErr
			}
			logger.Errorw("component config error; starting robot without component", "name", c.Components[idx].Name, "error", err)
		} else {
			c.Components[idx].ImplicitDependsOn = dependsOn
		}
		if err := c.validateUniqueResource(logger, seenResources, c.Components[idx].ResourceName().String()); err != nil {
			return err
		}
	}

	for idx := 0; idx < len(c.Processes); idx++ {
		if err := c.Processes[idx].Validate(fmt.Sprintf("%s.%d", "processes", idx)); err != nil {
			if c.DisablePartialStart {
				return err
			}
			logger.Errorw("process config error; starting robot without process", "name", c.Processes[idx].Name, "error", err)
		}

		if err := c.validateUniqueResource(logger, seenResources, c.Processes[idx].ID); err != nil {
			return err
		}
	}

	for idx := 0; idx < len(c.Services); idx++ {
		dependsOn, err := c.Services[idx].Validate(fmt.Sprintf("%s.%d", "services", idx), resource.APITypeServiceName)
		if err != nil {
			if c.DisablePartialStart {
				return err
			}
			logger.Errorw("service config error; starting robot without service", "name", c.Services[idx].Name, "error", err)
		} else {
			c.Services[idx].ImplicitDependsOn = dependsOn
		}

		if err := c.validateUniqueResource(logger, seenResources, c.Services[idx].ResourceName().String()); err != nil {
			return err
		}
	}

	for idx := 0; idx < len(c.Packages); idx++ {
		if err := c.Packages[idx].Validate(fmt.Sprintf("%s.%d", "packages", idx)); err != nil {
			fullErr := errors.Errorf("error validating package config %s", err)
			if c.DisablePartialStart {
				return fullErr
			}
			logger.Errorw("package config error; starting robot without package", "name", c.Packages[idx].Name, "error", err)
		}
		if err := c.validateUniqueResource(logger, seenResources, c.Packages[idx].Package); err != nil {
			return err
		}
	}

	for idx, globalLogConfig := range c.GlobalLogConfig {
		if err := globalLogConfig.Validate(fmt.Sprintf("global_log_configuration.%d", idx)); err != nil {
			logger.Errorw("log configuration error", "err", err)
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
	for idx := range conf.Remotes {
		conf.Remotes[idx].adjustPartialNames()
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
	c.DisablePartialStart = conf.DisablePartialStart
	c.GlobalLogConfig = conf.GlobalLogConfig

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
	for idx := range c.Remotes {
		c.Remotes[idx].adjustPartialNames()
	}

	return json.Marshal(configData{
		Cloud:               c.Cloud,
		Modules:             c.Modules,
		Remotes:             c.Remotes,
		Components:          c.Components,
		Processes:           c.Processes,
		Services:            c.Services,
		Packages:            c.Packages,
		Network:             c.Network,
		Auth:                c.Auth,
		Debug:               c.Debug,
		DisablePartialStart: c.DisablePartialStart,
		GlobalLogConfig:     c.GlobalLogConfig,
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
		Frame:                     conf.Frame,
		Auth:                      conf.Auth,
		ManagedBy:                 conf.ManagedBy,
		Insecure:                  conf.Insecure,
		AssociatedResourceConfigs: conf.AssociatedResourceConfigs,
		Secret:                    conf.Secret,
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
func (conf *Remote) Validate(path string) ([]string, error) {
	if conf.alreadyValidated {
		return nil, conf.cachedErr
	}
	conf.cachedErr = conf.validate(path)
	conf.alreadyValidated = true
	return nil, conf.cachedErr
}

// adjustPartialNames assumes this config comes from a place where the associated
// config type names are partially stored (JSON/Proto/Database) and will
// fix them up to the builtin values they are intended for.
func (conf *Remote) adjustPartialNames() {
	for idx := range conf.AssociatedResourceConfigs {
		conf.AssociatedResourceConfigs[idx].RemoteName = conf.Name
	}
}

func (conf *Remote) validate(path string) error {
	conf.adjustPartialNames()

	if conf.Name == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "name")
	}
	if !rutils.ValidNameRegex.MatchString(conf.Name) {
		return utils.NewConfigValidationError(path, rutils.ErrInvalidName(conf.Name))
	}
	if conf.Address == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "address")
	}
	if conf.Frame != nil {
		if conf.Frame.Parent == "" {
			return utils.NewConfigValidationFieldRequiredError(path, "frame.parent")
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
// The cloud source could be anything that supports http.
// URL is constructed as $Path?id=ID and secret is put in a http header.
type Cloud struct {
	ID                string
	Secret            string
	LocationSecret    string // Deprecated: Use LocationSecrets
	LocationSecrets   []LocationSecret
	ManagedBy         string
	FQDN              string
	LocalFQDN         string
	SignalingAddress  string
	SignalingInsecure bool
	Path              string
	LogPath           string
	AppAddress        string
	RefreshInterval   time.Duration

	// cached by us and fetched from a non-config endpoint.
	TLSCertificate string
	TLSPrivateKey  string
}

// Note: keep this in sync with Cloud.
type cloudData struct {
	// these three fields are only set within the config passed to the robot as an argumenet.
	ID         string `json:"id"`
	Secret     string `json:"secret,omitempty"`
	AppAddress string `json:"app_address,omitempty"`

	LocationSecret    string           `json:"location_secret"`
	LocationSecrets   []LocationSecret `json:"location_secrets"`
	ManagedBy         string           `json:"managed_by"`
	FQDN              string           `json:"fqdn"`
	LocalFQDN         string           `json:"local_fqdn"`
	SignalingAddress  string           `json:"signaling_address"`
	SignalingInsecure bool             `json:"signaling_insecure,omitempty"`
	Path              string           `json:"path,omitempty"`
	LogPath           string           `json:"log_path,omitempty"`
	RefreshInterval   string           `json:"refresh_interval,omitempty"`

	// cached by us and fetched from a non-config endpoint.
	TLSCertificate string `json:"tls_certificate"`
	TLSPrivateKey  string `json:"tls_private_key"`
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
		ManagedBy:         temp.ManagedBy,
		FQDN:              temp.FQDN,
		LocalFQDN:         temp.LocalFQDN,
		SignalingAddress:  temp.SignalingAddress,
		SignalingInsecure: temp.SignalingInsecure,
		Path:              temp.Path,
		LogPath:           temp.LogPath,
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
		ManagedBy:         config.ManagedBy,
		FQDN:              config.FQDN,
		LocalFQDN:         config.LocalFQDN,
		SignalingAddress:  config.SignalingAddress,
		SignalingInsecure: config.SignalingInsecure,
		Path:              config.Path,
		LogPath:           config.LogPath,
		AppAddress:        config.AppAddress,
		TLSCertificate:    config.TLSCertificate,
		TLSPrivateKey:     config.TLSPrivateKey,
	}
	if config.RefreshInterval != 0 {
		temp.RefreshInterval = config.RefreshInterval.String()
	}
	return json.Marshal(temp)
}

// Validate ensures all parts of the config are valid.
func (config *Cloud) Validate(path string, fromCloud bool) error {
	if config.ID == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "id")
	}
	if fromCloud {
		if config.FQDN == "" {
			return utils.NewConfigValidationFieldRequiredError(path, "fqdn")
		}
		if config.LocalFQDN == "" {
			return utils.NewConfigValidationFieldRequiredError(path, "local_fqdn")
		}
	} else if config.Secret == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "secret")
	}
	if config.RefreshInterval == 0 {
		config.RefreshInterval = 10 * time.Second
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

	// Sessions configures session management.
	Sessions SessionsConfig `json:"sessions"`
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

// Validate ensures all parts of the config are valid.
func (nc *NetworkConfig) Validate(path string) error {
	if nc.BindAddress != "" && nc.Listener != nil {
		return utils.NewConfigValidationError(path, errors.New("may only set one of bind_address or listener"))
	}
	if nc.BindAddress == "" {
		nc.BindAddress = DefaultBindAddress
		nc.BindAddressDefaultSet = true
	}
	if _, _, err := net.SplitHostPort(nc.BindAddress); err != nil {
		return utils.NewConfigValidationError(path, errors.Wrap(err, "error validating bind_address"))
	}
	if (nc.TLSCertFile == "") != (nc.TLSKeyFile == "") {
		return utils.NewConfigValidationError(path, errors.New("must provide both tls_cert_file and tls_key_file"))
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

// Validate ensures all parts of the config are valid.
func (sc *SessionsConfig) Validate(path string) error {
	if sc.HeartbeatWindow == 0 {
		sc.HeartbeatWindow = DefaultSessionHeartbeatWindow
	} else if sc.HeartbeatWindow < 30*time.Millisecond ||
		sc.HeartbeatWindow > time.Minute {
		return utils.NewConfigValidationError(path, errors.New("heartbeat_window must be between [30ms, 1m]"))
	}

	return nil
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
func (c *ExternalAuthConfig) Validate(path string) error {
	jwksPath := fmt.Sprintf("%s.jwks", path)
	jsonJWKs, err := json.Marshal(c.JSONKeySet)
	if err != nil {
		return utils.NewConfigValidationError(jwksPath, errors.Wrap(err, "failed to marshal jwks"))
	}

	keyset, err := jwks.ParseKeySet(string(jsonJWKs))
	if err != nil {
		return utils.NewConfigValidationError(jwksPath, errors.Wrap(err, "failed to parse jwks"))
	}

	if keyset.Len() == 0 {
		return utils.NewConfigValidationError(jwksPath, errors.New("must contain at least 1 key"))
	}

	for i := 0; i < keyset.Len(); i++ {
		// validate keys
		key, ok := keyset.Get(i)
		if !ok {
			return utils.NewConfigValidationError(fmt.Sprintf("%s.%d", jwksPath, i), errors.New("failed to parse jwks, missing index"))
		}

		if _, ok := allowedKeyTypesForExternalAuth[key.KeyType().String()]; !ok {
			return utils.NewConfigValidationError(fmt.Sprintf("%s.%d", jwksPath, i),
				errors.Errorf("failed to parse jwks, invalid key type (%s) only (RSA) supported", key.KeyType().String()))
		}

		if _, ok := allowedAlgsForExternalAuth[key.Algorithm()]; !ok {
			return utils.NewConfigValidationError(fmt.Sprintf("%s.%d", jwksPath, i),
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

// Validate ensures all parts of the config are valid.
func (config *AuthConfig) Validate(path string) error {
	seenTypes := make(map[string]struct{}, len(config.Handlers))
	for idx, handler := range config.Handlers {
		handlerPath := fmt.Sprintf("%s.%s.%d", path, "handlers", idx)
		if _, ok := seenTypes[string(handler.Type)]; ok {
			return utils.NewConfigValidationError(handlerPath, errors.Errorf("duplicate handler type %q", handler.Type))
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
		return utils.NewConfigValidationError(path, errors.New("handler must have type"))
	}
	switch config.Type {
	case rpc.CredentialsTypeAPIKey:
		if config.Config.String("key") == "" && len(config.Config.StringSlice("keys")) == 0 {
			return utils.NewConfigValidationError(fmt.Sprintf("%s.config", path), errors.New("key or keys is required"))
		}
	case rpc.CredentialsTypeExternal:
		return errors.New("robot cannot issue external auth tokens")
	default:
		return utils.NewConfigValidationError(path, errors.Errorf("do not know how to handle auth for %q", config.Type))
	}
	return nil
}

// TLSConfig stores the TLS config for the robot.
type TLSConfig struct {
	*tls.Config
	certMu  sync.Mutex
	tlsCert *tls.Certificate
}

// NewTLSConfig creates a new tls config.
func NewTLSConfig(cfg *Config) *TLSConfig {
	tlsCfg := &TLSConfig{}
	var tlsConfig *tls.Config
	if cfg.Cloud != nil && cfg.Cloud.TLSCertificate != "" {
		tlsConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			GetCertificate: func(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
				// always return same cert
				tlsCfg.certMu.Lock()
				defer tlsCfg.certMu.Unlock()
				return tlsCfg.tlsCert, nil
			},
			GetClientCertificate: func(_ *tls.CertificateRequestInfo) (*tls.Certificate, error) {
				// always return same cert
				tlsCfg.certMu.Lock()
				defer tlsCfg.certMu.Unlock()
				return tlsCfg.tlsCert, nil
			},
		}
	}
	tlsCfg.Config = tlsConfig
	return tlsCfg
}

// UpdateCert updates the TLS certificate to be returned.
func (t *TLSConfig) UpdateCert(cfg *Config) error {
	cert, err := tls.X509KeyPair([]byte(cfg.Cloud.TLSCertificate), []byte(cfg.Cloud.TLSPrivateKey))
	if err != nil {
		return err
	}
	t.certMu.Lock()
	t.tlsCert = &cert
	t.certMu.Unlock()
	return nil
}

// ProcessConfig processes robot configs.
func ProcessConfig(in *Config, tlsCfg *TLSConfig) (*Config, error) {
	out := *in
	var selfCreds *rpc.Credentials
	if in.Cloud != nil {
		if in.Cloud.TLSCertificate != "" {
			if err := tlsCfg.UpdateCert(in); err != nil {
				return nil, err
			}
		}

		selfCreds = &rpc.Credentials{rutils.CredentialsTypeRobotSecret, in.Cloud.Secret}
		out.Network.TLSConfig = tlsCfg.Config // override
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
			remoteCopy.Auth.SignalingAuthEntity = in.Cloud.ID
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
	// PackageTypeBoardDefs represents a linux board definition file.
	PackageTypeBoardDefs PackageType = "board_defs"
)

// SupportedPackageTypes is a list of all of the valid package types.
var SupportedPackageTypes = []PackageType{PackageTypeMlModel, PackageTypeModule, PackageTypeSlamMap, PackageTypeBoardDefs}

// A PackageConfig describes the configuration of a Package.
type PackageConfig struct {
	// Name is the local name of the package on the RDK. Must be unique across Packages. Must not be empty.
	Name string `json:"name"`
	// Package is the unqiue package name hosted by a remote PackageService. Must not be empty.
	Package string `json:"package"`
	// Version of the package ID hosted by a remote PackageService. If not specified "latest" is assumed.
	Version string `json:"version,omitempty"`
	// Types of the Package. If not specified it is assumed to be ml_model.
	Type PackageType `json:"type,omitempty"`

	Status *appValidationStatus `json:"status,omitempty"`

	alreadyValidated bool
	cachedErr        error
}

// Validate package config is valid.
func (p *PackageConfig) Validate(path string) error {
	if p.Status != nil {
		return errors.New(p.Status.Error)
	}

	if p.alreadyValidated {
		return p.cachedErr
	}
	p.cachedErr = p.validate(path)
	p.alreadyValidated = true
	return p.cachedErr
}

func (p *PackageConfig) validate(path string) error {
	if p.Name == "" {
		return utils.NewConfigValidationError(path, errors.New("empty package name"))
	}

	if p.Package == "" {
		return utils.NewConfigValidationError(path, errors.New("empty package id"))
	}

	if p.Type == "" {
		// for backwards compatibility
		p.Type = PackageTypeMlModel
	}

	if !slices.Contains(SupportedPackageTypes, p.Type) {
		return utils.NewConfigValidationError(path, errors.Errorf("unsupported package type %q. Must be one of: %v",
			p.Type, SupportedPackageTypes))
	}

	if !rutils.ValidNameRegex.MatchString(p.Name) {
		return utils.NewConfigValidationError(path, rutils.ErrInvalidName(p.Name))
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
// Ex: /home/user/.viam/packages/.data/ml_model/orgid_ballClassifier_0.1.2.
func (p *PackageConfig) LocalDataDirectory(packagesDir string) string {
	return filepath.Join(p.LocalDataParentDirectory(packagesDir), p.SanitizedName())
}

// LocalDownloadPath returns the file where the archive should be downloaded before extraction.
func (p *PackageConfig) LocalDownloadPath(packagesDir string) string {
	return filepath.Join(p.LocalDataParentDirectory(packagesDir), fmt.Sprintf("%s.download", p.SanitizedName()))
}

// LocalDataParentDirectory returns the folder that will contain the all packages of this type.
// Ex: /home/user/.viam/packages/.data/ml_model.
func (p *PackageConfig) LocalDataParentDirectory(packagesDir string) string {
	return filepath.Join(packagesDir, ".data", string(p.Type))
}

// SanitizedName returns the package name for the symlink/filepath of the package on the system.
func (p *PackageConfig) SanitizedName() string {
	return fmt.Sprintf("%s-%s", strings.ReplaceAll(p.Package, string(os.PathSeparator), "-"), p.sanitizedVersion())
}

// sanitizedVersion returns a cleaned version of the version so it is file-system-safe.
func (p *PackageConfig) sanitizedVersion() string {
	// replaces all the . if they exist with _
	return strings.ReplaceAll(p.Version, ".", "_")
}
