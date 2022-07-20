#!/bin/bash
set -e

# This is a helper script to determine if the current git commit constitutes a new release.
# Specifically, is it tagged in the format "v1.2.3" and a higher version than any other tags.
# It then creates stable and v1.2.3 appimages.

CUR_TAG=`git tag --points-at | sort -Vr | head -n1`
if [[ $CUR_TAG =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]
then
	NEWEST_TAG=`git tag -l "v*.*.*" | sort -Vr | head -n1`
	if [[ $CUR_TAG == $NEWEST_TAG ]]
	then
		BUILD_CHANNEL=stable appimage-builder --recipe viam-server-`uname -m`.yml
		BUILD_CHANNEL=$CUR_TAG appimage-builder --recipe viam-server-`uname -m`.yml
	fi
fi
