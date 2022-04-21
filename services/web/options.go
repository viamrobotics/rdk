// Package web provides gRPC/REST/GUI APIs to control and monitor a robot.
package web

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"

	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	rutils "go.viam.com/rdk/utils"
)

// Options are used for configuring the web server.
type Options struct {
	// Pprof turns on the pprof profiler accessible at /debug
	Pprof bool

	// SharedDir is the location of static web assets.
	SharedDir string

	// SignalingAddress is where to listen to WebRTC call offers at.
	SignalingAddress string

	// SignalingDialOpts are the dial options to used for signaling.
	SignalingDialOpts []rpc.DialOption

	// FQDN is the FQDN of this host. It is assumed this FQDN is unique to
	// one robot.
	FQDN string

	// LocalFQDN is the local FQDN of this host used for UI links
	// and SDK connections. It is assumed this FQDN is unique to one
	// robot.
	LocalFQDN string

	// Debug turns on various debugging features. For example, the echo gRPC
	// service is added.
	Debug bool

	// WebRTC configures whether or not to instruct all clients to prefer to
	// WebRTC connections over direct gRPC connections.
	WebRTC bool

	// Network describes networking settings for the web server.
	Network config.NetworkConfig

	// Auth describes authentication and authorization settings for the web server.
	Auth config.AuthConfig

	// Managed signifies if this server is remotely managed (e.g. from some cloud service).
	Managed bool

	// secure determines if sever communicates are secured or not.
	secure bool

	// baked information when managed to make local UI simpler
	BakedAuthEntity string
	BakedAuthCreds  rpc.Credentials
}

// NewOptions returns a default set of options which will have the
// web server run on config.DefaultBindAddress.
func NewOptions() Options {
	return Options{
		Pprof: false,
		Network: config.NetworkConfig{
			NetworkConfigData: config.NetworkConfigData{
				BindAddress:           config.DefaultBindAddress,
				BindAddressDefaultSet: true,
			},
		},
	}
}

// OptionsFromConfig returns an Options populated by the config passed in.
func OptionsFromConfig(cfg *config.Config) (Options, error) {
	options := NewOptions()

	options.Auth = cfg.Auth
	options.Network = cfg.Network
	options.FQDN = cfg.Network.FQDN
	if cfg.Cloud != nil {
		options.Managed = true
		options.LocalFQDN = cfg.Cloud.LocalFQDN
		options.FQDN = cfg.Cloud.FQDN
		options.SignalingAddress = cfg.Cloud.SignalingAddress

		if cfg.Cloud.TLSCertificate != "" {
			// override
			options.Network.TLSConfig = cfg.Network.TLSConfig

			// NOTE(RDK-148):
			// when we are managed and no explicit bind address is set,
			// we will listen everywhere on 8080. We assume this to be
			// secure because TLS will be enabled in addition to
			// authentication. NOTE: If you do not want the UI to function
			// without a specific secret being input, then you must set up
			// a dedicated auth handler in the config. Otherwise, the secret
			// for this robot will be baked into the UI. There may be a future
			// feature to disable the baked in credentials from the managed
			// interface.
			if cfg.Network.BindAddressDefaultSet {
				options.Network.BindAddress = ":8080"
			}

			cert, err := cfg.Network.TLSConfig.GetCertificate(&tls.ClientHelloInfo{})
			if err != nil {
				return Options{}, err
			}
			leaf, err := x509.ParseCertificate(cert.Certificate[0])
			if err != nil {
				return Options{}, err
			}
			options.Auth.TLSAuthEntities = leaf.DNSNames
		}

		options.Auth.Handlers = make([]config.AuthHandlerConfig, len(cfg.Auth.Handlers))
		copy(options.Auth.Handlers, cfg.Auth.Handlers)
		options.Auth.Handlers = append(options.Auth.Handlers, config.AuthHandlerConfig{
			Type: rutils.CredentialsTypeRobotLocationSecret,
			Config: config.AttributeMap{
				"secret": cfg.Cloud.LocationSecret,
			},
		})

		signalingDialOpts := []rpc.DialOption{rpc.WithEntityCredentials(
			cfg.Cloud.ID,
			rpc.Credentials{rutils.CredentialsTypeRobotSecret, cfg.Cloud.Secret},
		)}
		if cfg.Cloud.SignalingInsecure {
			signalingDialOpts = append(signalingDialOpts, rpc.WithInsecure())
		}

		options.BakedAuthEntity = options.FQDN
		options.BakedAuthCreds = rpc.Credentials{
			rutils.CredentialsTypeRobotLocationSecret,
			cfg.Cloud.LocationSecret,
		}
		options.SignalingDialOpts = signalingDialOpts
	}
	return options, nil
}

type Hosts struct {
	names    []string
	internal []string
	external []string
}

// Derive host configurations from options
func (options *Options) GetHosts(listenerTCPAddr *net.TCPAddr) Hosts {
	var hosts = Hosts{
		names:    []string{options.FQDN},
		external: []string{options.FQDN},
		internal: []string{options.FQDN},
	}

	listenerAddr := listenerTCPAddr.String()
	localhostWithPort := LocalHostWithPort(listenerTCPAddr)

	addSignalingHost := func(host string, set []string, seen map[string]bool) []string {
		if _, ok := seen[host]; ok {
			return set
		}
		seen[host] = true
		return append(set, host)
	}
	seenExternalSignalingHosts := map[string]bool{options.FQDN: true}
	seenInternalSignalingHosts := map[string]bool{options.FQDN: true}

	if !options.Managed {
		// allow signaling for non-unique entities.
		// This eases WebRTC connections.
		if options.FQDN != listenerAddr {
			hosts.external = addSignalingHost(listenerAddr, hosts.external, seenExternalSignalingHosts)
			hosts.internal = addSignalingHost(listenerAddr, hosts.internal, seenInternalSignalingHosts)
		}
		if listenerTCPAddr.IP.IsLoopback() {
			// plus localhost alias
			hosts.external = addSignalingHost(localhostWithPort, hosts.external, seenExternalSignalingHosts)
			hosts.internal = addSignalingHost(localhostWithPort, hosts.internal, seenInternalSignalingHosts)
		}
	}

	if options.LocalFQDN != "" {
		// only add the local FQDN here since we will already have DefaultFQDN
		// in the case that FQDNs was empty, avoiding a duplicate host. If FQDNs
		// is non-empty, we don't care about having a default for signaling/naming.
		hosts.names = append(hosts.names, options.LocalFQDN)
		hosts.internal = addSignalingHost(options.LocalFQDN, hosts.internal, seenInternalSignalingHosts)
		localFQDNWithPort := fmt.Sprintf("%s%s", options.LocalFQDN, listenerPortStr(listenerTCPAddr))
		hosts.internal = addSignalingHost(localFQDNWithPort, hosts.internal, seenInternalSignalingHosts)
	}

	return hosts
}

func LocalHostWithPort(listenerTCPAddr *net.TCPAddr) string {
	return fmt.Sprintf("localhost%s", listenerPortStr(listenerTCPAddr))
}

func listenerPortStr(listenerTCPAddr *net.TCPAddr) string {
	var listenerPortStr string

	listenerPort := listenerTCPAddr.Port
	if listenerPort != 443 {
		listenerPortStr = fmt.Sprintf(":%d", listenerPort)
	}
	return listenerPortStr
}
