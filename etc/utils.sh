#!/bin/bash

# This is a helper function to determine if the current git commit constitutes a new release.
# Specifically, is it tagged in the format "v1.2.3" and a higher version than any other tags.
# This is used for internal tagging and release building.
get_version_tag() {
	set -e
	CUR_TAG=`git tag --points-at | sort -Vr | head -n1`
	if [[ $CUR_TAG =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]
	then
		NEWEST_TAG=`git tag -l "v*.*.*" | sort -Vr | head -n1`
		if [[ $CUR_TAG == $NEWEST_TAG ]]
		then
			echo $CUR_TAG
			return 0
		else
			echo "latest"
			return 128
		fi
	fi
	return 1
}
