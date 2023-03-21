// Package config defines the structures to configure a robot and its connected parts.
package config

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils"
	"go.viam.com/utils/jwks"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/referenceframe"
	rutils "go.viam.com/rdk/utils"
)

// A Config describes the configuration of a robot.
type Config struct {
	Cloud      *Cloud                `json:"cloud,omitempty"`
	Modules    []Module              `json:"modules,omitempty"`
	Remotes    []Remote              `json:"remotes,omitempty"`
	Components []Component           `json:"components,omitempty"`
	Processes  []pexec.ProcessConfig `json:"processes,omitempty"`
	Services   []Service             `json:"services,omitempty"`
	Packages   []PackageConfig       `json:"packages,omitempty"`
	Network    NetworkConfig         `json:"network"`
	Auth       AuthConfig            `json:"auth"`
	Debug      bool                  `json:"debug,omitempty"`

	ConfigFilePath string `json:"-"`

	// AllowInsecureCreds is used to have all connections allow insecure
	// downgrades and send credentials over plaintext. This is an option
	// a user must pass via command line arguments.
	AllowInsecureCreds bool `json:"-"`

	// UntrustedEnv is used to disable Processes and shell for untrusted environments
	// where a process cannot be trusted. This is an option a user must pass via
	// command line arguments.
	UntrustedEnv bool `json:"-"`

	// LimitConfigurableDirectories is used to limit which directories users can configure for
	// storing data on-robot. This is set via command line arguments.
	LimitConfigurableDirectories bool `json:"-"`

	// FromCommand indicates if this config was parsed via the web server command.
	// If false, it's for creating a robot via the RDK library. This is helpful for
	// error messages that can indicate flags/config fields to use.
	FromCommand bool `json:"-"`

	// DisablePartialStart ensures that a robot will only start when all the components,
	// services, and remotes pass config validation. This value is false by default
	DisablePartialStart bool `json:"disable_partial_start"`

	// PackagePath sets the directory used to store packages locally. Defaults to ~/.viam/packages
	PackagePath string `json:"-"`
}

// ValidNameRegex is the pattern that matches to a valid name.
// The name must begin with a letter i.e. [a-zA-Z],
// and the body can only contain 0 or more numbers, letters, dashes and underscores i.e. [-\w]*.
var ValidNameRegex = regexp.MustCompile(`^[a-zA-Z][-\w]*$`)

// Ensure ensures all parts of the config are valid.
func (c *Config) Ensure(fromCloud bool) error {
	if c.Cloud != nil {
		if err := c.Cloud.Validate("cloud", fromCloud); err != nil {
			return err
		}
	}

	for idx := 0; idx < len(c.Modules); idx++ {
		if err := c.Modules[idx].Validate(fmt.Sprintf("%s.%d", "modules", idx)); err != nil {
			if c.DisablePartialStart {
				return err
			}
			golog.Global().Error(errors.Wrap(err, "Module config error, starting robot without module: "+c.Modules[idx].Name))
		}
	}

	for idx := 0; idx < len(c.Remotes); idx++ {
		if err := c.Remotes[idx].Validate(fmt.Sprintf("%s.%d", "remotes", idx)); err != nil {
			if c.DisablePartialStart {
				return err
			}
			golog.Global().Error(errors.Wrap(err, "Remote config error, starting robot without remote: "+c.Remotes[idx].Name))
		}
	}

	for idx := 0; idx < len(c.Components); idx++ {
		dependsOn, err := c.Components[idx].Validate(fmt.Sprintf("%s.%d", "components", idx))
		if err != nil {
			fullErr := errors.Errorf("error validating component %s: %s", c.Components[idx].Name, err)
			if c.DisablePartialStart {
				return fullErr
			}
			golog.Global().Error(errors.Wrap(err, "Component config error, starting robot without component: "+c.Components[idx].Name))
		} else {
			c.Components[idx].ImplicitDependsOn = dependsOn
		}
	}

	for idx := 0; idx < len(c.Processes); idx++ {
		if err := c.Processes[idx].Validate(fmt.Sprintf("%s.%d", "processes", idx)); err != nil {
			if c.DisablePartialStart {
				return err
			}
			golog.Global().Error(errors.Wrap(err, "Process config error, starting robot without process: "+c.Processes[idx].Name))
		}
	}

	for idx := 0; idx < len(c.Services); idx++ {
		dependsOn, err := c.Services[idx].Validate(fmt.Sprintf("%s.%d", "services", idx))
		if err != nil {
			if c.DisablePartialStart {
				return err
			}
			golog.Global().Error(errors.Wrap(err, "Service config error, starting robot without service: "+c.Services[idx].Name))
		} else {
			c.Services[idx].ImplicitDependsOn = dependsOn
		}
	}

	if err := c.Network.Validate("network"); err != nil {
		return err
	}

	if err := c.Auth.Validate("auth"); err != nil {
		return err
	}

	for idx := 0; idx < len(c.Packages); idx++ {
		if err := c.Packages[idx].Validate(fmt.Sprintf("%s.%d", "packages", idx)); err != nil {
			fullErr := errors.Errorf("error validating package config %s", err)
			if c.DisablePartialStart {
				return fullErr
			}
			golog.Global().Error(errors.Wrap(err, "Package config error, starting robot without package: "+c.Packages[idx].Name))
		}
	}

	return nil
}

