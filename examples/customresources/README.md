# CustomResources
This example demonstrates several ways RDK can be extended with custom resources. It contains several sections. Note that `make` is used throughout to help script commands. The actual commands being run should be printed as they are used. You can also look in the various "Makefile" named files throughout, to see the exact targets and what they do.

For a fully fleshed-out example of a Golang module that uses Github CI to upload to the Viam Registry, take a look at [wifi-sensor](https://github.com/viam-labs/wifi-sensor). For a list of example modules in different Viam SDKs, take a look [here](https://github.com/viamrobotics/upload-module/#example-repos).

## APIs
APIs represent new types of components or services, with a new interface definition. They consist of protobuf descriptions for the wire level protocol, matching Go interfaces, and concrete Go implementations of a gRPC client and server.

### gizmoapi
Custom (component) api called "Gizmo" (acme:component:gizmo).
Note that this is split into two files. The content of wrapper.go is only needed to support reconfiguration during standalone (non-modular) use.

### summationapi
Custom (service) api called "Summation" (acme:service:summation).

### proto
This folder contains the protobuf for the above two APIs. Only the .proto files are human modified. The rest is generated automatically by running "make" from within this directory. Note that the generation is performed using the "buf" command line tool, which itself is installed automatically as part of the make scripting. To generate protocols for other languages, other tooling or commands may be used. The key takeaway is that just the files with the .proto suffix are needed to generate the basic protobuf libraries for any given language.

## Models
Models are concrete implementations of a specific type (API) of component or service.

### mygizmo
A specific model (acme:demo:mygizmo) that implements the custom Gizmo API.

### mygizmosummer
A specific model (acme:demo:mygizmosummer) that implements the custom Gizmo API and depends on another custom API.

### mysum
A specific model (acme:demo:mysum) that implements the custom Summation API. Simply adds or subtracts numbers.

### mybase
Custom component (acme:demo:mybase) that implements Viam's built-in Base API (rdk:component:base) and in turn depends on two secondary "real" motors from the parent robot (such parental dependencies only work in modules, not as remote servers.)

### mynavigation
Custom service (acme:demo:mynavigation) that implements Viam's built-in Nativation API (rdk:service:navigation) and only reports a static location from its config, and allows waypoints to be added/removed. Defaults to Point Nemo.

## Demos
Each demo showcases an implementation of custom resources. They fall into two categories.

* One is a module, which is the newer, preferred method of implementing a custom resource. It involves building a small binary that will (after configuration) be automatically started by a parent-viam server process, and communicate with it via gRPC over a Unix (file-like) socket on the local system. All configuration is done via the single parent robot, with relevant bits being passed to the module as necessary.

* The other demo is the older, deprecated method, which is creating a standalone robot server (very similar to viam-server itself) with the new/custom component, and other non-needed parts stripped out, and then using that as a "remote" from a parent viam-server (even though it would technically run on the same local machine.) This requires two separate configs, one for the parent, and one for the custom server.

### complexmodule
This demo is centered around a custom module that supports all four of the custom models above, including both custom APIs. A client demo is also included in a sub folder.

#### Running
* Start the server `make run-module`
  * This uses module.json
  * This automatically compiles the module itself first, which can be done manually with `make module`.
* In a separate terminal, run the client with `make run-client` (or move into the client directory and simply run `make`.)

#### Notes
In the module.json config, the module is defined near the top of the file. The executable_path there is the filesystem path to the executable module file. This path can be either relative (to the working directory where the server is started) or absolute. The provided example is relative to the demo itself, and on real installations, absolute paths may be more reliable. Ex: "/usr/local/bin/custommodule"

Reconfiguration should work live. Simply edit the module.json file while the server is running and save. The server should detect the changes and update accordingly. You can try adjusting the coordinates for the mynavigation service, flip the "subtract" value of the mysum service, or change the name of "arg1" in mygizmo, then re-run the client to see that it's changed things.

Additionally, you can comment out the "Reconfigure()" method in either mygizmo, mynavigation, or mysum to see how reconfiguration becomes replacement. If a resource doesn't support direct reconfiguration, it will automatically be recreated with the new config and replaced instead.

### multiplemodules
This demo is centered around two custom modules that each supports a custom API. A client demo is also included in a sub folder.

#### Running
* Start the server `make run-module`
  * This uses module.json
  * This automatically compiles the module itself first, which can be done manually with `make module`.
* In a separate terminal, run the client with `make run-client` (or move into the client directory and simply run `make`.)

### simplemodule
This is a minimal version of a custom resource module, using the built-in Generic API, and where everything for the module is in one file. It has a simple "counter" component model included, which uses the rdk:component:generic interface. This component simply takes numbers and adds them to a running total, which can also be fetched. This also contains a client demo similar to the complex example.

#### Running
* Same steps as the complex demo above.
* Start the server `make run-module`
  * This uses module.json
  * This automatically compiles the module itself first, which can be done manually with `make module`.
* In a separate terminal, run the client with `make run-client` (or move into the client directory and simply run `make`.)

### combomodule
This demo centers on a *composite*: a single resource reachable under several co-equal APIs at once. The `combodevice` model serves `rdk:component:camera`, `rdk:component:movement_sensor`, and the custom `acme:component:gizmo` API from one identity. It is the hard case on purpose — camera and movement_sensor both declare a `Properties` method with *different* return types, so those colliding methods can't share a struct and are routed through per-API facades, while gizmo (a module-defined API) rides alongside to show a composite spans built-in and custom APIs alike. A client demo is included in a sub folder; it reaches the one `combo` resource under each API and uses the composite-aware SDK helpers `resource.NamedFromProvider` / `resource.APIsOf` to list every API the single object serves.

#### Running
* Start the server `make run-module`
  * This uses module.json
  * This automatically compiles the module itself first, which can be done manually with `make module`.
* In a separate terminal, run the client with `make run-client` (or move into the client directory and simply run `make`.)

#### Notes
In module.json the composite is a single component entry whose declared API is its canonical one (camera here); the server advertises it under every API the model is registered for, so the client sees one `combo` name under all three. The per-API lookups (`camera.FromProvider`, etc.) work against this server directly, while the api-less `resource.NamedFromProvider("combo")` handle resolves the bare composite over the SDK robot client, which depends on that client-side support being present.

### simplecombomodule
The minimal composite, and the companion to combomodule without the hard part. The `simplecombo` model serves just `rdk:component:sensor` and the custom `acme:component:gizmo` API. Because `sensor` (Readings) and `gizmo` (DoOne/DoTwo/…) share no method names, *one* struct satisfies both interfaces and there are no per-API facades at all — `resource.Compose` wraps the same object as each API's sub-resource. Start here to see the composite machinery on its own before reading combomodule's colliding-method handling.

#### Running
* Same steps as the demos above (`make run-module`, then `make run-client` in a separate terminal).
