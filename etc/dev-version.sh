#!/bin/bash

# Exit with a blank if tree is dirty
if [ -n "$(git status -s)" ]; then
    exit 0
fi

# See if we have a direct tag
DIRECT_TAG=$(git tag --points-at | sort -Vr | head -n1)
if [ -n "$DIRECT_TAG" ]; then
    echo ${DIRECT_TAG}
    exit 0
fi

if [ -z "$GITHUB_REF_NAME" ]; then
    GITHUB_REF_NAME=$(git rev-parse --abbrev-ref HEAD)
fi

# If we're not on main, we have no (automated) version to create
if [ "$GITHUB_REF_NAME" != "main" ]; then
    exit 0
fi

# If we don't have a direct tag, use the most recent non-RC tag
DESC=$(git describe --tags --match="v*" --exclude="*-rc*" --long | sed 's/^v//')

BASE_VERSION=$(echo "$DESC" | cut -d'-' -f1)
COMMITS_SINCE_TAG=$(echo "$DESC" | cut -d'-' -f2)

# Calculate next version by incrementing patch number
NEXT_VERSION=$(echo "$BASE_VERSION" | awk -F. '{$3+=1}1' OFS=.)

# Set TAG_VERSION based on commits since last tag
if [ "$COMMITS_SINCE_TAG" -eq 0 ]; then
    TAG_VERSION="$BASE_VERSION"
else
    TAG_VERSION="${NEXT_VERSION}-dev.${COMMITS_SINCE_TAG}"
fi

# Set PATH_VERSION based on TAG_VERSION
PATH_VERSION="v${TAG_VERSION}"

echo ${PATH_VERSION}
