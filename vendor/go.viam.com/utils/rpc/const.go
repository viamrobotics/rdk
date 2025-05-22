package rpc

import "time"

var (
	// MaxMessageSize is the maximum size a gRPC message can be.
	MaxMessageSize = 1 << 25

	// keepAliveTime is how often to establish Keepalive pings/expectations.
	keepAliveTime = 10 * time.Second

	// SocksProxyEnvVar is the name of an environment variable used by SOCKS
	// proxies to indicate the address through which to route all network traffic
	// via SOCKS5.
	SocksProxyEnvVar = "SOCKS_PROXY"

	// OnlySocksProxyEnvVar is the name of an environment variable used if all network
	// traffic should be done through SOCKS5.
	OnlySocksProxyEnvVar = "ONLY_SOCKS_PROXY"
)
