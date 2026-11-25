# Modules: meta.json, platforms and versions

A module is a program that adds models (components or services) to a machine. Module work is CLI only: no MCP tool generates, builds or uploads one. Every `viam module` command reads `./meta.json` unless `--module=<path>` names another.

## Commands

- `viam module generate` scaffolds a module. Run without a terminal, it needs `--name`, `--language` (`python`, `go` or `cpp`), `--visibility`, `--public-namespace`, `--resource-subtype` and `--model-name`, or it stops to prompt. `--register` also creates the module in the registry.
- `viam module create --name=<name>` registers an existing directory and writes `meta.json`; `--local-only` writes the file without registering.
- `viam module add-model` adds another model to an existing module.
- `viam module lint-manifest` checks `meta.json` offline. Passing does not guarantee an upload succeeds.
- `viam module update-models` rewrites `models` from the code (see below).
- `viam module update` pushes `meta.json` to the registry: description, visibility, models and links. Editing the file alone changes nothing in the registry.
- `viam module reload`, `reload-local` and `restart` run the module on a machine (see Testing).
- `viam module build start` and `viam module upload` publish a version (see Publishing).
- `viam module versions --id=<namespace:name>` lists published versions and their platforms; `viam module download` fetches one.

## meta.json

Required: `module_id`, `visibility`, `description`, `models`, `entrypoint`.

- `module_id` is `<namespace or org id>:<module name>`. The name cannot change after the module is created.
- `visibility` is `private`, `public` or `public_unlisted`.
- `url` is the repo. A cloud build from a git ref clones it, so for that it must be a public git repo.
- `entrypoint` is the executable's path inside the uploaded archive, relative to the archive root, not to the source directory. Upload refuses an archive with no executable file there.
- `first_run` is an optional setup script that runs on first install and after each version change, before the entrypoint. A non-zero exit stops the module from starting, and it times out after 1 hour.
- Each `models` entry has `api` (such as `rdk:component:sensor`), `model` (`<namespace>:<family>:<name>`, with the namespace matching the org's public namespace), and optional `short_description` (100 characters at most) and `markdown_link`. A model that serves several APIs lists them all in `apis`, and `api` must be one of them.
- `markdown_link` at the top level is the registry README, such as `README.md`; a model's `markdown_link` can point at a section, such as `README.md#my-sensor`.
- `build` is needed for cloud builds and for `reload`: `build` is the shell command (default `make module.tar.gz`), `path` the tarball it makes (default `module.tar.gz`), `arch` the platforms a cloud build makes (default `linux/amd64` and `linux/arm64`), with optional `setup`, `distro` and `darwin_deps`.

`update-models` runs the module binary (`--binary`, or `entrypoint` when left out), asks it for its models and replaces `models` with the answer. The binary must run on the current computer, so a cross-compiled build for another platform fails. It also sets each model's `markdown_link` when a `<namespace>_<family>_<name>.md` file sits next to `meta.json`. The registry sees the change only after `viam module update`.

## Platforms

An upload's `--platform` takes one value, so each platform is a separate upload; the same code gets the same version on every platform.

| Platform | Use for |
|---|---|
| `linux/amd64`, `linux/arm64` | Most compiled modules. `linux/arm64` covers a 64-bit Raspberry Pi and Jetson. |
| `linux/arm32v7`, `linux/arm32v6` | 32-bit Raspberry Pi OS. Cloud build does not make these. |
| `darwin/arm64`, `darwin/amd64` | Apple silicon and Intel Macs. Cloud build does not make `darwin/amd64`. |
| `any` | Code that runs anywhere, such as most Python modules shipped as source. |
| `linux/any`, `darwin/any` | Source that needs that OS but no particular CPU. |
| `any/amd64`, `any/arm64` | Most Docker-based modules. |

- `--tags` (such as `distro:debian` or `os_version:22.04`, repeated for several) limits an upload to machines that report every tag. A machine takes the most specific upload that matches it.
- Go cross-compiles, so one computer can build every platform. Python built with PyInstaller does not, so each platform needs a build on that CPU or a cloud build.

## Testing on a machine

- `viam module reload --part-id=<part>` builds in the cloud and the machine downloads the result; it needs `build` in `meta.json`.
- `viam module reload-local --part-id=<part>` builds on the current computer and copies the tarball to the machine, so the build must match the machine's platform. `--no-build` reuses the last build, `--file` sends a given tarball, and `--local` runs the entrypoint directly when the machine is the current computer.
- Both add only the module to the config; `--model-name=<model triple>` (with optional `--resource-name`) also adds a resource that uses it.
- `viam module restart --part-id=<part>` restarts a module already running.

A reloaded module is a development build that stays on that machine only. Keeping a module on a machine, or putting it on others, takes a published version.

## Publishing a version

Versions are semver 2.0 (`1.2.3`). Each version and platform pair can be uploaded once, so new code needs a new version, and a deleted version's number cannot be reused.

- `viam module build start --version=<version>` builds in the cloud from `url` at `--ref` (default `main`; `--token` for a private repo), for each `build.arch` platform or `--platforms`. `--from-source` uploads the local directory instead of cloning, and `--wait` blocks until it ends and exits non-zero on failure. Otherwise `viam module build list` and `viam module build logs --id=<build id> --wait` follow it.
- `viam module upload --version=<version> --platform=<platform> --upload=<archive, file or directory>` publishes a build made locally. `--force` skips the entrypoint check and can publish a version that does not start.
- `viam module build link-repo` connects a GitHub repo, and the `viamrobotics/build-action` GitHub Action then builds on each release. It reads an org API key from the repo's secrets.

A public module's new version reaches every machine that uses it with `"version": "latest"`.

## The module in a machine's config

```json
{
  "modules": [
    { "type": "registry", "name": "<local name>", "module_id": "<namespace>:<module name>", "version": "latest" }
  ],
  "components": [
    { "name": "<name>", "api": "rdk:component:sensor", "model": "<namespace>:<family>:<model>", "attributes": {} }
  ]
}
```

- `"version": "latest"` picks up each new version within minutes. An exact version stays until the config changes it.
- A module on the machine's own disk is `"type": "local"` with `executable_path` in place of `module_id` and `version`.
- `env` sets environment variables for the module process, and `log_level` its log level.
- Removing the module from the config deletes its data directory on the machine.

## When a module does not run

| Symptom | Cause |
|---|---|
| `exec format error` in the logs | The upload was built for a different platform than the machine. |
| The module restarts over and over | It crashes at startup. Its own log lines just before each restart give the reason. |
| The module never becomes ready | It did not start within 5 minutes, or `first_run` failed or is still running. |
| The machine keeps an old version | The `modules` entry pins an exact version, or no upload matches the machine's platform and tags. `viam module versions` lists what exists. |
| It works with `reload-local` and fails when published | Hard-coded paths, missing environment variables, or dependencies on the dev computer that the archive leaves out. |
| Upload says `meta.json` is invalid | `viam module lint-manifest` names the field. |
