# MySensor

This example demonstrates a user defining a new sensor model.


## How to add custom models

Models can be added to the viam-server by creating a struct that implements the interface of the subtype we want to register.
For example, if we want to create a sensor, we would want to make sure that we implement the Sensor interface, which includes the method `Readings` and also `DoCommand` from the `generic.Generic` interface.
The `generic.Generic` interface allows you to add arbitrary commands with arbitrary input arguments and return messages, which is useful if you want to extend the functionality of a model implementation beyond the subtype interface.

```
    type Sensor interface {
        // Readings return data specific to the type of sensor and can be of any type.
        Readings(ctx context.Context) (map[string]interface{}, error)
        generic.Generic
    }
```

To make sure the struct we create implements the interface, we can check it using the go linter inside the go package like this.
```
    var _ = sensor.Sensor(&mySensor{})
```

The model then has to be registered through an init function, which should live in the package implementing the new model.
Init functions are run on import, so we have to make sure we are importing it somewhere in our code!

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

In this case, we would want to import it somewhere in the server code. A good place to put this import is the top of the server file.
```
	// import the custom sensor package to register it properly
	_ "go.viam.com/rdk/examples/mysensor/mysensor"
```

The `server` package can now be built then run (`go build -o main server/server.go` then `./main`) or run directly (`go run server/server.go`)

Check the custom sensor code out [here](https://github.com/viamrobotics/rdk/blob/main/examples/mysensor/mysensor/mysensor.go), the server code [here](https://github.com/viamrobotics/rdk/blob/main/examples/mysensor/server/server.go), and a simple client [here](https://github.com/viamrobotics/rdk/blob/main/examples/mysensor/client/client.go).
## Running the example

* Run the server implementing a new sensor `go run server/server.go`. Alternatively, you can build it by `go build server/server.go`.
* Run the client `go run client/client.go`.
