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

### Development
Building (you must [install go](https://go.dev/doc/install) first):
```sh
go build -o ~/go/bin/viam cli/viam/main.go
```
