#!/bin/bash

ARGS=("$@")

# add for 32-bit arm builds
if [[ `uname -m` =~ armv[6,7]l ]]; then
	ARGS+=("-Wl,--long-plt")
fi

# exec early if we're not actually filtering anything
if [[ -z "${VIAM_STATIC_BUILD}" ]]; then
	exec gcc ${ARGS[@]}
fi

# List of linker arguments to ignore
STRIPPED_ARGS="-g -O2"

# List of arguments to prefix with -Bstatic
STATIC_ARGS="-lx264 -lnlopt -ljpeg -ltensorflowlite_c"

# add explicit static standard library flags
FILTERED="-static-libgcc -static-libstdc++"
# Loop through the arguments and filter
for ARG in "${ARGS[@]}"; do
  if [[ "${STRIPPED_ARGS[@]}" =~ "${ARG}" ]]; then
    # don't forward the arg
  	:
  elif [[ "${STATIC_ARGS[@]}" =~ "${ARG}" ]]; then
    # wrap the arg as a static one
    FILTERED+=("-Wl,-Bstatic ${ARG} -Wl,-Bdynamic")
  else
  	# pass through with no filtering
    FILTERED+=("${ARG}")
  fi
done

# add libstdc++ statically (and last)
FILTERED+=("-Wl,-Bstatic -lstdc++ -Wl,-Bdynamic")

if [[ -z "${LONG_PLT}" ]]; then
	FILTERED+=("${LONG_PLT}")
fi

# Call GCC with the filtered arguments
exec gcc ${FILTERED[@]}
