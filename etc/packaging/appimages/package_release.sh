#!/bin/bash

set -e
SELF=$(realpath $0)
source "$(dirname $SELF)/../../utils.sh"

if get_version_tag > /dev/null
then
	CUR_TAG=$(get_version_tag)
	BUILD_CHANNEL=stable appimage-builder --recipe viam-server-`uname -m`.yml
	BUILD_CHANNEL=$CUR_TAG appimage-builder --recipe viam-server-`uname -m`.yml
fi
