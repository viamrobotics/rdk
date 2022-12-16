/*
Package module provides services for external resource and logic modules.

# Module Resource System Overview

The module system allows a user to build an external binary, either in Golang, using this package and any others from the RDK ecosystem,
or in any other language, provided it can properly support protobuf/grpc. The path to the binary (the module) and a name for it must
be given in the Modules section of the robot config. The normal viam-server (rdk) process will then start this binary, and query it via
GRPC for what protocols (protobuf described APIs) and models it supports. Then, any components or services that match will be handled
seamlessly by the module, including reconfiguration, shutdown, and dependency management. Modular components may depend on others from
either the parent (aka built-in resources) or other modules, and vice versa. Modular resources should behave identically to built-in
resources from a user perspective.

# Startup

The module manager (modmanager) integrates with the robot and resource manager. During startup, a dedicated GRPC module service is started,
listening on a unix socket in a temporary directory (ex: /tmp/viam-modules-893893/parent.sock) and then individual modules are executed.
These are each passed dedicated socket address of their own in the same directory, and based on the module name.
(ex: /tmp/viam-modules-893893/acme.sock) The parent then queries this address with Ready() and waits for confirmation. The ready response
also includes a HandlerMap that defines which protocols and models the module provides support for. The parent then registers these
subtypes and models, with creator functions that call the manager's AddResource() method. Once all modules are started, normal robot
loading continues.

When resources or components are attempting to load that are not built in, their creator method calls AddResource() and a request is built
and sent to the module. The entire config is sent as part of this, as are dependencies. Dependencies are passed by name only through GRPC,
and the module library on the module side automatically creates grpc clients for each resource, before calling the component/service
constructor. In this way, fully usable dependencies are provided, just as they would be during built-in resource creation.

Back on the parent side, once the AddResource() call completes, the modmanager then establishes an rpc client for the resource,
and returns that to the resource manager, which inserts it into the resource graph. For built-in protocols (arm, motor, base, etc.) this
rpc client is cast to the expected interface, and is functionally identical to a built-in component. For new protocols, the client created
is wrapped as a ForeignResource, which (along with the reflection service in the module) allows it to be used normally by external clients
that are also aware of the new protocol in question.

# Reconfiguration

Reconfiguration is handled as transparently as possible to the end user. When a resource would be reconfigured by the resource manager,
it is checked if it belongs to a module. If true, then a ReconfigureResource() request is sent to the module instead. (The existing grpc
client object on the parent side is untouched.) In the module, the receiving method attempts to cast the real resource to
registry.ReconfigurableComponent/Service. If successful, the Reconfigure() method is called on the resource. This method receives the
full new config (and dependencies) just as AddResource would. It's then up to the resource itself to reconfigure itself accordingly.
If the cast fails (e.g. the resource doesn't have the Reconfigure method.) then the existing resource is closed, and a new one created in
its place. Note that unlike built-in resources, no proxy resource is used, since the real client is in the parent, and will automatically
get the new resource, since it is looked up by name on each function call.

For removal (during shutdown) RemoveResource() is called, and only passes the resource.Name to the module.

# Shutdown

Shutdown is hooked so that during the Close() of the resource manager, resources are checked if they are modular, and if so,
RemoveResource() is called after the parent-side rpc client is closed. The grpc module service is also kept open as late as possible.
Otherwise, shutdown happens as normal, including the closing of components in topological (dependency) order.

# Module Protocol Requirements

A module can technically be built in any language, with or without support of this RDK or other Viam SDKs. From a technical point of view,
all that's required is that the module:
  - Is an executable file by unix standards. This can be a compiled binary, or a script with the proper shebang to its interpreter, such
    as python.
  - Looks at the first argument passed to it at execution, and uses that as it's grpc socket path.
  - Listens with plaintext GRPC on that socket.
  - GRPC must provide the Module service (https://github.com/viamrobotics/api/tree/main/proto/viam/module/v1/module.proto), a reflection
    service, and any APIs needed for the resources it intends to serve. Note that the "robot" service itself is NOT required.
  - Handles the Module service's calls for Ready(), and Add/Remove/ReconfigureResource()
  - Cleanly exits when sent a SIGINT or SIGTERM signal.

# Module Creation Considerations

Under Golang, the module side of things tries to use as much of the "RDK" idioms as possible. Most notably, this includes the registry. So
when creating modular components with this package, resources (and protocols) register their "Creator" methods and such during init() or
during main(). They then are explicitly added via AddModelFromRegistry() so that merely importing a module doesn't add unneeded/unused
grpc services.

In other languages, and for small modules not part of a larger code ecosystem, the registry concept may not make as much sense, and
foregoing the registry step in favor of some more direct AddModel() call (which takes the creation handler func directly) may be better.
*/
package module
