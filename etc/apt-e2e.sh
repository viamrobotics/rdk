#!/bin/bash
# Installs viam-cli from the public Viam apt repo, exactly as the docs describe.
# Run in a clean Debian/Ubuntu container as root. Set EXPECTED_VERSION (no
# leading v) to assert the installed version, retrying while the repo index
# catches up after an upload.
set -euo pipefail

apt-get update -qq
apt-get install -yqq curl gpg ca-certificates >/dev/null

curl -fsSL https://us-apt.pkg.dev/doc/repo-signing-key.gpg | gpg --dearmor -o /usr/share/keyrings/viam.gpg
echo "deb [signed-by=/usr/share/keyrings/viam.gpg] https://us-apt.pkg.dev/projects/static-file-server-310021 viam main" \
	> /etc/apt/sources.list.d/viam.list

for attempt in $(seq 1 10); do
	apt-get update -qq
	if [ -z "${EXPECTED_VERSION:-}" ] || apt-cache policy viam-cli | grep -qF "$EXPECTED_VERSION"; then
		break
	fi
	if [ "$attempt" = 10 ]; then
		echo "version $EXPECTED_VERSION never appeared in the repo" >&2
		exit 1
	fi
	sleep 30
done

apt-get install -yqq viam-cli >/dev/null
viam version
if [ -n "${EXPECTED_VERSION:-}" ]; then
	viam version | grep -qF "$EXPECTED_VERSION"
fi

# `viam update` must go through apt and leave dpkg-owned files untouched
# (dpkg -V exits 0 even when it reports differences, so assert on its output)
viam update
if [ -n "$(dpkg -V viam-cli)" ]; then
	echo "viam update modified dpkg-owned files:" >&2
	dpkg -V viam-cli >&2
	exit 1
fi

echo "apt e2e OK: $(viam version)"
