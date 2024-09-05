#!/bin/bash

# find the real linker, from env or same defaults as go
REAL_LD="${CC}"
if [[ -z "${REAL_LD}" ]]; then
	REAL_LD="$(which gcc)"
fi
if [[ -z "${REAL_LD}" ]]; then
	REAL_LD="$(which clang)"
fi


ARGS=("$@")

# add for 32-bit arm builds
if [[ `uname -m` =~ armv[6,7]l ]]; then
	ARGS+=("-Wl,--long-plt")
fi

# exec early if we're not actually filtering anything
if [[ -z "${VIAM_STATIC_BUILD}" ]]; then
	exec "$REAL_LD" "${ARGS[@]}"
fi

if [[ `uname` != "Linux" ]]; then
	echo "Static building is currently only supported under Linux"
	exit 1
fi

# list of linker arguments to ignore
STRIPPED_ARGS="-g -O2"

# list of arguments to prefix with -Bstatic
STATIC_ARGS="-lx264 -lnlopt -ltensorflowlite_c -lpigpio -lstdc++"

# add explicit static standard library flags
FILTERED=("-static-libgcc" "-static-libstdc++")
# Loop through the arguments and filter
for ARG in "${ARGS[@]}"; do
	if [[ "${STRIPPED_ARGS[@]}" =~ "${ARG}" ]]; then
		# don't forward the arg
		:
	elif [[ "${STATIC_ARGS[@]}" =~ "${ARG}" ]]; then
		# wrap the arg as a static one
		FILTERED+=("-Wl,-Bstatic" "${ARG}" "-Wl,-Bdynamic")
	else
		# pass through with no filtering
		FILTERED+=("${ARG}")
	fi
done

# add libstdc++ statically (and last)
FILTERED+=("-Wl,-Bstatic" "-lstdc++" "-Wl,-Bdynamic")

# call the real linker with the filtered arguments
exec "$REAL_LD" "${FILTERED[@]}"
