#!/bin/bash

set -e
SELF=$(realpath $0)
source "$(dirname $SELF)/utils.sh"

if get_version_tag > /dev/null
then
	echo -X \'go.viam.com/rdk/config.Version=$(get_version_tag)\'
fi
exit 0
