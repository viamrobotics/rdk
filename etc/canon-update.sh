#!/bin/bash

# This script will pull new canon images if the cache images are more than 24h old
# OR the main images are older than the minimum date passed as an argument ($1).

if ! docker info > /dev/null
then
	echo "Docker must be installed and running to use canon targets."
	exit 1
fi

if [ -z "$1" ]
then
	MIN_DATE="@0"
else
	MIN_DATE=$1
fi

for TAG in amd64 arm64 amd64-cache arm64-cache
do
	TIMESTAMP=$(date +%s -d `docker inspect -f '{{ .Created }}' ghcr.io/viamrobotics/canon:$TAG || echo @0`)
	if ( [[ $TAG =~ -cache$ ]] && [ $TIMESTAMP -lt $(date +%s -d yesterday) ] ) || [ $TIMESTAMP -lt $(date +%s -d "$MIN_DATE") ]
	then
		docker pull -a ghcr.io/viamrobotics/canon
		break
	fi
done



