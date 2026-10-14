package cli

import (
	"path/filepath"

	"github.com/pkg/errors"
)

const (
	visualizerAssetPrefix = "viam-visualizer"
	// visualizerBundleName is the macOS app bundle inside the .app.tar.gz asset.
	visualizerBundleName = "Viam Visualizer.app"
	osDarwin             = "darwin"
	archAmd64            = "amd64"
	archArm64            = "arm64"
)

// visualizerArch maps GOARCH to the arch segment the release workflow uses in asset names.
var visualizerArch = map[string]string{
	archAmd64: "x86_64",
	archArm64: "aarch64",
}

// releaseAssetName derives the asset the CLI installs for a platform:
// darwin -> .app.tar.gz, linux -> .AppImage, windows -> the portable .exe. Linux arm64 and
// Windows arm64 are not published, so they return an error naming the combination.
func releaseAssetName(version, goos, goarch string) (string, error) {
	arch, ok := visualizerArch[goarch]
	if !ok {
		return "", errors.Errorf("no Viam Visualizer build for %s/%s", goos, goarch)
	}
	var suffix string
	switch goos {
	case osDarwin:
		suffix = ".app.tar.gz"
	case osLinux:
		if goarch != archAmd64 {
			return "", errors.Errorf("no Viam Visualizer build for %s/%s", goos, goarch)
		}
		suffix = ".AppImage"
	case osWindows:
		if goarch != archAmd64 {
			return "", errors.Errorf("no Viam Visualizer build for %s/%s", goos, goarch)
		}
		suffix = ".exe"
	default:
		return "", errors.Errorf("no Viam Visualizer build for %s/%s", goos, goarch)
	}
	return visualizerAssetPrefix + "-" + version + "-" + goos + "-" + arch + suffix, nil
}

// installedVisualizerBinary is the executable path inside a version's cache directory.
func installedVisualizerBinary(root, version, goos string) string {
	dir := filepath.Join(root, version)
	switch goos {
	case osDarwin:
		return filepath.Join(dir, visualizerBundleName, "Contents", "MacOS", visualizerBundleBinary)
	case osWindows:
		return filepath.Join(dir, visualizerBundleBinary+".exe")
	default:
		return filepath.Join(dir, visualizerBundleBinary+".AppImage")
	}
}
