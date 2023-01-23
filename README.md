# RDK (Robot Development Kit)

<p align="center">
  <a href="https://pkg.go.dev/go.viam.com/rdk"><img src="https://pkg.go.dev/badge/go.viam.com/rdk" alt="PkgGoDev"></a>
</p>

Viam provides an open source robot architecture that provides robotics functionality via simple APIs

**Website**: [viam.com](https://www.viam.com)

**Documentation**: [docs.viam.com](https://docs.viam.com)

**Cloud App**: [app.viam.com](https://app.viam.com)

## Contact

* Community Slack: [join](https://join.slack.com/t/viamrobotics/shared_invite/zt-1f5xf1qk5-TECJc1MIY1MW0d6ZCg~Wnw)
* Support: https://support.viam.com

If you have a bug or an idea, please open an issue  in our [JIRA project](https://viam.atlassian.net/).

## Building and Using

### Dependencies

* Install `make`.
* Run `make setup` to install dev environment requirements.
  * This also installs some client side git hooks.

### Build and Run
* Build: `make server`. Then run `./bin/<your architecture>/server [parameters]`
* Run without building: `go run web/cmd/server/main.go [parameters]`

Example with a dummy configuration: `go run web/cmd/server/main.go -config etc/configs/fake.json`. Then visit http://localhost:8080 to access remote control.

### Examples
* [SimpleServer](https://pkg.go.dev/go.viam.com/rdk/examples/simpleserver) - example for creating a simple custom server.
* [MySensor](https://pkg.go.dev/go.viam.com/rdk/examples/mysensor) - example for creating a custom sensor.
* [MyComponent](https://pkg.go.dev/go.viam.com/rdk/examples/mycomponent) - example for creating a custom resource subtype.

### SDKs

Multiple SDKs are available for writing client applications that interface with the Viam RDK.

* Go: Provided by this repository [here](https://github.com/viamrobotics/rdk/tree/main/robot/client). Documentation can be found [here](https://pkg.go.dev/go.viam.com/rdk/robot/client)
* Python: [Docs](https://python.viam.dev), [Repository](https://github.com/viamrobotics/viam-python-sdk)
* Rust: [Repository](https://github.com/viamrobotics/viam-rust-sdk)

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

### Committing

* Follow git's guidance on commit messages:
> Describe your changes in imperative mood, e.g. "make xyzzy do frotz"
> instead of "[This patch] makes xyzzy do frotz" or "[I] changed xyzzy
> to do frotz", as if you are giving orders to the codebase to change
> its behavior.  Try to make sure your explanation can be understood
> without external resources. Instead of giving a URL to a mailing list
> archive, summarize the relevant points of the discussion.
* Each commit has a linting pre-commit hook run if `make setup` was used. It can be skipped with `git commit --no-verify`.


### Frontend

To start the client development environment, first run the same `go run` command mentioned in Building and Using, but with the environmental variable `ENV=development` (e.g. `ENV=development go run web/cmd/server/main.go -debug -config etc/configs/fake.json`). 

Then navigate to `web/frontend` and run `npm start` in a new terminal tab. Visit `localhost:8080` to view the app, not `localhost:5173`. The latter is a hot module replacement server that rebuilds frontend asset changes.

#### Frontend against a remote host

See documentation in [Direct Remote Control](./web/cmd/directremotecontrol/main.go).

### License check

We run [LicenseFinder](https://github.com/pivotal/LicenseFinder) in CI to verify 3rd-party libraries have approved software licenses.
If you add a 3rd-party library to this project, please run `make license` to verify that it can be used.

### Windows Support (Experimental)

Windows 10 22H2 and up.

#### Development Dependencies

* bash (from https://gitforwindows.org/ is good)
* gcc (from https://www.msys2.org/ `mingw-w64-x86_64-toolchain` is good)

Support is not well tested yet.

#### Known Issues

* motion planning is not supported yet (https://viam.atlassian.net/browse/RSDK-1772).
* video streaming is not supported yet (https://viam.atlassian.net/browse/RSDK-1771). 
* rpc: ICE between local connections found via ICE mDNS appear to be flaky in the establishment phase.

## License
Copyright 2021-2022 Viam Inc.

AGPLv3 - See [LICENSE](https://github.com/viamrobotics/rdk/blob/main/LICENSE) file
