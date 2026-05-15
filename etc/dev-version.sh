#!/bin/bash

# Exit with a blank if tree is dirty
if [ -n "$(git status -s)" ]; then
    exit 0
fi

# See if we have a direct tag
DIRECT_TAG=$(git tag --points-at | tr - \~ | sort -Vr | tr \~ - | head -n1)
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

# Find the highest stable tag globally (not just ancestors of HEAD).
# Release tags are cut on release branches and are not reachable from main,
# so `git describe` would otherwise report an older tag than the true latest.
LATEST_TAG=$(git tag --list 'v*' | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | sort -V | tail -n1)

BASE_VERSION=$(echo "$LATEST_TAG" | sed 's/^v//')
# rev-list A..HEAD is set-subtraction on reachable commits; it works even when
# A is not an ancestor of HEAD (i.e. tag lives on a release branch).
COMMITS_SINCE_TAG=$(git rev-list --count "${LATEST_TAG}..HEAD")
COMMIT_HASH=$(git rev-parse --short=9 HEAD)

# Calculate next version by incrementing patch number
NEXT_VERSION=$(echo "$BASE_VERSION" | awk -F. '{$3+=1}1' OFS=.)

# Set TAG_VERSION based on commits since last tag
if [ "$COMMITS_SINCE_TAG" -eq 0 ]; then
    TAG_VERSION="$BASE_VERSION"
else
    TAG_VERSION="${NEXT_VERSION}-dev.${COMMITS_SINCE_TAG}-${COMMIT_HASH}"
fi

# Set PATH_VERSION based on TAG_VERSION
PATH_VERSION="v${TAG_VERSION}"

echo ${PATH_VERSION}