// FindComponent finds a particular component by name.
func (c Config) FindComponent(name string) *Component {
	for _, cmp := range c.Components {
		if cmp.Name == name {
			return &cmp
		}
	}
	return nil
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
	Name                    string
	Address                 string
	Frame                   *referenceframe.LinkConfig
	Auth                    RemoteAuth
	ManagedBy               string
	Insecure                bool
	ConnectionCheckInterval time.Duration
	ReconnectInterval       time.Duration
	ServiceConfig           []ResourceLevelServiceConfig

	// Secret is a helper for a robot location secret.
	Secret string
}

// Note: keep this in sync with Remote.
type remoteData struct {
	Name                    string                       `json:"name"`
	Address                 string                       `json:"address"`
	Frame                   *referenceframe.LinkConfig   `json:"frame,omitempty"`
	Auth                    RemoteAuth                   `json:"auth"`
	ManagedBy               string                       `json:"managed_by"`
	Insecure                bool                         `json:"insecure"`
	ConnectionCheckInterval string                       `json:"connection_check_interval,omitempty"`
	ReconnectInterval       string                       `json:"reconnect_interval,omitempty"`
	ServiceConfig           []ResourceLevelServiceConfig `json:"service_config"`

	// Secret is a helper for a robot location secret.
	Secret string `json:"secret"`
}

// UnmarshalJSON unmarshals JSON data into this config.
func (config *Remote) UnmarshalJSON(data []byte) error {
	var temp remoteData
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}
	*config = Remote{
		Name:          temp.Name,
		Address:       temp.Address,
		Frame:         temp.Frame,
		Auth:          temp.Auth,
		ManagedBy:     temp.ManagedBy,
		Insecure:      temp.Insecure,
		ServiceConfig: temp.ServiceConfig,
		Secret:        temp.Secret,
	}
	if temp.ConnectionCheckInterval != "" {
		dur, err := time.ParseDuration(temp.ConnectionCheckInterval)
		if err != nil {
			return err
		}
		config.ConnectionCheckInterval = dur
	}
	if temp.ReconnectInterval != "" {
		dur, err := time.ParseDuration(temp.ReconnectInterval)
		if err != nil {
			return err
		}
		config.ReconnectInterval = dur
	}
	return nil
}

