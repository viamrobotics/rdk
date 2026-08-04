#!/bin/bash
# Installs viam-cli from the public Viam apt repo, exactly as the docs describe.
# Run in a clean Debian/Ubuntu container as root. Set EXPECTED_VERSION (no
# leading v) to assert the installed version, retrying while the repo index
# catches up after an upload.
set -euo pipefail

# Artifact Registry republishes the signed index (InRelease/Packages)
# asynchronously after an upload, so a freshly released version can be briefly
# absent from the repo even though the upload itself succeeded. Wait it out.
ATTEMPTS=20
SLEEP_SECONDS=30

apt-get update -qq
apt-get install -yqq curl gpg ca-certificates >/dev/null

curl -fsSL https://us-apt.pkg.dev/doc/repo-signing-key.gpg | gpg --dearmor -o /usr/share/keyrings/viam.gpg
echo "deb [signed-by=/usr/share/keyrings/viam.gpg] https://us-apt.pkg.dev/projects/static-file-server-310021 viam main" \
	> /etc/apt/sources.list.d/viam.list

apt-get update -qq

if [ -n "${EXPECTED_VERSION:-}" ]; then
	for attempt in $(seq 1 "$ATTEMPTS"); do
		# Capture before matching. Piping into `grep -q` makes grep exit on the
		# first match, killing apt-cache with SIGPIPE (141); `set -o pipefail`
		# then reports the pipeline as failed even though the match succeeded.
		policy=$(apt-cache policy viam-cli)

		# Match on Candidate rather than mere presence in the version table:
		# Candidate is the version `apt-get install viam-cli` will actually pick,
		# and it keeps 1.2.0 from matching an unrelated 21.2.01.
		case $policy in
		*"Candidate: $EXPECTED_VERSION"*) break ;;
		esac

		if [ "$attempt" = "$ATTEMPTS" ]; then
			echo "version $EXPECTED_VERSION never appeared in the repo; apt-cache policy reported:" >&2
			echo "$policy" >&2
			exit 1
		fi

		sleep "$SLEEP_SECONDS"
		apt-get update -qq
	done
fi

apt-get install -yqq viam-cli >/dev/null
viam version
if [ -n "${EXPECTED_VERSION:-}" ]; then
	installed=$(viam version)
	case $installed in
	*"$EXPECTED_VERSION"*) ;;
	*)
		echo "installed viam reports '$installed', expected $EXPECTED_VERSION" >&2
		exit 1
		;;
	esac
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
