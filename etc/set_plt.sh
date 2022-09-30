#!/bin/bash

set -e

if [ `uname -m` = "armv6l" ]
then
	echo "-extldflags '-Wl,--long-plt'"
fi
exit 0
