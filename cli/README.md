## [EXPERIMENTAL] Viam Command Line Interface

This is an experimental feature, so things may change without notice. All feedback on the cli is greatly appreciated.


### Getting Started
Enter `viam login` and follow instructions to authenticate.

### Installation

With brew (macOS & linux amd64):
```sh
brew trust viamrobotics/brews
brew tap viamrobotics/brews
brew install viam
```
As a binary (linux amd64):
```sh
sudo curl -o /usr/local/bin/viam https://storage.googleapis.com/packages.viam.com/apps/viam-cli/viam-cli-stable-linux-amd64
sudo chmod a+rx /usr/local/bin/viam
```

As a binary (linux arm64):
```sh
sudo curl -o /usr/local/bin/viam https://storage.googleapis.com/packages.viam.com/apps/viam-cli/viam-cli-stable-linux-arm64
sudo chmod a+rx /usr/local/bin/viam
```

From source (you must [install go](https://go.dev/doc/install) first):
```sh
go install go.viam.com/rdk/cli/viam@latest
# add go binaries to your path (if you haven't already)
echo 'export PATH="$HOME/go/bin:$PATH"' >> ~/.bashrc
```

### Shell completion

The CLI can emit tab-completion scripts for your shell. Once the `viam` binary is on your `$PATH`, source the output to enable completion of commands, subcommands, and flag names:

```sh
# bash (add to ~/.bashrc)
source <(viam completion bash)

# zsh (add to ~/.zshrc)
source <(viam completion zsh)

# fish
viam completion fish > ~/.config/fish/completions/viam.fish

# PowerShell
viam completion pwsh | Out-String | Invoke-Expression
```

### Source upload metadata

`viam module reload` and `viam module build start --from-source` include a
generated `.viam-source.json` at the root of the uploaded source archive:

```json
{"git":{"revision":"0123456789abcdef0123456789abcdef01234567","modified":false}}
```

Build scripts can read this file to embed the source revision and dirty state
without uploading `.git`. `revision` is the full Git HEAD hash. `modified`
includes staged, unstaged and untracked changes under the source directory,
following Git's ignore rules. The generated metadata and upload archive are
excluded from that status check. Git worktrees and source subdirectories are
supported. Avoid editing the source while it is being archived.

If Git is unavailable, the directory is not a repository, or it has no commit,
the record is `{"git":null}`; the upload still proceeds. Treat this as unknown
provenance, not a clean build. This is uploader-provided metadata, not an
attestation or an automatic binary stamp.

The filename is reserved in source uploads: the CLI replaces any existing entry
in the archive with freshly collected metadata, without changing that file in
your checkout. With `--workdir`, the file remains at the source archive root.

### Development
Building (you must [install go](https://go.dev/doc/install) first):
```sh
go build -o ~/go/bin/viam cli/viam/main.go
```

Or if you have installed the CLI through homebrew:
```sh
go build -o /opt/homebrew/bin/viam cli/viam/main.go
```

Then afterwards reset your homebrew installation with:
```sh
brew unlink viam && brew link --overwrite viam
```
