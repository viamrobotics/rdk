# RDK (Robot Development Kit)

<p align="center">
  <a href="https://go.viam.com/pkg/go.viam.com/rdk/"><img src="https://pkg.go.dev/badge/go.viam.com/rdk" alt="PkgGoDev"></a>
</p>

* [Programs](#programs)
* [Dependencies](#dependencies)
* [Development](#development)

### API Documentation & more devices
To see more examples, check out the [Wiki](https://github.com/viamrobotics/rdk/wiki)

## Dependencies

* Run `make setup` or `etc/setup.sh` (if make is not yet installed) to install a full dev environment.
  * Note that on Raspberry Pi, Nvidia Jetson, etc. only a minimal environment is installed.

### First time run

* Try out `go run web/cmd/server/main.go robots/configs/fake.json` and visit http://localhost:8080

## Development

### Conventions
* Write tests!
* Follow this [Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)
* Always run `make lint` and test before pushing. `make build` should be run if `control.js` or proto files have changed. `make setup` should be run if any dependencies have changed (run it once per PR to check), but does not need to be run otherwise.
* If `control.js`, `webappindex.html` or proto files have changed, double check the UI still works through the instructions in [here](#first-time-run)
* Usually merge and squash your PRs and more rarely do merge commits with each commit being a logical unit of work.
* If you add a new package, please add it to this README.
* If you add a new sample or command, please add it to this README.
* Experiments should go in samples or any subdirectory with /samples/ in it. As "good" pieces get abstracted, put into a real package command directory.
* Use imperative mood for commits (see [Git Documentation](https://git.kernel.org/pub/scm/git/git.git/tree/Documentation/SubmittingPatches?id=a5828ae6b52137b913b978e16cd2334482eb4c1f#n136)).
* Try to avoid large merges unless you're really doing a big merge. Try to rebase (e.g. `git pull --rebase`).
* Delete any non-release branches ASAP when done, or use a personal fork
* Prefer metric SI prefixes where possible (e.g. millis) https://www.nist.gov/pml/weights-and-measures/metric-si-prefixes. The type of measurement (e.g. meters) is not necessary if it is implied (e.g. rulerLengthMillis).

### Resources

All resources implemented within the RDK follow the pattern of registering themselves within an `func init()` block. This requires the package they are implemented in be imported, but typically not explicitly used. The place where we currently put blank imports (`_ "pkgpath"`) is in the corresponding resource's register package.

### Protocol Buffers/gRPC

For API intercommunication, we use Protocol Buffers to serialize data and gRPC to communicate it. For more information on both technologies, see https://developers.google.com/protocol-buffers and https://grpc.io/.

Some guidelines on using these:
1. Follow the [Protobuf style guide](https://docs.buf.build/style-guide/).
1. After making changes to a `.proto` file, make sure to run `make buf` to generate new files. Make sure `protoc-gen-go-grpc` and `protoc-gen-go`, usually located in `~/go/bin`, are in your `$PATH`.

#### gRPC Language Samples

* [Go](./grpc) - See `client` and `server`.
* [Python](./grpc/python)
* [Java](./grpc/java)
* [C++](./grpc/cpp)

### Testing with big data

Let's assume big data is > 10KiB. This kind of data is annoying to slow to pull down with git and is typically not needed except for certain tests. In order to add large data test artifacts, you need to do the following:

```
# get ARTIFACT_GOOGLE_APPLICATION_CREDENTIALS by talking to Eliot or Eric
# export the path with the json file as an environment variable: 
export ARTIFACT_GOOGLE_APPLICATION_CREDENTIALS=/path/to/your/json/credentials
go install go.viam.com/utils/artifact/cmd/artifact
# place new artifacts in ./artifact/data
artifact push
git add .artifact
# commit the file at some point
```

General workflow:
1. Add your file of interest to the `.artifact/data` directory, wherever you want. You can even make a new folder for it.
2. `artifact push` to create an entry for it in `.artfact/tree.json`
3. `artifact pull` to download all the files that are in the `tree.json` file