// MarshalJSON marshals out this config.
func (config Remote) MarshalJSON() ([]byte, error) {
	temp := remoteData{
		Name:          config.Name,
		Address:       config.Address,
		Frame:         config.Frame,
		Auth:          config.Auth,
		ManagedBy:     config.ManagedBy,
		Insecure:      config.Insecure,
		ServiceConfig: config.ServiceConfig,
		Secret:        config.Secret,
	}
	if config.ConnectionCheckInterval != 0 {
		temp.ConnectionCheckInterval = config.ConnectionCheckInterval.String()
	}
	if config.ReconnectInterval != 0 {
		temp.ReconnectInterval = config.ReconnectInterval.String()
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
	Managed                bool             `json:""`
	SignalingServerAddress string           `json:""`
	SignalingAuthEntity    string           `json:""`
	SignalingCreds         *rpc.Credentials `json:""`
}

// Validate ensures all parts of the config are valid.
func (config *Remote) Validate(path string) error {
	if config.Name == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "name")
	}
	if !ValidNameRegex.MatchString(config.Name) {
		return utils.NewConfigValidationError(path,
			errors.Errorf("Remote name %q must only contain letters, numbers, dashes, and underscores", config.Name))
	}
	if config.Address == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "address")
	}
	if config.Frame != nil {
		if config.Frame.Parent == "" {
			return utils.NewConfigValidationFieldRequiredError(path, "frame.parent")
		}
	}

	if config.Secret != "" {
		config.Auth = RemoteAuth{
			Credentials: &rpc.Credentials{
				Type:    rutils.CredentialsTypeRobotLocationSecret,
				Payload: config.Secret,
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
	ID                string           `json:"id"`
	Secret            string           `json:"secret"`
	LocationSecret    string           `json:"location_secret"`
	LocationSecrets   []LocationSecret `json:"location_secrets"`
	ManagedBy         string           `json:"managed_by"`
	FQDN              string           `json:"fqdn"`
	LocalFQDN         string           `json:"local_fqdn"`
	SignalingAddress  string           `json:"signaling_address"`
	SignalingInsecure bool             `json:"signaling_insecure,omitempty"`
	Path              string           `json:"path"`
	LogPath           string           `json:"log_path"`
	AppAddress        string           `json:"app_address"`
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
	ID string
	// Payload of the secret
	Secret string
}

// NetworkConfig describes networking settings for the web server.
type NetworkConfig struct {
	NetworkConfigData
}

// NetworkConfigData is the network config data that gets marshaled/unmarshaled.
type NetworkConfigData struct {
	// FQDN is the unique name of this server.
	FQDN string `json:"fqdn"`

	// Listener is the listener that the web server will use. This is mutually
	// exclusive with BindAddress.
	Listener net.Listener `json:"-"`

	// BindAddress is the address that the web server will bind to.
	// The default behavior is to bind to localhost:8080. This is mutually
	// exclusive with Listener.
	BindAddress string `json:"bind_address"`

	BindAddressDefaultSet bool `json:"-"`

	// TLSCertFile is used to enable secure communications on the hosted HTTP server.
	// This is mutually exclusive with TLSCertPEM and TLSKeyPEM.
	TLSCertFile string `json:"tls_cert_file"`

	// TLSKeyFile is used to enable secure communications on the hosted HTTP server.
	// This is mutually exclusive with TLSCertPEM and TLSKeyPEM.
	TLSKeyFile string `json:"tls_key_file"`

	// TLSConfig is used to enable secure communications on the hosted HTTP server.
	// This is mutually exclusive with TLSCertFile and TLSKeyFile.
	TLSConfig *tls.Config `json:"-"`

	// Sessions configures session management.
	Sessions SessionsConfig `json:"sessions"`
}

// MarshalJSON marshals out this config.
func (nc *NetworkConfig) MarshalJSON() ([]byte, error) {
	configCopy := *nc
	if configCopy.BindAddressDefaultSet {
		configCopy.BindAddress = ""
	}
	return json.Marshal(configCopy.NetworkConfigData)
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
	} else if sc.HeartbeatWindow < 10*time.Millisecond ||
		sc.HeartbeatWindow > time.Minute {
		return utils.NewConfigValidationError(path, errors.New("heartbeat_window must be between [10ms, 1m]"))
	}

	return nil
}

// AuthConfig describes authentication and authorization settings for the web server.
type AuthConfig struct {
	Handlers        []AuthHandlerConfig `json:"handlers"`
	TLSAuthEntities []string            `json:"tls_auth_entities"`
	// TODO(erd): test
	ExternalAuthConfig *ExternalAuthConfig `json:"external_auth_config,omitempty"`
}

// ExternalAuthConfig contains information needed to verify externally authenticated tokens.
type ExternalAuthConfig struct {
	// contains the raw jwks json.
	JSONKeySet AttributeMap `json:"jwks"`

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
	Config AttributeMap        `json:"config"`
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
	case rpc.CredentialsType("oauth-web-auth"):
		// TODO(APP-1412): remove after a week from being deployed
		return nil
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

// Updateable is implemented when component/service of a robot should be updated with the config.
type Updateable interface {
	// Update updates the resource
	Update(context.Context, *Config) error
}

// Regex to match if a config is referencing a Package. Group is the package name.
var packageReferenceRegex = regexp.MustCompile(`^\$\{packages\.([A-Za-z0-9_\/-]+)}(.*)`)

// DefaultPackageVersionValue default value of the package version used when empty.
const DefaultPackageVersionValue = "latest"

// A PackageConfig describes the configuration of a Package.
type PackageConfig struct {
	// Name is the local name of the package on the RDK. Must be unique across Packages. Must not be empty.
	Name string `json:"name"`
	// Package is the unqiue package name hosted by a remote PackageService. Must not be empty.
	Package string `json:"package"`
	// Version of the package ID hosted by a remote PackageService. If not specified "latest" is assumed.
	Version string `json:"version,omitempty"`
}

// Validate package config is valid.
func (p *PackageConfig) Validate(path string) error {
	if p.Name == "" {
		return utils.NewConfigValidationError(path, errors.New("empty package name"))
	}

	if p.Package == "" {
		return utils.NewConfigValidationError(path, errors.New("empty package id"))
	}

	if !ValidNameRegex.MatchString(p.Name) {
		return errors.Errorf("package %s name must contain only letters, numbers, underscores and hyphens", path)
	}

	return nil
}

// GetPackageReference a PackageReference if the given path has a Package reference eg. ${packages.some-package}/path.
// Returns nil if no package reference is found.
func GetPackageReference(path string) *PackageReference {
	// return early before regex match
	if len(path) == 0 || path[0] != '$' {
		return nil
	}

	match := packageReferenceRegex.FindStringSubmatch(path)
	if match == nil {
		return nil
	}

	if len(match) != 3 {
		return nil
	}

	return &PackageReference{Package: match[1], PathInPackage: match[2]}
}

// PackageReference contains the deconstructed parts of a package reference in the config.
// Eg: ${packages.some-package}/path/a/b/c -> {"some-package", "/path/a/b/c"}.
type PackageReference struct {
	Package       string
	PathInPackage string
}
