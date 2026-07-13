#!/bin/bash

# Use the system selected C++ compiler as the linker. We want to pick up
# a dynamic dependency on the system C++ runtime, just like the system C runtime.
# If, for some reason, it isn't set, do our best with `c++` which is probably fine.
REAL_LD="${CXX}"
if [[ -z "${REAL_LD}" ]]; then
	REAL_LD="$(which c++)"
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
STATIC_ARGS="-lx264 -lnlopt"

FILTERED=()

# Loop through the arguments and filter
for ARG in "${ARGS[@]}"; do
	if [[ "${STRIPPED_ARGS[@]}" =~ "${ARG}" ]]; then
		# don't forward the arg
		:
	elif [[ "${STATIC_ARGS[@]}" =~ "${ARG}" ]]; then
		# wrap the arg as a static one
		FILTERED+=("-Wl,--push-state,-Bstatic" "${ARG}" "-Wl,--pop-state")
	else
		# pass through with no filtering
		FILTERED+=("${ARG}")
	fi
done

# call the real linker with the filtered arguments
exec "$REAL_LD" "${FILTERED[@]}"
