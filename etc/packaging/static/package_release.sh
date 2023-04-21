#!/bin/bash

set -e
cd $(realpath $0)
source "../../utils.sh"

if get_version_tag > /dev/null
then
	BIN_SRC="../../../bin/$(uname -s)-$(uname -m)"
	mkdir -p deploy
	cp "${BIN_SRC}/viam-server" "deploy/viam-server-$(get_version_tag)-$(uname -m)"
	cp "${BIN_SRC}/viam-server" "deploy/viam-server-stable-$(uname -m)"
fi
