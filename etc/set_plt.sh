#!/bin/bash

set -e

if [[ `uname -m` =~ armv[6,7]l ]]
then
	echo "-extldflags '-Wl,--long-plt'"
fi
exit 0
