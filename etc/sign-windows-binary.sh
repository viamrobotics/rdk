#!/bin/bash
# Authenticode-signs a Windows executable with Viam's EV code signing key in Google Cloud KMS.
#
# Usage: sign-windows-binary.sh <display name> <file>
#
# Requires java on PATH and these environment variables:
#   GCP_ACCESS_TOKEN  access token for the ev-code-signing service account
#   SIGNING_CERT      PEM certificate chain for the signing key
set -euo pipefail

if [ $# -ne 2 ]; then
	echo "usage: $0 <display name> <file>" >&2
	exit 1
fi
: "${GCP_ACCESS_TOKEN:?must be set}"
: "${SIGNING_CERT:?must be set}"

JSIGN_VERSION=7.1
WORK_DIR=/tmp/jsign
JSIGN_JAR="$WORK_DIR/jsign-$JSIGN_VERSION.jar"
CERT_FILE="$WORK_DIR/cert.pem"

mkdir -p "$WORK_DIR"
if [ ! -f "$JSIGN_JAR" ]; then
	curl -fsSL -o "$JSIGN_JAR" "https://github.com/ebourg/jsign/releases/download/$JSIGN_VERSION/jsign-$JSIGN_VERSION.jar"
fi
printf '%s\n' "$SIGNING_CERT" > "$CERT_FILE"

# java is a native Windows program, so msys2 and Git Bash paths must be converted before passing them.
winpath() {
	if command -v cygpath > /dev/null; then
		cygpath -m "$1"
	else
		echo "$1"
	fi
}

java -jar "$(winpath "$JSIGN_JAR")" \
	--name "$1" \
	--storetype GOOGLECLOUD \
	--keystore projects/engineering-tools-310515/locations/global/keyRings/release_signing_key \
	--storepass "$GCP_ACCESS_TOKEN" \
	--alias ev-code-signing-key/cryptoKeyVersions/2 \
	--certfile "$(winpath "$CERT_FILE")" \
	--tsaurl http://timestamp.digicert.com \
	--tsmode rfc3161 \
	"$(winpath "$2")"
