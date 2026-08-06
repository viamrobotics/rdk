#!/bin/bash

# Verify that published viam-server manifests advertise the sha256 of the object
# their upload-path actually points at.
#
# A manifest and the binary it describes are published by two separate gsutil mv
# calls (staticbuild-deploy.yml), and whether a build is a prerelease is decided
# twice over: packaging.make keys the manifest's upload-path on "-dev" appearing
# in BUILD_CHANNEL, while the deploy workflow keys the binary's destination on
# release_type == 'latest'. Nothing enforces that those two agree, and nothing
# re-reads the result. When they disagree -- or when concurrent runs interleave
# the two moves -- the bucket is left with a manifest describing a binary nobody
# can download, and every agent targeting that version rejects its download and
# retries forever (see viam-agent v1.0.1 windows, viamrobotics/agent#280).
#
# This never consults a local build. It answers "is the bucket self-consistent?"
# rather than "does the bucket match me?", which is what makes it meaningful
# regardless of which of two racing runs published last.
#
# Reads are anonymous; no credentials needed.

set -euo pipefail

BUCKET="packages.viam.com"
MANIFEST_DIR="apps/viam-subsystems"
PREFIX="$MANIFEST_DIR/viam-server-"
BASE_URL="https://storage.googleapis.com"
PLATFORMS="x86_64|aarch64|windows-x86_64|darwin-aarch64"

usage() {
	cat <<'EOF'
Usage:
  verify-manifests.sh NAME.json [NAME.json ...]  check exactly these manifests
  verify-manifests.sh --version v1.2.3           every platform for one version
  verify-manifests.sh --count 20                 the 20 most recently written
  verify-manifests.sh --count 0                  every release manifest
  verify-manifests.sh --count 0 --include-prerelease   ... plus -dev./-rc

Names are basenames within gs://packages.viam.com/apps/viam-subsystems/. The
publishing workflow passes the manifests it just moved, so every run that
publishes re-reads what it published.

On a mismatch this prints the corrected manifest and the gsutil command that
would apply it, but never writes. Rewriting the sha256 is only the right repair
when the published binary is the artifact you meant to ship; if the binary is
what is wrong, rebuild instead. See the summary this prints on failure.
EOF
}

count=""
include_prerelease=false
version_filter=""
explicit_names=()

while [ $# -gt 0 ]; do
	case "$1" in
	--count)
		count="${2:?--count needs a value}"
		shift
		;;
	--include-prerelease)
		include_prerelease=true
		;;
	--version)
		version_filter="${2:?--version needs a value}"
		shift
		;;
	-h | --help)
		usage
		exit 0
		;;
	-*)
		echo "unknown argument: $1" >&2
		usage >&2
		exit 2
		;;
	*)
		explicit_names+=("$1")
		;;
	esac
	shift
done

