#!/bin/bash

# Min and Max versions for each package
BINARIES=( \
	"go 1.16 1.17.99" \
	"npm 6.14.14 7.99" \
	"buf 0.40.0 0.42.1" \
	"protoc 3.15.6 3.17.3" \
	"protoc-gen-go 1.26.0 1.26.0" \
	"protoc-gen-go-grpc 1.1.0 1.1.0" \
	"protoc-gen-grpc-gateway dev dev" \
	"protoc-gen-doc 1.3.2 1.3.2"\
	#"grpcurl dev dev" \
)

# To be checked via pkg-config --modversion (not always the package version)
LIBRARIES=( \
	"vpx 1.7.0 1.10.0" \
	"x264 0.155.2917 0.161.3049" \
	"nlopt 2.7.0 2.7.0" \
)


# This one has to be checked via npm
PROTOC_GEN_TS_VERSIONS=("0.14.0" "0.15.0")

# This binary doesn't have a version option, so we check the hash
# Version 1.2.1, Linux and Darwin binaries
PROTOC_GEN_GRPC_WEB_VERSION="1.2.1"
PROTOC_GEN_GRPC_WEB_HASHS=(
	'6ce1625db7902d38d38d83690ec578c182e9cf2abaeb58d3fba1dae0c299c597'\
	'81bb5d4d3ae0340568fd0739402c052f32476dd520b44355e5032b556a3bc0da'\
	)

vercomp () {
    if [[ $1 == $2 ]]
    then
        return 0
    fi
    local IFS=.
    local i ver1=($1) ver2=($2)
    # fill empty fields in ver1 with zeros
    for ((i=${#ver1[@]}; i<${#ver2[@]}; i++))
    do
        ver1[i]=0
    done
    for ((i=0; i<${#ver1[@]}; i++))
    do
        if [[ -z ${ver2[i]} ]]
        then
            # fill empty fields in ver2 with zeros
            ver2[i]=0
        fi
        if ((10#${ver1[i]} > 10#${ver2[i]}))
        then
            return 1
        fi
        if ((10#${ver1[i]} < 10#${ver2[i]}))
        then
            return 2
        fi
    done
    return 0
}


verminmax() {

	if [[ "$2" =~ "dev" || "$3" =~ "dev" ]] && [[ $1 =~ "dev" ]]
	then
		return 0
	fi

	vercomp $1 $2
	if [ $? == 2 ]
	then
		return 1
	fi

	vercomp $1 $3
	if [ $? == 1 ]
	then
		return 1
	fi

	return 0
}

ALL_GOOD=1

if ! which pkg-config > /dev/null || (! which shasum > /dev/null && ! which sha256sum )
then
	echo "Missing utilities! Please install pkg-config and openssl."
	exit 1
fi

echo "Checking program versions..."

for BINARYSTRING in "${BINARIES[@]}"
do
	BINARY=($BINARYSTRING)
	if which "${BINARY[0]}" > /dev/null
	then
		if [[ ${BINARY[0]} == "go" ]]
		then
			VERSION=`${BINARY[0]} version 2>&1 | grep -Eo '[0-9]*\.[0-9\.]*'`
		else
			VERSION=`${BINARY[0]} --version 2>&1 | grep -Eo '[0-9]*\.[0-9\.]*'`
		fi

		if [[ ${BINARY[1]} == "dev" || ${BINARY[2]} == "dev" ]] 
		then
			VERSION="dev"
		fi
	else
		echo "${BINARY[0]} not installed. Please run setup.sh or install version ${BINARY[1]}"
		ALL_GOOD=0
	fi

	if verminmax $VERSION ${BINARY[1]} ${BINARY[2]}
	then
		echo "OK: ${BINARY[0]} $VERSION"
	else
		echo "Fail: ${BINARY[0]} version ($VERSION) is outside expected range: ${BINARY[1]} - ${BINARY[2]}"
		ALL_GOOD=0
	fi
done


for LIBRARYSTRING in "${LIBRARIES[@]}"
do
	LIBRARY=($LIBRARYSTRING)
	if pkg-config ${LIBRARY[0]} > /dev/null
	then
		VERSION=`pkg-config --modversion ${LIBRARY[0]} 2>&1 | grep -Eo '[0-9]*\.[0-9\.]*'`
	else
		echo "${LIBRARY[0]} library not installed. Please run setup.sh or install version ${LIBRARY[1]}"
		ALL_GOOD=0
	fi

	if verminmax $VERSION ${LIBRARY[1]} ${LIBRARY[2]}
	then
		echo "OK: ${LIBRARY[0]} library $VERSION"
	else
		echo "Fail: ${LIBRARY[0]} library version ($VERSION) is outside expected range: ${LIBRARY[1]} - ${LIBRARY[2]}"
		ALL_GOOD=0
	fi
done



# protoc-gen-grpc-web doesn't contain it's own version number, so we check known hashes instead.
if which protoc-gen-grpc-web > /dev/null
then
	HASH=`openssl sha256 \`which protoc-gen-grpc-web\` | awk '{print $2}'`
else
	echo "protoc-gen-grpc-web not installed. Please run setup.sh or install version $PROTOC_GEN_GRPC_WEB_VERSION"
	ALL_GOOD=0
fi

if [[ " ${PROTOC_GEN_GRPC_WEB_HASHS[@]} " =~ " ${HASH} " ]]
then
	echo "OK: protoc-gen-grpc-web $PROTOC_GEN_GRPC_WEB_VERSION"
else
	echo "Fail: protoc-gen-grpc-web version is unknown. Please install $PROTOC_GEN_GRPC_WEB_VERSION"
	ALL_GOOD=0
fi



# protoc-gen-ts has to be checked via npm
if which protoc-gen-ts > /dev/null
then
	PROTOC_GEN_TS_VERSION=`npm -g list ts-protoc-gen | grep -Eo '[0-9]*\.[0-9\.]*'`
else
	echo "protoc-gen-ts not installed. Please run setup.sh or install version ${PROTOC_GEN_TS_VERSION[0]}"
	ALL_GOOD=0
fi

if verminmax $PROTOC_GEN_TS_VERSION ${PROTOC_GEN_TS_VERSION[0]} ${PROTOC_GEN_TS_VERSION[0]}
then
	echo "OK: protoc-gen-ts $PROTOC_GEN_TS_VERSION"
else
	echo "Fail: protoc-gen-ts version ($PROTOC_GEN_TS_VERSION) is outside expected range: ${PROTOC_GEN_TS_VERSION[0]} - ${PROTOC_GEN_TS_VERSION[1]}"
	ALL_GOOD=0
fi


if [[ ALL_GOOD -eq 1 ]]
then
	exit 0
else
	exit 1
fi
