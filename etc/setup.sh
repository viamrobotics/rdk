#!/bin/bash

set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
ROOT_DIR="$DIR/../"

GO_PATH=$(which go)
if [ ! -f $GO_PATH ]; then
	echo "You need to install golang"
fi

if [ "$(uname)" = "Linux" ]; then
	sudo apt install python2.7-dev libvpx-dev libx264-dev
fi

if [ "$(uname)" = "Darwin" ]; then
	brew install libvpx x264 pkgconfig
	make python-macos
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
