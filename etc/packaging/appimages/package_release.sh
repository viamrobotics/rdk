#!/bin/bash
set -e

# This is a helper script to determine if the current git commit constitutes a new release.
# Specifically, is it tagged in the format "v1.2.3" and a higher version than any other tags.
# It then creates stable and v1.2.3 appimages.

CUR_TAG=`git tag --points-at`
if [[ $CUR_TAG =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]
then
	NEWEST_TAG=`git tag -l "v*.*.*" | sort -Vr | head -n1`
	if [[ $CUR_TAG == $NEWEST_TAG ]]
	then
		sed -E "s/([- ])latest/\1stable/g" viam-server-latest-`uname -m`.yml > viam-server-stable-`uname -m`.yml
		sed -E "s/([- ])latest/\1$CUR_TAG/g" viam-server-latest-`uname -m`.yml > viam-server-$CUR_TAG-`uname -m`.yml
		appimage-builder --recipe viam-server-stable-`uname -m`.yml
		appimage-builder --recipe viam-server-$CUR_TAG-`uname -m`.yml
		rm viam-server-stable-`uname -m`.yml viam-server-$CUR_TAG-`uname -m`.yml
	fi
fi
