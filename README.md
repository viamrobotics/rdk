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

* Install `make`.
* Run `make setup` to install a full dev environment.
  * Note that on Raspberry Pi, Nvidia Jetson, etc. only a minimal environment is installed.

### First time run

* Try out `go run web/cmd/server/main.go robots/configs/fake.json` and visit http://localhost:8080

## Development

### Conventions
* Write tests!
* Work in your own fork, not a fork of the company repository.
* Follow this [Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)
* Always run `make lint` and test before pushing. `make build` should be run if `control.js` or proto files have changed. `make setup` should be run if any dependencies have changed, but does not need to be run otherwise.
* If `control.js`, `webappindex.html` or proto files have changed, double check the UI still works through the instructions in [here](#first-time-run)
* Usually merge and squash your PRs and more rarely do merge commits with each commit being a logical unit of work.
* If you add a new package, please add it to this README.
* If you add a new sample or command, please add it to this README.
* Experiments should go in samples or any subdirectory with /samples/ in it. As "good" pieces get abstracted, put into a real package command directory.
* Use imperative mood for commits (see [Git Documentation](https://git.kernel.org/pub/scm/git/git.git/tree/Documentation/SubmittingPatches?id=a5828ae6b52137b913b978e16cd2334482eb4c1f#n136)).
* Try to avoid large merges unless you're really doing a big merge. Try to rebase (e.g. `git pull --rebase`).
* Delete any non-release branches ASAP when done, or use a personal fork
* Prefer metric SI prefixes where possible (e.g. millis) https://www.nist.gov/pml/weights-and-measures/metric-si-prefixes. The type of measurement (e.g. meters) is not necessary if it is implied (e.g. rulerLengthMillis).

### Getting Started
* Fork the main repository to your own account and create feature branch(es) there for any code you want to submit. Unless you have specific reasons, do not create branches on the company repository.
* If you haven't already done so, run `make setup` to install development environment tools.
  * This is somewhat optional. See "Canon Tooling" below.
* After making your changes, make sure to rebuild/lint/test before preparing to submit a Pull Request. Run `make clean-all build lint test` as a final check to build and test "from scratch."
  * Note that building and linting will often modify files or even create new ones. *Always* check `git status` to see if there are any automated modifications you need to commit. Only if this step passes without any modifications will your PR pass testing.
* When ready, submit a PR from your branch against the "main" branch of the company repo.
* Automated workflows in GitHub run tests against all PRs, and this will begin automatically on PR submission.
* If you need to modify the code in your PR after submitting it, simply push your changes to your (source) branch and testing will automatically re-run, canceling any previously in-progress tests. There is no need to open a new PR or close your existing one.
* If you'd like an AppImage (the distributable binary "viam-server") built for your PR, add the "appimage" label to the PR, and you'll get a notice/link when it's ready. You can use this in place of the normal viam-server binary to test your changes on real hardware.
  * This AppImage will also be set up to self-update to the same PR "build channel" so future pushes/builds on the PR require only an auto-update of the viam-server on any test machines.
* Look over the file changes yourself, and make sure you've left no debugging or other unneeded statements behind.
* When ready, add one or more reviewers to your PR (top left of the main PR page.) All reviewers added must approve before merging is allowed.
* If reviewers request changes, or make comments, please work with them to resolve things, make any agreed upon changes and push them. When again ready, re-request review from your reviewer(s).
* On full approval, and with all tests passing, you can merge. Best practice is to do a "squash and merge" and generally remove/clear the _body_ of the commit message, leaving only the title/header for it (which defaults to the title of the PR itself.)
  * Make sure the commit message header is in imperative mood (see Conventions above.)
* After merging, the PR will be automatically closed, and you can use the link to delete the now-merged source branch if you like.
* Also after merging, tests will run directly against the main branch as a final caution, and if successful, new AppImages will be built for the "latest" build channel of viam server.
* Note that after merging (or otherwise closing a PR) the build channel for the PR's AppImages will be deleted. Any in-use copies of the PR-specific viam-server will need to be replaced with a normal ("latest") build (which should now include your newly merged features.)

### Troubleshooting `make setup`

#### Setup is hanging on brew update
If while running setup, you see brew hanging on update, you may be getting rate limited by GitHub while brew is inspecting taps via the GitHub public API. In this case, the only reasonable solution is to use a GitHub PAT (Personal Access Token) for brew. To do so, create a PAT with no scoped permissions following https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token. Then, in your `~/.bash_profile` or `~/.zprofile`, add `export HOMEBREW_GITHUB_API_TOKEN=yourpat` and try again.

### Canon Tooling
* To provide a consistent build environment, the same "canonical" docker images used by automated testing and building are available to use on your desktop environment as well.
* To get started, make sure you have docker installed and working. https://docs.docker.com/get-docker/
  * On Mac, check settings>>experimental and enable the new virt framework to get a speed boost. Though emulation of Linux under Mac will be slower than native. (But it can still be faster than attempting to build on a Raspberry Pi or other SBC that requires linux.)
  * Also check the resource limits and make sure to allocate enough. Full builds can take 4GB or more of memory, and the more cpu the better.
* The main entrypoint is to run `make canon-shell` which will drop you into an interactive bash shell where your working rdk directory is mounted as /host
  * There are also `make canon-shell-arm64` and `make canon-shell-amd64` to allow building/testing under specific architectures.
  * See canon.make for other related make targets.
* From here, you can run any development tasks you need, without installing tooling to your outside environment.
  * Ex: You can run `make build lint test` here without ever having run `make setup` and when you exit, only changes made within the rdk codebase itself will persist. Nothing will be modified in the rest of your home directory or system.
  * This also has the benefit that all tools and such will have the exact same versions as will be used during automated PR testing.

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

### Frontend

To start the client development environment, first run the same `go run` command mentioned in getting started, but with the environmental variable `ENV=development` (like: `ENV=development go run web/cmd/server/main.go robots/configs/fake.json`). Then navigate to `web/frontend` and run `npm start` in a new terminal tab.

Note that you should still visit `localhost:8080` to view the app, not `localhost:5173`. The latter is a hot module replacement server that rebuilds frontend asset changes.

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

## License
GPLv3 - See [LICENSE][https://github.com/viamrobotics/main/LICENSE] file
