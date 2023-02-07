/*
Package session provides support for robot session management.

When working with robots, we want a protocol and system-wide means to be able to understand the
presence of a client connected to a robot. This provides for safer operation scenarios when dealing
with actuating controls. Specifically, without this, controls that would be "sticky" (e.g. SetPower of a base)
based on the last input of a client, can have a robot try to continue what it was told to do forever.
These clients range from SDK scripts, input controllers, and robots talking amongst themselves.

# Summary

Part of the solution to this is session management. A session, as defined here, is a presence mechanism at
the application layer (i.e. RDK, not TCP) maintained by a client (e.g. SDK) with a server (e.g. RDK).
Since the client maintains the session, it is responsible for telling the server it is still present
every so often; this will be called staying within the heartbeat window. The client must send at least one
session heartbeat within this window. As soon as the window lapses/expires, the server will safely stop all resources
that are marked for safety monitoring that have been last used by that session, and no others; a lapsed client
will attempt to establish a new session immediately prior to the next operation it performs.

# Goals of session management

  - Session management is opt-in from a protocol perspective

  - If an SDK doesn't start/maintain a session and it disconnects, the server will not Stop anything in
    response to that.

  - Once implemented in an SDK, session management is opt-out from the client usage perspective.

  - Users will need to provide options to disable it in order to acknowledge the safety risk

  - Sessions are only maintained between a client and server.

  - Sessions are bound to metadata, not any kind of TCP connection, in order to support many ways to maintain
    a session.

  - The heartbeat window has a sensible default of 2s but is user configurable between 10ms and 1min.

  - Authenticated users can only use the sessions they create and no others.

  - Remotes will respond back to parents with resources that were safety monitored in the request such
    that they can be stopped if the parent session expires.

  - The remotes will not know about the parent session and its details.

  - Adding a new component/service makes it easier to describe which of its gRPC methods are safety monitored.

# API

The API for session management is supported by four parts in Protobuf/gRPC

 1. The "RobotService" exposes a StartSession and SendSessionHeartbeat in order to facilitate the construction
    and maintenance of a session. (see [robot.proto])
 2. All methods in gRPC will accept a "viam-sid" session ID metadata header to indicate that the method to be invoked is
    bound to a session and may be safety monitored depending on the resource associated with the request.
 3. All methods in gRPC will be able to opt into safety monitoring via the common.v1.safety_heartbeat_monitored
    extension boolean. (see [common.proto] and [base.proto])
 4. All responses to methods in gRPC may return a metadata header of "viam-smrn" to indicate a list of resources
    that should be safety monitored in the event the caller has its own delegated session management.

# SDK Client Implementation Notes

This assumes that the SDK has a concept of a gRPC Transport (either Direct or WebRTC based) with client interceptor
support.

The interceptor should be split up into two parts, the session heatbeater/manager and session interceptor.

# Client Session Heartbeater/Manager

The manager should implement a session metadata method that returns the current session ID for all methods except:

	/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo
	/proto.rpc.webrtc.v1.SignalingService/Call
	/proto.rpc.webrtc.v1.SignalingService/CallUpdate
	/proto.rpc.webrtc.v1.SignalingService/OptionalWebRTCConfig
	/proto.rpc.v1.AuthService/Authenticate
	/viam.robot.v1.RobotService/ResourceNames
	/viam.robot.v1.RobotService/ResourceRPCSubtypes
	/viam.robot.v1.RobotService/StartSession
	/viam.robot.v1.RobotService/SendSessionHeartbeat

If the current session ID does not exist, the method should StartSession and start a background heartbeater that
runs at an interval of StartSession's response heartbeat_window at a factor of 5. The heartbeater itself should
send SendSessionHeartbeat requests at that interval. If the heartbeat request ever fails due to an Unavailable gRPC code,
the session ID should be unset and the background routine stopped in order to signal to the main thread that a new session is required.
Once a new session is required, this process will restart.

# Client Session Interceptor

The interceptor should call the manager's session metadata method and use the return value to fill the "viam-sid" metadata header.
If the invoked gRPC call fails with code "InvalidArgument" and message "SESSION_EXPIRED", the session should be reset and
the request should be retried. If it is too difficult to support retry functionality (particularly for streams that would
need to buffer messages), then it is okay to leave out and propagate the "SESSION_EXPIRED" error.

Resetting a session means to simply start a new session on the next valid intercepted call. In the event of a disconnection,
the heartbeater stops as mentioned above but it is possible that the session is still alive on the server. In that case,
it would be nice to preserve that session so the client should attempt to start a session again with the resume field
set to the last session. This will end up heartbeating it if it exists; if it does not exist, a new session must be returned.

# Client Session Options

By default, this feature is opt-in, but you must provide a client option to disable session heartbeating.

# Client Session Implementations

Starting points:
  - [Typescript Client]
  - [golang Client]

# SDK Server Implementation Notes

This assumes that the server has a concept of gRPC server interceptors.

The interceptor should be split up into two parts, the session heatbeater/manager and session interceptor.

# Server Session Manager

The session manager is responsible for keeping track of all sessions in addition to which session was the last
to be associated with a safety monitored resource. It should check session expiration on an interval less than
or equal to the minimum heartbeat window (e.g. 1ms). When a session expires, it should Stop all resources it
was associated with having their safety monitored methods called.

# Server Session Interceptor

The interceptor should exempt the following methods:

	/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo
	/proto.rpc.webrtc.v1.SignalingService/Call
	/proto.rpc.webrtc.v1.SignalingService/CallUpdate
	/proto.rpc.webrtc.v1.SignalingService/OptionalWebRTCConfig
	/proto.rpc.v1.AuthService/Authenticate
	/viam.robot.v1.RobotService/ResourceNames
	/viam.robot.v1.RobotService/ResourceRPCSubtypes
	/viam.robot.v1.RobotService/StartSession
	/viam.robot.v1.RobotService/SendSessionHeartbeat

Before handling the method request, the interceptor must do two things:

 1. Check if the session passed in is still alive. If it is not, a gRPC error with code "InvalidArgument"
    and message "SESSION_EXPIRED" should be returned.
 2. Detect the resource being referenced and check if its proto method description has the extension boolean
    mentioned above set to true. If it does not, the method should be invoked. If it does, the resource should
    be associated with the current session. If there is no current session, the resource should have its
    last associated session cleared in the manager.

Additionally, where possible, the interceptor should pass to the invoked handler, access to the session manager to associate
resources indirectly accessed that need to be safety monitored. For example, if invoking an input controller method
via gRPC, the implementation of the controller needs to tell the manager what resources it's about to control. In
the go implementation, we provide a session.SafetyMonitor method that accepts a request context and a resource name
to monitor.

Note: When gRPC server streaming is used, the safety monitored resources must be returned before the first
response is sent out. This is due to limitations in not being able to send response headers during a gRPC
response.

# Server Session Implementations

Starting points:
  - [golang Server]

# Remote Robot Considerations

When connecting to a remote robot, the underlying client will maintain its own, single, session that is
separated from the session of some end user calling a method into the main robot. Because of how the interceptor
works, we will still mark a remote resource name as being safety monitored. Furthermore, the remotely invoked method
can return metadata for the caller to associate up into the session that ultimately invoked it. This provides for
two facets of safety. If the end user disconnects, its session will expire and the safety monitored resources it
accessed last (if no one else accessed them) will be stopped. If the robot disconnects or crashes from the remote,
then the remote robot will have the remote session be expired and also terminate all resources that the connecting
robot had accessed last in the same vein.

# Security Considerations

  - Since the loss of a session can result in stopping moves to components, which we would consider an authorized
    action, then sessions must be associated with the authorization subject, if present. That means if there two
    subjects, Alice and Bob, and Alice starts Session 1 (S1) and Bob Session 2 (S2), then it must be forbidden that
    Bob can send heartbeats and attach session metadata for Alice's session (and vice versa).

[robot.proto]: https://github.com/viamrobotics/api/blob/d06de64f8202ca3151beea3285dcff8fe2c2df81/proto/viam/robot/v1/robot.proto#L97
[common.proto]: https://github.com/viamrobotics/api/blob/d06de64f8202ca3151beea3285dcff8fe2c2df81/proto/viam/common/v1/common.proto#L154
[base.proto]: https://github.com/viamrobotics/api/blob/d06de64f8202ca3151beea3285dcff8fe2c2df81/proto/viam/component/base/v1/base.proto#L36
[Typescript Client]: https://github.com/viamrobotics/viam-typescript-sdk/blob/04bcf3248c5a6a35653d4ddc777e2caf2965893d/src/SessionTransport.ts
[golang Client]: https://github.com/viamrobotics/rdk/blob/f209d34564d51e5b70e8e87f942ff43890ece200/robot/client/client.go#L218
[golang Server]: https://github.com/viamrobotics/rdk/blob/f209d34564d51e5b70e8e87f942ff43890ece200/robot/web/web.go#L727
*/
package session
