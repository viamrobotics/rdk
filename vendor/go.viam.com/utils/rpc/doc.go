/*
Package rpc provides a remote procedure call (RPC) library based on gRPC.

In a server context, this package should be preferred over gRPC directly
since it provides higher level configuration with more features built in,
such as grpc-web, gRPC via RESTful JSON, and gRPC via WebRTC.

WebRTC services gRPC over DataChannels. The work was initially adapted from
https://github.com/jsmouret/grpc-over-webrtc.

# Connection

All connections to RPC servers are done by way of the Dial method which will try the
following mechanisms to connect to a target server:

1. mDNS (direct/WebRTC)
2. WebRTC
3. Direct gRPC

By default it will try to connect with mDNS (1) and WebRTC (2) in parallel and use the
first established connection. If both fail then it will try to connection with Direct
gRPC. This ordering can be modified by disabling some of these methods with DialOptions.

# Direct gRPC

This is the simplest form of connection and for the most part passes straight through to the gRPC
libraries.

# WebRTC

This is the most complex form of connection. A WebRTC connection is established by way of a provided
WebRTC signaling server that exchanges connection information about the client and server. This exchange
has the client and server act as peers and connects them in the best way possible via ICE. If this succeeds
a DataChannel is established that we use to tunnel over gRPC method calls
(mimicking the gRPC over HTTP2 specification (https://github.com/grpc/grpc/blob/master/doc/PROTOCOL-HTTP2.md)).
Using this is a powerful means of connection because it exposes ContextPeerConnection which makes it possible
to use gRPC methods to modify the Video/Audio part of the connection.

Multicast DNS (mDNS)

By default, a server will broadcast its ability to be connected to over gRPC/WebRTC over mDNS. When a dial
target that matches an instance name provided by the WithInstanceNames ServerOption is found via mDNS, it
will convey information about its internal gRPC IP address so that a connection can be made to it directly,
either via gRPC or WebRTC. This is powerful in situations where clients/servers can only communicate in
a local direct or a faster connection can be made without going out to the internet. Not that the broadcasted
address is that of the internal gRPC listener. That behavior can be changed via the WithExternalListenerAddress
ServerOption. That means, if the WithInternalTLSConfig option is used, any mDNS connections made will
request client certificates. This will not negatively affect any UI hosted (peer certificate prompt) that uses
the server's GRPCHandler because since that handler will only use the tls.Config of the http.Server hosting it,
separate from the internal one.

# Authentication Modes

Authentication into gRPC works by configuring a server with a series of authentication handlers provided
by this framework. When one authentication handler is enabled, all requests must be authenticated, except
for the Authenticate method itself.
Each handler is associated with a type of credentials to handle (rpc.CredentialsType) and an rpc.AuthHandler
which has a single method called Authenticate. Authenticate is responsible for taking the name
of an entity and a payload that proves the caller is allowed to assume the role of that entity. It returns
metadata about the entity (e.g. an email, a user ID, etc.). The framework then
returns a JWT to the client to use in subsequent requests. On those subsequent requests, the JWT
is included in an HTTP header called Authorization with a value of Bearer <token>. The framework then
intercepts all calls and ensures that there is a JWT present in the header and is cryptographically valid.
An optionally supplied TokenVerificationKeyProvider associated with the credential type can be used to provide
a key used to verify the JWT if it was not signed by the RPC service provider. Once verified an optionally supplied
EntityDataLoader associated with the credential type can use the JWT metadata to produce application to produce
data for the entity to be accessible via rpc.MustContextAuthEntity.

Additionally, authentication via mutual TLS is supported by way of the WithTLSAuthHandler and
WithInternalTLSConfig ServerOptions. Using these two options in tandem will ask clients connecting
to present a client certificate, which will be verified. This verified certificate is then caught by
the authentication middleware before JWT presence is checked. If any of the DNS names in the verified
client certificate match that of the entities checked in WithTLSAuthHandler, then the request will be
allowed to proceed.

For WebRTC, we assume that signaling is implemented as an authenticated/authorized service and for now,
do not pass any JWTs over the WebRTC data channels that are established.

There is an additional feature, called AuthenticateTo provided by the ExternalAuthService which allows
for external authentication on another server to "authenticate to" an entity on the target being
connected to. This is an extension that can be used via the WithAuthenticateToHandler ServerOption.
When a client is connecting, it will first connect to the external auth server with credentials intended
for that server, and then it will call AuthenticateTo to get a JWT destined for the actual target being
connected to. AuthenticateTo requires an entity to authenticate as. You can think of this feature as
the ability to assume the role of another entity.

Expiration of JWTs is not yet handled/support.

# Authorization Modes

Authorization is strictly not handled by this framework. It's up to your registered services/methods
to handle authorization.
*/
package rpc
