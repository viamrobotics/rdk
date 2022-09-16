# RDK (Robot Development Kit)

<p align="center">
  <a href="https://go.viam.com/pkg/go.viam.com/rdk/"><img src="https://pkg.go.dev/badge/go.viam.com/rdk" alt="PkgGoDev"></a>
</p>

Viam provides an open source robot architecture that provides robotics functionality via simple APIs
Website: [viam.com](https://www.viam.com)
Documentation: [docs.viam.com](https://docs.viam.com)
Cloud App: [app.viam.com](https://app.viam.com)


## Building and Using

### Dependencies

* Install `make`.
* Run `make setup` to install dev environment requiements.

### Build and Run
* Build: `make server`. Then run `./<your architecture>/server [parameters]`
* Run without building: `go run web/cmd/server/main.go [parameters]`

Example with a dummy configuration: `go run web/cmd/server/main.go -config etc/configs/fake.json`. Then visit http://localhost:8080 to access remote control.

### SDKs

Multiple SDKs are available for writing client applications that interface with the Viam RDK.

Go: Provided by this repository (see https://github.com/viamrobotics/rdk/robot/client)
Python: https://github.com/viamrobotics/viam-python-sdk
Rust: https://github.com/viamrobotics/viam-rust-sdk

## Development

Sign the Contribution Agreement before submitting pull requests.

### API
The API is defined with Protocol Buffers/gRPC which can be found at https://github.com/viamrobotics/api.

### Conventions

* Write tests!
* Follow this [Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md).
* Run `make lint` and `make test`.
* Run `make build` and commit resulting frontend artifacts.
* Use imperative mood for commits (see [Git Documentation](https://git.kernel.org/pub/scm/git/git.git/tree/Documentation/SubmittingPatches?id=a5828ae6b52137b913b978e16cd2334482eb4c1f#n136)).
* Prefer metric SI prefixes where possible (e.g. millis) https://www.nist.gov/pml/weights-and-measures/metric-si-prefixes. The type of measurement (e.g. meters) is not necessary if it is implied (e.g. rulerLengthMillis).

### Frontend

To start the client development environment, first run the same `go run` command mentioned in Building and Using, but with the environmental variable `ENV=development` (e.g. `ENV=development go run web/cmd/server/main.go -config etc/configs/fake.json`). 

Then navigate to `web/frontend` and run `npm start` in a new terminal tab. Visit `localhost:8080` to view the app, not `localhost:5173`. The latter is a hot module replacement server that rebuilds frontend asset changes.

## Contact

Slack: https://viamrobotics.slack.com
Support: https://support.viam.com

## License
Copyright 2021-2022 Viam Inc.

AGPLv3 - See [LICENSE](https://github.com/viamrobotics/rdk/blob/main/LICENSE) file
