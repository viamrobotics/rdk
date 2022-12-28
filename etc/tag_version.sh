#!/bin/bash

set -e
SELF=$(realpath $0)
source "$(dirname $SELF)/utils.sh"
fn_name="get_version_tag"

if declare -F "$fn_name" > /dev/null
then
	echo $($fn_name)
fi
exit 0
