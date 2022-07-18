#!/bin/bash

# This script will pull new canon images if the cache images are more than 24h old
# OR the main images are older than the minimum date passed as an argument ($1).

if ! docker info > /dev/null
then
	echo "Docker must be installed and running to use canon targets."
	exit 1
fi

MIN_DATE=$1

date_to_seconds(){
	if [ -z "$1" ]
	then
		echo 0
		return
	fi

	# Check if we're on Mac, which provides a horrible "date" utility.
	if date -j > /dev/null 2>&1
	then
		if [[ $1 =~ ([0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2})\.[0-9]*Z ]]
		then
			date -u -j -f %FT%T ${BASH_REMATCH[1]} +%s
		else
			echo -n 0
		fi
	else
		# Linux / GNU date
		date -d $1 +%s
	fi
}

yesterday() {
	if date -j > /dev/null 2>&1
	then
		# Mac
		date -u -j -v-1d +%s
	else
		# Linux
		date -d yesterday +%s
	fi
}

for TAG in amd64 arm64 amd64-cache arm64-cache
do
	TIMESTAMP=$(date_to_seconds `docker inspect -f '{{ .Created }}' ghcr.io/viamrobotics/canon:$TAG` )
	if ( [[ $TAG =~ -cache$ ]] && [ $TIMESTAMP -lt $(yesterday) ] ) || [ $TIMESTAMP -lt $(date_to_seconds $MIN_DATE) ]
	then
		# Always "succeed" in case network is down
		docker pull ghcr.io/viamrobotics/canon:$TAG || true
	fi
done
