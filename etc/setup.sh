#!/bin/bash

set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
ROOT_DIR="$DIR/../"

GO_PATH=$(which go)
if [ ! -f $GO_PATH ]; then
	echo "You need to install golang"
  exit 1
fi

if [ "$(uname)" = "Linux" ]; then
	sudo apt install python2.7-dev libvpx-dev libx264-dev pkg-config
  PREFIX="/usr/local" && \
  VERSION="0.40.0" && \
    curl -sSL \
      "https://github.com/bufbuild/buf/releases/download/v${VERSION}/buf-$(uname -s)-$(uname -m).tar.gz" | \
      sudo tar -xvzf - -C "${PREFIX}" --strip-components 1
  curl -L https://github.com/grpc/grpc-web/releases/download/1.2.1/protoc-gen-grpc-web-1.2.1-linux-x86_64 --output protoc-gen-grpc-web
  chmod +x protoc-gen-grpc-web
  sudo mv protoc-gen-grpc-web /usr/local/bin/
  TEMP_DIR=$(mktemp -d 2>/dev/null || mktemp -d -t 'protoctmp')
  cd $TEMP_DIR
  curl -OL https://github.com/protocolbuffers/protobuf/releases/download/v3.15.6/protoc-3.15.6-linux-x86_64.zip
  unzip protoc-3.15.6-linux-x86_64.zip
  sudo cp bin/* /usr/local/bin
  sudo cp -R include/* /usr/local/include
  sudo chmod 755 /usr/local/bin/protoc
  cd $DIR
fi

if [ "$(uname)" = "Darwin" ]; then
  if [ ! -d "/Applications/Xcode.app" ]; then
    echo "You need to install Xcode"
    exit 1
  fi
  brew tap bufbuild/buf
  brew bundle --file=- <<-EOS
    brew "libvpx"
    brew "x264"
    brew "pkgconfig"
    brew "protobuf", args: ["ignore-dependencies", "go"]
    brew "buf"
EOS
	make python-macos
  curl -L https://github.com/grpc/grpc-web/releases/download/1.2.1/protoc-gen-grpc-web-1.2.1-darwin-x86_64 --output protoc-gen-grpc-web
  chmod +x protoc-gen-grpc-web
  sudo mv protoc-gen-grpc-web /usr/local/bin/
fi

go install google.golang.org/protobuf/cmd/protoc-gen-go \
  google.golang.org/grpc/cmd/protoc-gen-go-grpc \
  github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway \
  github.com/pseudomuto/protoc-gen-doc/cmd/protoc-gen-doc \
  github.com/fullstorydev/grpcurl/cmd/grpcurl

sudo npm install -g ts-protoc-gen

NLOPT_OK=1
pkg-config nlopt || NLOPT_OK=0
if [ $NLOPT_OK -eq 0 ] ; then
  nlopttmp=$(mktemp -d 2>/dev/null || mktemp -d -t 'nlopttmp')
  cd $nlopttmp
  curl -O https://codeload.github.com/stevengj/nlopt/tar.gz/v2.7.0 && tar xzvf v2.7.0 && cd nlopt-2.7.0
  rm -rf v2.7.0
  cmake . && make -j$(sysctl -n hw.ncpu) && sudo make install
  cd $ROOT_DIR
  rm -rf $nlopttmp
fi

GIT_SSH_REWRITE_OK=$(git config --get url.ssh://git@github.com/.insteadOf)
if [ "$GIT_SSH_REWRITE_OK" != "https://github.com/" ]; then
  git config url.ssh://git@github.com/.insteadOf https://github.com/
fi

ENV_OK=1
if [ "$(uname)" = "Linux" ]; then
  echo $PKG_CONFIG_PATH | grep -q /usr/local/lib/pkgconfig || ENV_OK=0
  echo $PKG_CONFIG_PATH | grep -q /usr/lib/pkgconfig || ENV_OK=0
fi
echo $GOPRIVATE | grep -Fq "github.com/viamrobotics/*,go.viam.com/*" || ENV_OK=0

if ((ENV_OK)) ; then
	exit 0
fi

case $(basename $SHELL) in
  bash)
    echo "You need some exports in your .bashrc"
    if [ "$(uname)" = "Linux" ]; then
      echo 'echo export PKG_CONFIG_PATH=/usr/local/lib/pkgconfig:/usr/lib/pkgconfig:$PKG_CONFIG_PATH >> ~/.bashrc'
    fi
    echo 'echo export GOPRIVATE=github.com/viamrobotics/*,go.viam.com/*  >> ~/.bashrc'
    ;;

  zsh)
    echo "You need some exports in your .zshrc"
    if [ "$(uname)" = "Linux" ]; then
      echo 'echo export PKG_CONFIG_PATH=/usr/local/lib/pkgconfig:/usr/lib/pkgconfig:$PKG_CONFIG_PATH >> ~/.zshrc'
    fi
    echo 'echo export GOPRIVATE=github.com/viamrobotics/*,go.viam.com/*  >> ~/.zshrc'
    ;;
  *)
    ;;
esac
