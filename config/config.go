// Package config defines the structures to configure a robot and its connected parts.
package config

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/pkg/errors"
	"go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"

	functionvm "go.viam.com/rdk/function/vm"
	"go.viam.com/rdk/rlog"

	configpb "go.viam.com/rdk/proto/api/config/v1"
	 "google.golang.org/protobuf/types/known/durationpb"

)

// SortComponents sorts list of components topologically based off what other components they depend on.
func SortComponents(components []Component) ([]Component, error) {
	componentToConfig := make(map[string]Component, len(components))
	dependencies := map[string][]string{}

	for _, config := range components {
		if _, ok := componentToConfig[config.Name]; ok {
			return nil, errors.Errorf("component name %q is not unique", config.Name)
		}
		componentToConfig[config.Name] = config
		dependencies[config.Name] = config.DependsOn
	}

	for name, dps := range dependencies {
		for _, depName := range dps {
			if _, ok := componentToConfig[depName]; !ok {
				return nil, utils.NewConfigValidationError(
					fmt.Sprintf("%s.%s", "components", name),
					errors.Errorf("dependency %q does not exist", depName),
				)
			}
		}
	}

	sortedCmps := make([]Component, 0, len(components))
	visited := map[string]bool{}

	var dfsHelper func(string, []string) error
	dfsHelper = func(name string, path []string) error {
		for idx, cmpName := range path {
			if name == cmpName {
				return errors.Errorf("circular dependency detected in component list between %s", strings.Join(path[idx:], ", "))
			}
		}

		path = append(path, name)
		if _, ok := visited[name]; ok {
			return nil
		}
		visited[name] = true
		dps := dependencies[name]
		for _, dp := range dps {
			// create a deep copy of current path
			pathCopy := make([]string, len(path))
			copy(pathCopy, path)

			if err := dfsHelper(dp, pathCopy); err != nil {
				return err
			}
		}
		sortedCmps = append(sortedCmps, componentToConfig[name])
		return nil
	}

	for _, c := range components {
		if _, ok := visited[c.Name]; !ok {
			var path []string
			if err := dfsHelper(c.Name, path); err != nil {
				return nil, err
			}
		}
	}

	return sortedCmps, nil
}

// A Config describes the configuration of a robot.
type Config struct {
	ConfigFilePath string
	Cloud          *Cloud                      `json:"cloud,omitempty"`
	Remotes        []Remote                    `json:"remotes,omitempty"`
	Components     []Component                 `json:"components,omitempty"`
	Processes      []pexec.ProcessConfig       `json:"processes,omitempty"`
	Functions      []functionvm.FunctionConfig `json:"functions,omitempty"`
	Services       []Service                   `json:"services,omitempty"`
	Network        NetworkConfig               `json:"network"`
	Auth           AuthConfig                  `json:"auth"`
}