selectors=0
[ -n "$version_filter" ] && selectors=$((selectors + 1))
[ -n "$count" ] && selectors=$((selectors + 1))
[ ${#explicit_names[@]} -gt 0 ] && selectors=$((selectors + 1))
if [ "$selectors" -gt 1 ]; then
	echo "explicit names, --version and --count select different things; pass only one" >&2
	exit 2
fi

sha256() {
	if command -v sha256sum >/dev/null; then
		sha256sum
	else
		shasum -a 256
	fi
}

# Emit "<updated>\t<object name>" for every object under a prefix. Anonymous
# reads are enough; no gcloud auth required.
list_objects() {
	local prefix="$1" token="" url page
	while :; do
		url="$BASE_URL/storage/v1/b/$BUCKET/o?prefix=$prefix&fields=items(name,updated),nextPageToken&maxResults=1000"
		if [ -n "$token" ]; then
			url="$url&pageToken=$token"
		fi
		page=$(curl -fsS --retry 3 -H "Cache-Control: no-cache" "$url")
		jq -r '.items[]? | "\(.updated)\t\(.name)"' <<<"$page"
		token=$(jq -r '.nextPageToken // empty' <<<"$page")
		if [ -z "$token" ]; then
			break
		fi
	done
}

names=()
if [ ${#explicit_names[@]} -gt 0 ]; then
	for n in "${explicit_names[@]}"; do
		names+=("${n##*/}")
	done
else
	if [ -n "$version_filter" ]; then
		# Listed under the version's own prefix, so this is one small API call
		# rather than a full bucket listing. The platform-suffix anchor still
		# matters: the prefix v1.2.3- also matches v1.2.3-dev.N and v1.2.3-rcN,
		# which are different sets of artifacts.
		escaped=$(printf '%s' "$version_filter" | sed 's/\./\\./g')
		candidates=$(list_objects "$PREFIX$version_filter-" | cut -f2 |
			grep -E "/viam-server-${escaped}-($PLATFORMS)\.json$" | sort || true)
	else
		# Newest first by mtime rather than by version: a clobbered manifest gets
		# a fresh mtime, so the recently-written window is where a race shows up.
		# (Name order would not do: the bucket still holds pre-semver
		# viam-server-<timestamp>-* manifests that sort ahead of every v*.)
		candidates=$(list_objects "$PREFIX" | sort -r | cut -f2)
		if [ "$include_prerelease" != true ]; then
			candidates=$(printf '%s\n' "$candidates" | grep -v -e '-dev\.' -e '-rc' || true)
		fi
		if [ "${count:-20}" -gt 0 ]; then
			candidates=$(printf '%s\n' "$candidates" | sed -n "1,${count:-20}p")
		fi
	fi

	if [ -z "$candidates" ]; then
		echo "no manifests matched gs://$BUCKET/$PREFIX${version_filter}*" >&2
		exit 1
	fi

	while IFS= read -r object; do
		names+=("${object##*/}")
	done <<<"$candidates"
fi

echo "Checking ${#names[@]} manifest(s) in gs://$BUCKET/$MANIFEST_DIR/"
echo

sha=""
reason_lines=()

# Compare one manifest against the object its upload-path names. Sets `sha` on
# success, `reason_lines` on failure.
check_manifest() {
	local name="$1" manifest_url manifest want upload_path binary_url got fix_json
	sha=""
	reason_lines=()

	manifest_url="$BASE_URL/$BUCKET/$MANIFEST_DIR/$name"
	if ! manifest=$(curl -fsS --retry 3 -H "Cache-Control: no-cache" "$manifest_url"); then
		reason_lines=("manifest not readable at $manifest_url")
		return 1
	fi

	want=$(jq -r '.sha256 // empty' <<<"$manifest")
	upload_path=$(jq -r '."upload-path" // empty' <<<"$manifest")
	if [ -z "$want" ] || [ -z "$upload_path" ]; then
		reason_lines=("manifest is missing sha256 or upload-path")
		return 1
	fi

	binary_url="$BASE_URL/$upload_path"
	if ! got=$(curl -fsSL --retry 3 -H "Cache-Control: no-cache" "$binary_url" | sha256 | cut -d' ' -f1); then
		reason_lines=("cannot download upload-path $binary_url")
		return 1
	fi

	if [ "$want" != "$got" ]; then
		reason_lines=("manifest sha256 $want" "binary   sha256 $got  ($binary_url)")
		# Printed only. A mismatch looks the same whether the manifest's sha is
		# wrong or the wrong binary landed at upload-path, and this script cannot
		# tell which; applying it in the second case blesses the wrong artifact
		# for every agent on that version. An operator decides.
		fix_json=$(jq --tab --arg s "$got" '.sha256 = $s' <<<"$manifest")
		reason_lines+=("" "if the object above is the artifact you meant to publish," "correct the manifest with:" "")
		while IFS= read -r line; do
			reason_lines+=("  $line")
		done <<<"$fix_json"
		reason_lines+=("" "  gsutil -h \"Cache-Control:no-cache\" cp $name gs://$BUCKET/$MANIFEST_DIR/" "")
		reason_lines+=("otherwise the binary itself is wrong; see the summary below.")
		return 1
	fi

	sha="$want"
	return 0
}

failures=0
for name in "${names[@]}"; do
	if check_manifest "$name"; then
		echo "ok    $name  $sha"
		continue
	fi

	# A publish writes the binary before the manifest, so a version being
	# published right now looks like a mismatch until its manifest lands.
	# Re-check once before failing; a real mismatch persists.
	sleep 10
	if check_manifest "$name"; then
		echo "ok    $name  $sha  (cleared on re-check, publish was in flight)"
		continue
	fi

	echo "FAIL  $name"
	printf '        %s\n' "${reason_lines[@]}"
	failures=$((failures + 1))
done

echo
if [ "$failures" -gt 0 ]; then
	echo "$failures of ${#names[@]} manifest(s) do not match the object they point at."
	echo
	echo "Repair by hand, choosing what matches what the object at upload-path is:"
	echo
	echo "  The published binary is the artifact you meant to ship and only the"
	echo "  sha256 is wrong -- apply the corrected manifest printed above."
	echo "  Binaries are untouched. This is the repair for a concurrent-run"
	echo "  clobber or a prerelease-path disagreement."
	echo
	echo "  The binary itself is wrong -- rebuild the channel and republish"
	echo "  binary and manifest together. Windows binaries are not reproducible"
	echo "  (Authenticode timestamps), so that publishes new bytes rather than"
	echo "  restoring the old ones."
	echo
	echo "Either way, agents cache the bad sha and skip re-reading it for an"
	echo "unchanged version string, so expect a version bump or a cache clear to"
	echo "unstick machines that already pulled it."
	exit 1
fi
echo "All ${#names[@]} manifest(s) match."
