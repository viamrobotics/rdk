# gRPC test server for Arduino

## Notes

* Tested on Arduino Due with an Ethernet Shield 2.0.

## Dependencies

* Run `make setup`

## Protobuf
[nanopb](https://github.com/nanopb/nanopb) was used for this specific example with the options in [./src/gen/robot.options](./src/gen/robot.options) in order to support serving a single compass with a heading. To make future changes, refer to the [simple example in nanopb](https://github.com/nanopb/nanopb/tree/master/examples/simple).

## Running

* Open the `server.ino` in the Arduino editor and upload it to your Arduino. The Arduino will print out its IP address to serial.
