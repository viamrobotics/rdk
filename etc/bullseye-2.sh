#!/usr/bin/env bash

set -euo pipefail

# Raspberry Pi support
[[ "$(uname -m)" == "aarch64" || $(uname -m) == "armv7l" ]] && apt-get install -y libpigpio-dev && echo okay libgipo || echo failed libgpio

# upx
UPX_VERSION=4.0.2
UPX_URL=https://github.com/upx/upx/releases/download/v$UPX_VERSION/upx-$UPX_VERSION-amd64_linux.tar.xz
if [ "$(uname -m)" = "aarch64" ]; then
    UPX_URL=https://github.com/upx/upx/releases/download/v$UPX_VERSION/upx-$UPX_VERSION-arm64_linux.tar.xz
elif [ "$(uname -m)" = "armv7l" ]; then
    UPX_URL=https://github.com/upx/upx/releases/download/v$UPX_VERSION/upx-$UPX_VERSION-arm_linux.tar.xz
fi
echo $UPX_URL
curl -L "\$UPX_URL" | tar -C /usr/local/bin/ --strip-components=1 --wildcards -xJv '*/upx'

# canon
GOBIN=/usr/local/bin go install github.com/viamrobotics/canon@latest

# license_finder
apt-get install -y ruby && gem install license_finder

# This workaround is for https://viam.atlassian.net/browse/RSDK-526, without the application default credential file our tests will
# create goroutines that get leaked and fail. Once https://github.com/googleapis/google-cloud-go/issues/5430 is fixed we can remove this.
check_gcloud_auth(){
	APP_CREDENTIALS_DIR="$HOME/.config/gcloud"
	mkdir -p $APP_CREDENTIALS_DIR
	APP_CREDENTIALS_FILE="$APP_CREDENTIALS_DIR/application_default_credentials.json"	
	if [ ! -f "$APP_CREDENTIALS_FILE" ]; then
		echo "Missing gcloud application default credentials, this can cause goroutines to leak if not configured. Creating with empty config at $APP_CREDENTIALS_FILE"
		echo '{"client_id":"XXXX","client_secret":"XXXX","refresh_token":"XXXX","type":"authorized_user"}' > $APP_CREDENTIALS_FILE
	fi
}

check_gcloud_auth
