## [EXPERIMENTAL] Viam Command Line Interface

This is an experimental feature, so things may change without notice. All feedback on the cli is greatly appreciated.


### Getting Started
Enter `viam login` and follow instructions to authenticate.

### Installation

With brew (macOS & linux amd64):
```sh
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
