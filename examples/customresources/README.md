# CustomResources
This example demonstrates several ways viam-server can be extended with custom resources. It contains several elements.

## Elements

### gizmoapi
Custom (component) api/protocol called "Gizmo" (acme:component:gizmo).
Note that this is split into two files. The content of wrapper.go is only needed to support reconfiguration during standalone (non-modular) use.

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

## Reconfiguration
Reconfiguration should work live, especially for the modular method. Simply edit the module.json file while the server is running and save. The server should detect the changes and update accordingly. You can try adjusting the coordinates for the mynavigation service, flip the "subtract" value of the mysum service, or change the name of "arg1" in mygizmo, then reun the client to see that it's changed things.

Additionally, you can comment out the "Reconfigure()" method in either mygizmo, mynavigation, or mysum to see how reconfiguration becomes replacement. If a resource doesn't support direct reconfiguration, it will automatically be recreated with the new config and replaced instead. (The mybase component works like this by default.)
