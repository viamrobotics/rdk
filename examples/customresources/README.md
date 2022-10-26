# CustomResources
This example demonstrates several ways viam-server can be extended with custom resources. It contains several elements.

## Elements

### gizmoapi
Custom (component) api/protocol called "Gizmo" (acme:component:gizmo).

### mygizmo
A specific model (acme:demo:mygizmo) that implements the Gizmo API.

### summationapi
Custom (service) api/protocol called "Summation" (acme:service:summation).

### mysum
A specific model (acme:demo:mysum) that implements the Summation API. Simply adds or subtracts numbers.

### mybase
Custom component (acme:demo:mybase) that implements Viam's built-in Base API (rdk:service:base) and in turn depends on two secondary "real" motors from the parent robot (such parental dependencies only work in the module method below.)

### mynavigation
Custom service (acme:demo:mynavigation) that implements Viam's built-in Nativation API (rdk:service:navigation) and only reports a static location from its config, and allows waypoints to be added/removed. Defaults to Point Nemo.

### client
Test client that uses/tests the mygizmo, mysum, and mynavigation resources and APIs.

### server
Standalone robot server (for use as a "remote") which serves serves a mygizmo, mynavigation, and mysum.

### module
The lighter weight (and preferred method) to integrate custom components. Serves all above APIs/components/services.

### proto
This contains the protobuf files for the custom APIs. Only the .proto files are manually modified, and the rest are (re)generated from that with `make buf`.

## Running

### Remote Server
* Run the server implementing custom resources `make run-remote`.
* Run a standard server connecting to the custom resource server as a remote `make run-parent`.
* Run the client that knows about custom APIs but talks to it via the parent `make run-client`.

### Modular Resource
* Start the server with `make run-module` Note: this automatically compiles the module itself first, which can be done manually with `make module`.
* Run the client `make run-client`.
