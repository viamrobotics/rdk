# MySensor

This example demonstrates a user defining a new sensor model.


## How to add custom models

Models can be added to the viam-server by creating a struct that implements the interface of the subtype we want to register.
For example, if we want to create a sensor, we would want to make sure that we implement the Sensor interface, which includes the method `Readings` and also `DoCommand` from the `generic.Generic` interface.

```
    type Sensor interface {
        // Readings return data specific to the type of sensor and can be of any type.
        Readings(ctx context.Context) (map[string]interface{}, error)
        generic.Generic
    }
```

To make sure the struct we create implements the interface, we can check it using the go linter like this.
```
    var _ = sensor.Sensor(&mySensor{})
```

The model then has to be registered.

```
    // registering the component model on init is how we make sure the new model is picked up and usable.
    func init() {
        registry.RegisterComponent(
            sensor.Subtype,
            "mySensor",
            registry.Component{Constructor: func(
                ctx context.Context,
                deps registry.Dependencies,
                config config.Component,
                logger golog.Logger,
            ) (interface{}, error) {
                return newSensor(config.Name), nil
            }})
    }
```

After registering the model in the init function, we also have to make sure the viam server is importing it.
```
	// import the custom sensor package to register it properly
	_ "go.viam.com/rdk/examples/mysensor/mysensor"
```

The `server` package can now be built then run (`go build -o main server/server.go` then `./main`) or run directly (`go run server/server.go`)
## Running the example

* Run the server implementing a new sensor `go run server/server.go`. Alternatively, you can build it by `go build server/server.go`.
* Run the client `go run client/client.go`.
