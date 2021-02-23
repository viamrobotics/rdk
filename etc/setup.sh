#!/bin/bash

set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
ROOT_DIR="$DIR/../"

GO_PATH=$(which go)
if [ ! -f $GO_PATH ]; then
	echo "You need to install golang"
fi

if [ "$(uname)" = "Linux" ]; then
	sudo apt install python2.7-dev swig yasm	
	if [ ! -f "/usr/local/lib/libvpx.a" ]; then
		echo "You may need to set up libvpx (see README)"
	fi
fi

if [ "$(uname)" = "Darwin" ]; then
	brew install swig yasm libvpx
	make python-macos
fi

ENV_OK=1
echo $PKG_CONFIG_PATH | grep -q /usr/local/lib/pkgconfig || ENV_OK=0
echo $PKG_CONFIG_PATH | grep -q /usr/lib/pkgconfig || ENV_OK=0
echo $LD_LIBRARY_PATH | grep -q /usr/local/lib || ENV_OK=0
echo $LD_LIBRARY_PATH | grep -q /usr/lib || ENV_OK=0

if ((ENV_OK)) ; then
	exit 0
fi

case $(basename $SHELL) in
  bash)
    echo "You need some exports in your .bashrc"
    echo 'echo export PKG_CONFIG_PATH=/usr/local/lib/pkgconfig:/usr/lib/pkgconfig:$PKG_CONFIG_PATH >> ~/.bashrc'
	echo 'echo export LD_LIBRARY_PATH=/usr/local/lib:/usr/lib:$LD_LIBRARY_PATH  >> ~/.bashrc'
    ;;

  zsh)
    echo "You need some exports in your .zshrc"
    echo 'echo export PKG_CONFIG_PATH=/usr/local/lib/pkgconfig:/usr/lib/pkgconfig:$PKG_CONFIG_PATH >> ~/.zshrc'
	echo 'echo export LD_LIBRARY_PATH=/usr/local/lib:/usr/lib:$LD_LIBRARY_PATH  >> ~/.zshrc'
    ;;
  *)
    ;;
esac
