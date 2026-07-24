package client

import (
	"crypto/tls"

	"go.viam.com/utils/rpc"
)

// This file re-exports the client-side dial-option surface of go.viam.com/utils/rpc so
// that SDK users can configure machine connections without importing
// go.viam.com/utils/rpc directly. The underlying implementation still lives in goutils;
// these aliases and wrappers are the rdk-owned entry point for it.

// DialOption configures how a client dials a gRPC server. It is accepted by
// WithDialOptions.
type DialOption = rpc.DialOption

// ClientConn is an established client connection to a gRPC server.
//
//nolint:revive
type ClientConn = rpc.ClientConn

// Credentials packages up both a type of credential along with its payload which
// is formatted specific to the type.
type Credentials = rpc.Credentials

// CredentialsType signifies a means of representing a credential. For example,
// an API key.
type CredentialsType = rpc.CredentialsType

// DialWebRTCOptions control how WebRTC is utilized in a dial attempt.
type DialWebRTCOptions = rpc.DialWebRTCOptions

// DialMulticastDNSOptions dictate any special settings to apply while dialing via mDNS.
type DialMulticastDNSOptions = rpc.DialMulticastDNSOptions

const (
	// CredentialsTypeAPIKey is intended for external users, human and computer.
	CredentialsTypeAPIKey = rpc.CredentialsTypeAPIKey

	// CredentialsTypeExternal is for credentials that are to be produced by some external
	// authentication endpoint intended for another, different consumer at a different
	// endpoint.
	CredentialsTypeExternal = rpc.CredentialsTypeExternal
)

// WithInsecure returns a DialOption which disables transport security for this
// ClientConn. Note that transport security is required unless WithInsecure is set.
func WithInsecure() DialOption {
	return rpc.WithInsecure()
}

// WithCredentials returns a DialOption which sets the credentials to use for
// authenticating the request. The associated entity is assumed to be the address of the
// server. This is mutually exclusive with WithEntityCredentials.
func WithCredentials(creds Credentials) DialOption {
	return rpc.WithCredentials(creds)
}

// WithEntityCredentials returns a DialOption which sets the entity credentials to use for
// authenticating the request. This is mutually exclusive with WithCredentials.
func WithEntityCredentials(entity string, creds Credentials) DialOption {
	return rpc.WithEntityCredentials(entity, creds)
}

// WithExternalAuth returns a DialOption which sets the address to use to perform
// authentication. Authentication done in this manner will never have the dialed address
// be authenticated against but instead have access tokens sent directly to it.
func WithExternalAuth(addr, toEntity string) DialOption {
	return rpc.WithExternalAuth(addr, toEntity)
}

// WithExternalAuthInsecure returns a DialOption which disables transport security for
// this ClientConn when doing external auth.
func WithExternalAuthInsecure() DialOption {
	return rpc.WithExternalAuthInsecure()
}

// WithStaticAuthenticationMaterial returns a DialOption which sets the already
// authenticated material (opaque) to use for authenticated requests.
func WithStaticAuthenticationMaterial(authMaterial string) DialOption {
	return rpc.WithStaticAuthenticationMaterial(authMaterial)
}

// WithStaticExternalAuthenticationMaterial returns a DialOption which sets the already
// authenticated material (opaque) to use for authenticated requests against an external
// auth service.
func WithStaticExternalAuthenticationMaterial(authMaterial string) DialOption {
	return rpc.WithStaticExternalAuthenticationMaterial(authMaterial)
}

// WithTLSConfig sets the TLS configuration to use for all secured connections.
func WithTLSConfig(config *tls.Config) DialOption {
	return rpc.WithTLSConfig(config)
}

// WithWebRTCOptions returns a DialOption which sets the WebRTC options to use if the
// dialer tries to establish a WebRTC connection.
//
// Note: this sets the complete DialWebRTCOptions base struct, replacing WebRTC
// options set before it. The granular ICE/TURN options (WithForceP2P,
// WithForceRelay, WithTurn*) are meant to layer on top, so pass them after
// WithWebRTCOptions and they override just their own fields. The base struct is
// typically set first (often by the dialer), so this ordering is the norm.
func WithWebRTCOptions(webrtcOpts DialWebRTCOptions) DialOption {
	return rpc.WithWebRTCOptions(webrtcOpts)
}

