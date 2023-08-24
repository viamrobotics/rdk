// Package weboptions provides Options for configuring a web server
package weboptions

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"

	"github.com/pion/webrtc/v3"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/utils"
)

// Options are used for configuring the web server.
type Options struct {
	// Pprof turns on the pprof profiler accessible at /debug
	Pprof bool

	// SharedDir is the location of static web assets.
	SharedDir string

	// StaticHost is a url to use for static assets, like app.viam.com
	StaticHost string

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

	// Secure determines if sever communicates are secured or not.
	Secure bool

	// baked information when managed to make local UI simpler
	BakedAuthEntity string
	BakedAuthCreds  rpc.Credentials

	WebRTCOnPeerAdded   func(pc *webrtc.PeerConnection)
	WebRTCOnPeerRemoved func(pc *webrtc.PeerConnection)

	DisableMulticastDNS bool
}

// New returns a default set of options which will have the
// web server run on config.DefaultBindAddress.
func New() Options {
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

// FromConfig returns an Options populated by the config passed in.
func FromConfig(cfg *config.Config) (Options, error) {
	options := New()

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

			// This will only happen if we're switching from a local config to a cloud config.
			if cfg.Network.TLSConfig == nil {
				return Options{}, errors.New("switching from local config to cloud config not currently supported")
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

		allLocationSecrets := secretsToStringSlice(cfg.Cloud.LocationSecrets)
		if len(allLocationSecrets) == 0 {
			return options, errors.New("no LocationSecrets specified")
		}

		options.Auth.Handlers = append(options.Auth.Handlers, config.AuthHandlerConfig{
			Type: utils.CredentialsTypeRobotLocationSecret,
			Config: utils.AttributeMap{
				"secrets": allLocationSecrets,
			},
		})

		signalingDialOpts := []rpc.DialOption{rpc.WithEntityCredentials(
			cfg.Cloud.ID,
			rpc.Credentials{utils.CredentialsTypeRobotSecret, cfg.Cloud.Secret},
		)}
		if cfg.Cloud.SignalingInsecure {
			signalingDialOpts = append(signalingDialOpts, rpc.WithInsecure())
		}

		options.BakedAuthEntity = options.FQDN
		options.BakedAuthCreds = rpc.Credentials{
			utils.CredentialsTypeRobotLocationSecret,
			allLocationSecrets[0],
		}
		options.SignalingDialOpts = signalingDialOpts
	}
	return options, nil
}

func secretsToStringSlice(secrets []config.LocationSecret) []string {
	out := make([]string, 0, len(secrets))
	for _, s := range secrets {
		out = append(out, s.Secret)
	}
	return out
}

// Hosts configurations.
type Hosts struct {
	Names    []string
	Internal []string
	External []string
}

// GetHosts derives host configurations from options.
func (options *Options) GetHosts(listenerTCPAddr *net.TCPAddr) Hosts {
	hosts := Hosts{
		Names:    []string{options.FQDN},
		External: []string{options.FQDN},
		Internal: []string{options.FQDN},
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
			hosts.External = addSignalingHost(listenerAddr, hosts.External, seenExternalSignalingHosts)
			hosts.Internal = addSignalingHost(listenerAddr, hosts.Internal, seenInternalSignalingHosts)
		}
		if listenerTCPAddr.IP.IsLoopback() {
			// plus localhost alias
			hosts.External = addSignalingHost(localhostWithPort, hosts.External, seenExternalSignalingHosts)
			hosts.Internal = addSignalingHost(localhostWithPort, hosts.Internal, seenInternalSignalingHosts)
		}
	}

	if options.LocalFQDN != "" {
		// only add the local FQDN here since we will already have DefaultFQDN
		// in the case that FQDNs was empty, avoiding a duplicate host. If FQDNs
		// is non-empty, we don't care about having a default for signaling/naming.
		hosts.Names = append(hosts.Names, options.LocalFQDN)
		hosts.Internal = addSignalingHost(options.LocalFQDN, hosts.Internal, seenInternalSignalingHosts)
		localFQDNWithPort := fmt.Sprintf("%s%s", options.LocalFQDN, listenerPortStr(listenerTCPAddr))
		hosts.Internal = addSignalingHost(localFQDNWithPort, hosts.Internal, seenInternalSignalingHosts)
	}

	return hosts
}

// LocalHostWithPort returns a properly formatted localhost address with port.
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
