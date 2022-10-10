// Package config defines the structures to configure a robot and its connected parts.
package config

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"

	rutils "go.viam.com/rdk/utils"
)

// A Config describes the configuration of a robot.
type Config struct {
	Cloud      *Cloud                `json:"cloud,omitempty"`
	Remotes    []Remote              `json:"remotes,omitempty"`
	Components []Component           `json:"components,omitempty"`
	Processes  []pexec.ProcessConfig `json:"processes,omitempty"`
	Services   []Service             `json:"services,omitempty"`
	Network    NetworkConfig         `json:"network"`
	Auth       AuthConfig            `json:"auth"`

	Debug bool `json:"debug,omitempty"`

	ConfigFilePath string `json:"-"`

	// AllowInsecureCreds is used to have all connections allow insecure
	// downgrades and send credentials over plaintext. This is an option
	// a user must pass via command line arguments.
	AllowInsecureCreds bool `json:"-"`

	// FromCommand indicates if this config was parsed via the web server command.
	// If false, it's for creating a robot via the RDK library. This is helpful for
	// error messages that can indicate flags/config fields to use.
	FromCommand bool `json:"-"`
}

// Ensure ensures all parts of the config are valid.
func (c *Config) Ensure(fromCloud bool) error {
	if c.Cloud != nil {
		if err := c.Cloud.Validate("cloud", fromCloud); err != nil {
			return err
		}
	}

	for idx := 0; idx < len(c.Remotes); idx++ {
		if err := c.Remotes[idx].Validate(fmt.Sprintf("%s.%d", "remotes", idx)); err != nil {
			return err
		}
	}

	for idx := 0; idx < len(c.Components); idx++ {
		dependsOn, err := c.Components[idx].Validate(fmt.Sprintf("%s.%d", "components", idx))
		if err != nil {
			return errors.Errorf("error validating component %s: %s", c.Components[idx].Name, err)
		}
		c.Components[idx].ImplicitDependsOn = dependsOn
	}

	for idx := 0; idx < len(c.Processes); idx++ {
		if err := c.Processes[idx].Validate(fmt.Sprintf("%s.%d", "processes", idx)); err != nil {
			return err
		}
	}

	for idx := 0; idx < len(c.Services); idx++ {
		if err := c.Services[idx].Validate(fmt.Sprintf("%s.%d", "services", idx)); err != nil {
			return err
		}
	}

	if err := c.Network.Validate("network"); err != nil {
		return err
	}

	if err := c.Auth.Validate("auth"); err != nil {
		return err
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
// The Frame field defines how the "world" node of the remote robot should be reconciled with the "world" node of the
// the current robot. All components of the remote robot who have Parent as "world" will be attached to the parent defined
// in Frame, and with the given offset as well.
type Remote struct {
	Name                    string                       `json:"name"`
	Address                 string                       `json:"address"`
	Frame                   *Frame                       `json:"frame,omitempty"`
	Auth                    RemoteAuth                   `json:"auth"`
	ManagedBy               string                       `json:"managed_by"`
	Insecure                bool                         `json:"insecure"`
	ConnectionCheckInterval time.Duration                `json:"connection_check_interval,omitempty"`
	ReconnectInterval       time.Duration                `json:"reconnect_interval,omitempty"`
	ServiceConfig           []ResourceLevelServiceConfig `json:"service_config"`

	// Secret is a helper for a robot location secret.
	Secret string `json:"secret"`
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
	ID                string        `json:"id"`
	Secret            string        `json:"secret"`
	LocationSecret    string        `json:"location_secret"`
	ManagedBy         string        `json:"managed_by"`
	FQDN              string        `json:"fqdn"`
	LocalFQDN         string        `json:"local_fqdn"`
	SignalingAddress  string        `json:"signaling_address"`
	SignalingInsecure bool          `json:"signaling_insecure,omitempty"`
	Path              string        `json:"path"`
	LogPath           string        `json:"log_path"`
	AppAddress        string        `json:"app_address"`
	RefreshInterval   time.Duration `json:"refresh_interval,omitempty"`

	// cached by us and fetched from a non-config endpoint.
	TLSCertificate string `json:"tls_certificate"`
	TLSPrivateKey  string `json:"tls_private_key"`
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

	return nil
}

// AuthConfig describes authentication and authorization settings for the web server.
type AuthConfig struct {
	Handlers        []AuthHandlerConfig `json:"handlers"`
	TLSAuthEntities []string            `json:"tls_auth_entities"`
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