// WithDialDebug returns a DialOption which informs the client to be in a debug mode as
// much as possible.
func WithDialDebug() DialOption {
	return rpc.WithDialDebug()
}

// WithAllowInsecureDowngrade returns a DialOption which allows connections to be
// downgraded to plaintext if TLS cannot be detected properly. This is not used when there
// are credentials present.
func WithAllowInsecureDowngrade() DialOption {
	return rpc.WithAllowInsecureDowngrade()
}

// WithAllowInsecureWithCredentialsDowngrade returns a DialOption which allows connections
// to be downgraded to plaintext if TLS cannot be detected properly while using
// credentials. This is generally unsafe to use but can be requested.
func WithAllowInsecureWithCredentialsDowngrade() DialOption {
	return rpc.WithAllowInsecureWithCredentialsDowngrade()
}

// WithDialMulticastDNSOptions returns a DialOption which allows setting options to
// specifically be used while doing a dial based off mDNS discovery.
func WithDialMulticastDNSOptions(opts DialMulticastDNSOptions) DialOption {
	return rpc.WithDialMulticastDNSOptions(opts)
}

// WithDisableDirectGRPC returns a DialOption which disables directly dialing a gRPC
// server. There's not really a good reason to use this unless it's for testing.
func WithDisableDirectGRPC() DialOption {
	return rpc.WithDisableDirectGRPC()
}

// WithForceDirectGRPC forces direct dialing to the target address. This option disables
// WebRTC connections and mDNS lookup.
func WithForceDirectGRPC() DialOption {
	return rpc.WithForceDirectGRPC()
}

// WithForceP2P returns a DialOption which forces ICE connections to use only
// non-relay candidates (host, server-reflexive, and peer-reflexive) by
// stripping TURN servers.
//
// Note: this sets only its own field, layering on top of any WithWebRTCOptions.
// Apply it after WithWebRTCOptions, since a later WithWebRTCOptions replaces the
// entire struct and would overwrite this.
func WithForceP2P() DialOption {
	return rpc.WithForceP2P()
}

// WithForceRelay returns a DialOption which forces ICE connections to use relay
// (TURN) candidates only.
//
// Note: this sets only its own field, layering on top of any WithWebRTCOptions.
// Apply it after WithWebRTCOptions, since a later WithWebRTCOptions replaces the
// entire struct and would overwrite this.
func WithForceRelay() DialOption {
	return rpc.WithForceRelay()
}

// WithTurnURI returns a DialOption which filters the signaling server's TURN
// list down to the server whose parsed URI matches uri (e.g.
// "turn:turn.viam.com:443"). Has no effect when ForceP2P is set, since TURN
// servers are stripped in that case.
//
// Note: this sets only its own field, layering on top of any WithWebRTCOptions.
// Apply it after WithWebRTCOptions, since a later WithWebRTCOptions replaces the
// entire struct and would overwrite this.
func WithTurnURI(uri string) DialOption {
	return rpc.WithTurnURI(uri)
}

// WithTurnScheme returns a DialOption which overrides the scheme ("turn" or
// "turns") of the URI matched by WithTurnURI.
//
// Note: this sets only its own field, layering on top of any WithWebRTCOptions.
// Apply it after WithWebRTCOptions, since a later WithWebRTCOptions replaces the
// entire struct and would overwrite this.
func WithTurnScheme(scheme string) DialOption {
	return rpc.WithTurnScheme(scheme)
}

// WithTurnTransport returns a DialOption which overrides the transport ("tcp" or
// "udp") of the URI matched by WithTurnURI.
//
// Note: this sets only its own field, layering on top of any WithWebRTCOptions.
// Apply it after WithWebRTCOptions, since a later WithWebRTCOptions replaces the
// entire struct and would overwrite this.
func WithTurnTransport(transport string) DialOption {
	return rpc.WithTurnTransport(transport)
}

// WithTurnPort returns a DialOption which overrides the port of the URI matched
// by WithTurnURI. A port of 0 means no override.
//
// Note: this sets only its own field, layering on top of any WithWebRTCOptions.
// Apply it after WithWebRTCOptions, since a later WithWebRTCOptions replaces the
// entire struct and would overwrite this.
func WithTurnPort(port int) DialOption {
	return rpc.WithTurnPort(port)
}
