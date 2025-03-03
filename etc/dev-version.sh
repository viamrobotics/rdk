#!/bin/bash

# Get the most recent tag
LAST_TAG=$(git describe --tags --abbrev=0 2>/dev/null)

# Count commits since last tag
COMMITS_SINCE_TAG=$(git rev-list "${LAST_TAG}..HEAD" --count 2>/dev/null)

# Remove 'v' prefix from tag
BASE_VERSION=${LAST_TAG#v}

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
