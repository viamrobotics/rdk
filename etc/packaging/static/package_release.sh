#!/bin/bash

set -e
SELF=$(realpath $0)
source "$(dirname $SELF)/../../utils.sh"

if get_version_tag > /dev/null
then
	BIN_SRC="$(dirname $SELF)/../../../bin/$(uname -s)-$(uname -m)"
	mkdir -p deploy
	cp "${BIN_SRC}/viam-server" "$(dirname $SELF)/deploy/viam-server-$(get_version_tag)-$(uname -m)"
	cp "${BIN_SRC}/viam-server" "$(dirname $SELF)/deploy/viam-server-stable-$(uname -m)"
fi