// Ensure ensures all parts of the config are valid and sorts components based on what they depend on.
func (c *Config) Ensure(fromCloud bool) error {
	if c.Cloud != nil {
		if err := c.Cloud.Validate("cloud", fromCloud); err != nil {
			return err
		}
	}

	for idx, config := range c.Remotes {
		if err := config.Validate(fmt.Sprintf("%s.%d", "remotes", idx)); err != nil {
			return err
		}
	}

	for idx, config := range c.Components {
		if err := config.Validate(fmt.Sprintf("%s.%d", "components", idx)); err != nil {
			return err
		}
	}

	if len(c.Components) > 0 {
		srtCmps, err := SortComponents(c.Components)
		if err != nil {
			return err
		}
		c.Components = srtCmps
	}

	for idx, config := range c.Processes {
		if err := config.Validate(fmt.Sprintf("%s.%d", "processes", idx)); err != nil {
			return err
		}
	}

	for idx, config := range c.Functions {
		if err := config.Validate(fmt.Sprintf("%s.%d", "functions", idx)); err != nil {
			return err
		}
	}

	for idx, config := range c.Services {
		if err := config.Validate(fmt.Sprintf("%s.%d", "services", idx)); err != nil {
			return err
		}
	}

	if err := c.Network.Validate("network"); err != nil {
		return err
	}

	if err := c.Auth.Validate("auth"); err != nil {
		return err
	}

	if len(c.Auth.Handlers) == 0 {
		host, _, err := net.SplitHostPort(c.Network.BindAddress)
		if err != nil {
			return err // unexpected since network validation validates this
		}
		if host == "" || host == "0.0.0.0" || host == "::" {
			rlog.Logger.Warn("binding to all interfaces without authentication")
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

// A Remote describes a remote robot that should be integrated.
// The Frame field defines how the "world" node of the remote robot should be reconciled with the "world" node of the
// the current robot. All components of the remote robot who have Parent as "world" will be attached to the parent defined
// in Frame, and with the given offset as well.
type Remote struct {
	Name    string     `json:"name"`
	Address string     `json:"address"`
	Prefix  bool       `json:"prefix"`
	Frame   *Frame     `json:"frame,omitempty"`
	Auth    RemoteAuth `json:"auth"`
}

// RemoteAuth specifies how to authenticate against a remote. If no credentials are
// specified, authentication does not happen. If an entity is specified, the
// authentication request will specify it.
type RemoteAuth struct {
	Credentials *rpc.Credentials `json:"credentials"`
	Entity      string           `json:"entity"`
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
	return nil
}

// A Cloud describes how to configure a robot controlled by the
// cloud.
// The cloud source could be anything that supports http.
// URL is constructed as $Path?id=ID and secret is put in a http header.
type Cloud configpb.Cloud

// Validate ensures all parts of the config are valid.
func (config *Cloud) Validate(path string, fromCloud bool) error {
	if config.Id == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "id")
	}
	if fromCloud {
		if len(config.Fqdns) == 0 {
			return utils.NewConfigValidationFieldRequiredError(path, "fqdns")
		}
		for idx, fqdn := range config.Fqdns {
			if fqdn == "" {
				return utils.NewConfigValidationFieldRequiredError(path, fmt.Sprintf("%s.%d", "fqdns", idx))
			}
		}
	} else if config.Secret == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "secret")
	}
	if config.RefreshInterval.AsDuration() == 0 {
		config.RefreshInterval = durationpb.New(10 * time.Second)
	}
	return nil
}

// NetworkConfig describes networking settings for the web server.
type NetworkConfig struct {
	// BindAddress is the address that the web server will bind to.
	// The default behavior is to bind to localhost:8080.
	BindAddress string `json:"bind_address"`

	// TLSCertFile is used to enable secure communications on the hosted HTTP server.
	TLSCertFile string `json:"tls_cert_file"`

	// TLSKeyFile is used to enable secure communications on the hosted HTTP server.
	TLSKeyFile string `json:"tls_key_file"`
}

// Validate ensures all parts of the config are valid.
func (config *NetworkConfig) Validate(path string) error {
	if config.BindAddress == "" {
		config.BindAddress = "localhost:8080"
	}
	if _, _, err := net.SplitHostPort(config.BindAddress); err != nil {
		return utils.NewConfigValidationError(path, errors.Wrap(err, "error validating bind_address"))
	}
	if (config.TLSCertFile == "") != (config.TLSKeyFile == "") {
		return utils.NewConfigValidationError(path, errors.New("must provide both tls_cert_file and tls_key_file"))
	}
	return nil
}

// AuthConfig describes authentication and authorization settings for the web server.
type AuthConfig struct {
	Handlers []AuthHandlerConfig `json:"handlers"`
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
		if config.Config.String("key") == "" {
			return utils.NewConfigValidationFieldRequiredError(fmt.Sprintf("%s.config", path), "key")
		}
	default:
		return utils.NewConfigValidationError(path, errors.Errorf("do not know how to handle auth for %q", config.Type))
	}
	return nil
}
