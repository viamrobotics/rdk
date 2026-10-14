package cli

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"

	rutils "go.viam.com/rdk/utils"
)

const (
	// visualizerReleaseAPI lists releases of viamrobotics/visualization; "/latest" gives the newest.
	visualizerReleaseAPI = "https://api.github.com/repos/viamrobotics/visualization/releases"
	// visualizerDownloadBase is where release assets live: <base>/v<version>/<asset>.
	visualizerDownloadBase  = "https://github.com/viamrobotics/visualization/releases/download"
	visualizerChecksumsFile = "checksums.txt"
	// visualizerUpdateCheckInterval throttles the "newer version available" hint.
	visualizerUpdateCheckInterval = time.Hour
)

// visualizerInstallOptions carries everything ensureVisualizer needs, so tests can point it at a
// temp root and an httptest server.
type visualizerInstallOptions struct {
	// root is the cache directory, ~/.viam/visualizer in production.
	root string
	// version pins a release ("1.2.3", no leading v). Empty means the latest installed, else the
	// latest release.
	version string
	// update forces a lookup of the latest release and installs it if newer than the cache.
	update bool
	goos   string
	goarch string
	// apiURL and downloadURL default to visualizerReleaseAPI and visualizerDownloadBase.
	apiURL      string
	downloadURL string
	client      *http.Client
	// out receives progress lines and the update hint. Never the token.
	out io.Writer
}

// defaultVisualizerInstallOptions is the production configuration.
func defaultVisualizerInstallOptions(out io.Writer, version string, update bool) visualizerInstallOptions {
	return visualizerInstallOptions{
		root:        visualizerCacheRoot(),
		version:     version,
		update:      update,
		goos:        runtime.GOOS,
		goarch:      runtime.GOARCH,
		apiURL:      visualizerReleaseAPI,
		downloadURL: visualizerDownloadBase,
		client:      http.DefaultClient,
		out:         out,
	}
}

// visualizerCacheRoot is ~/.viam/visualizer.
func visualizerCacheRoot() string {
	return filepath.Join(rutils.ViamDotDir, "visualizer")
}

const (
	visualizerLastCheckFile = "last-update-check"
	visualizerTmpSuffix     = ".tmp"
)

func (opts *visualizerInstallOptions) applyDefaults() {
	if opts.apiURL == "" {
		opts.apiURL = visualizerReleaseAPI
	}
	if opts.downloadURL == "" {
		opts.downloadURL = visualizerDownloadBase
	}
	if opts.client == nil {
		opts.client = http.DefaultClient
	}
	if opts.goos == "" {
		opts.goos = runtime.GOOS
	}
	if opts.goarch == "" {
		opts.goarch = runtime.GOARCH
	}
	if opts.out == nil {
		opts.out = io.Discard
	}
}

// ensureVisualizer returns the path of an installed visualizer binary, downloading and verifying a
// release into opts.root when the cache has none that satisfies opts.version and opts.update.
func ensureVisualizer(ctx context.Context, opts visualizerInstallOptions) (string, error) {
	opts.applyDefaults()
	if opts.version != "" {
		bin := installedVisualizerBinary(opts.root, opts.version, opts.goos)
		if _, err := os.Stat(bin); err == nil {
			return bin, nil
		}
		return installVisualizerRelease(ctx, opts, opts.version)
	}
	installed, err := latestInstalledVersion(opts.root)
	if err != nil {
		return "", err
	}
	if opts.update || installed == "" {
		latest, err := latestReleaseVersion(ctx, opts.client, opts.apiURL)
		if err != nil {
			return "", err
		}
		if installed == "" || versionNewer(latest, installed) {
			return installVisualizerRelease(ctx, opts, latest)
		}
		return installedVisualizerBinary(opts.root, installed, opts.goos), nil
	}
	opts.maybeHintUpdate(ctx, installed)
	return installedVisualizerBinary(opts.root, installed, opts.goos), nil
}

// versionNewer reports whether candidate is a higher semver than current.
func versionNewer(candidate, current string) bool {
	c, err := semver.NewVersion(candidate)
	if err != nil {
		return false
	}
	cur, err := semver.NewVersion(current)
	if err != nil {
		return true
	}
	return c.GreaterThan(cur)
}

// maybeHintUpdate prints a throttled "newer version" hint. It never fails or blocks launch on error.
func (opts visualizerInstallOptions) maybeHintUpdate(ctx context.Context, installed string) {
	stamp := filepath.Join(opts.root, visualizerLastCheckFile)
	if data, err := os.ReadFile(stamp); err == nil { //nolint:gosec
		if last, perr := time.Parse(time.RFC3339, strings.TrimSpace(string(data))); perr == nil &&
			time.Since(last) < visualizerUpdateCheckInterval {
			return
		}
	}
	latest, err := latestReleaseVersion(ctx, opts.client, opts.apiURL)
	if err != nil {
		return
	}
	os.WriteFile(stamp, []byte(time.Now().UTC().Format(time.RFC3339)), 0o600) //nolint:errcheck
	if versionNewer(latest, installed) {
		fmt.Fprintf(opts.out, "A newer Viam Visualizer (%s) is available. Run with --update to install it.\n", latest) //nolint:errcheck
	}
}

// latestReleaseVersion returns the newest release version (no leading v) from the GitHub API.
func latestReleaseVersion(ctx context.Context, client *http.Client, apiURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL+"/latest", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "looking up latest Viam Visualizer release")
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode != http.StatusOK {
		return "", errors.Errorf("looking up latest Viam Visualizer release: unexpected status %s", resp.Status)
	}
	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", errors.Wrap(err, "decoding latest Viam Visualizer release")
	}
	version := strings.TrimPrefix(release.TagName, "v")
	if version == "" {
		return "", errors.New("latest Viam Visualizer release has no tag_name")
	}
	return version, nil
}

// latestInstalledVersion returns the highest semver directory under root, or "" when none.
func latestInstalledVersion(root string) (string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	var best *semver.Version
	bestName := ""
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasSuffix(entry.Name(), visualizerTmpSuffix) {
			continue
		}
		v, err := semver.NewVersion(entry.Name())
		if err != nil {
			continue
		}
		if best == nil || v.GreaterThan(best) {
			best, bestName = v, entry.Name()
		}
	}
	return bestName, nil
}

// installVisualizerRelease downloads the asset for version into opts.root/<version>, after
// verifying its sha256 against checksums.txt, and returns the binary path. Nothing is left behind
// on failure.
func installVisualizerRelease(ctx context.Context, opts visualizerInstallOptions, version string) (_ string, err error) {
	asset, err := releaseAssetName(version, opts.goos, opts.goarch)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(opts.out, "Downloading Viam Visualizer %s (%s)...\n", version, asset) //nolint:errcheck
	if err := os.MkdirAll(opts.root, 0o750); err != nil {
		return "", err
	}
	base := opts.downloadURL + "/v" + version + "/"
	checksums, err := fetchVisualizerBytes(ctx, opts.client, base+visualizerChecksumsFile)
	if err != nil {
		return "", err
	}
	want, err := expectedChecksum(checksums, asset)
	if err != nil {
		return "", err
	}
	download, err := downloadVisualizerAsset(ctx, opts, base+asset, asset, want)
	if err != nil {
		return "", err
	}
	defer os.Remove(download) //nolint:errcheck

	tmpDir := filepath.Join(opts.root, version+visualizerTmpSuffix)
	defer func() {
		if err != nil {
			os.RemoveAll(tmpDir) //nolint:errcheck
		}
	}()
	if err := os.RemoveAll(tmpDir); err != nil {
		return "", err
	}
	if err := os.MkdirAll(tmpDir, 0o750); err != nil {
		return "", err
	}
	if err := unpackVisualizer(download, tmpDir, opts.root, version, opts.goos); err != nil {
		return "", errors.Wrapf(err, "unpacking %s", asset)
	}
	finalDir := filepath.Join(opts.root, version)
	if err := os.RemoveAll(finalDir); err != nil {
		return "", err
	}
	if err := os.Rename(tmpDir, finalDir); err != nil {
		return "", err
	}
	fmt.Fprintf(opts.out, "Installed Viam Visualizer %s\n", version) //nolint:errcheck
	return installedVisualizerBinary(opts.root, version, opts.goos), nil
}

func fetchVisualizerBytes(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	resp, err := getVisualizerURL(ctx, client, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck
	return io.ReadAll(resp.Body)
}

func getVisualizerURL(ctx context.Context, client *http.Client, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "downloading %s", filepath.Base(url))
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close() //nolint:errcheck
		return nil, errors.Errorf("downloading %s: unexpected status %s", filepath.Base(url), resp.Status)
	}
	return resp, nil
}

// downloadVisualizerAsset streams url to a temp file under opts.root while hashing it, and returns
// the file path once the sha256 matches want. The file is removed on any failure.
func downloadVisualizerAsset(ctx context.Context, opts visualizerInstallOptions, url, asset, want string) (_ string, err error) {
	resp, err := getVisualizerURL(ctx, opts.client, url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close() //nolint:errcheck
	f, err := os.CreateTemp(opts.root, "download-*")
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			os.Remove(f.Name()) //nolint:errcheck
		}
	}()
	hash := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, hash), resp.Body); err != nil {
		f.Close() //nolint:errcheck
		return "", errors.Wrapf(err, "downloading %s", asset)
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	if !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), want) {
		return "", errors.Errorf("checksum mismatch for %s", asset)
	}
	return f.Name(), nil
}

// unpackVisualizer places the downloaded asset into tmpDir so that, after tmpDir is renamed to
// <root>/<version>, installedVisualizerBinary points at the executable.
func unpackVisualizer(download, tmpDir, root, version, goos string) error {
	if goos == osDarwin {
		return untarVisualizer(download, tmpDir)
	}
	dest := installedVisualizerBinary(root, version+visualizerTmpSuffix, goos)
	if err := copyVisualizerFile(download, dest, 0o755); err != nil {
		return err
	}
	if goos == osLinux {
		return os.Chmod(dest, 0o755) //nolint:gosec
	}
	return nil
}

func copyVisualizerFile(src, dest string, mode os.FileMode) error {
	in, err := os.Open(src) //nolint:gosec
	if err != nil {
		return err
	}
	defer in.Close() //nolint:errcheck
	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		return err
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode) //nolint:gosec
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close() //nolint:errcheck
		return err
	}
	return out.Close()
}

// untarVisualizer extracts a gzip tarball into dir, rejecting entries that escape it.
func untarVisualizer(archive, dir string) error {
	f, err := os.Open(archive) //nolint:gosec
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close() //nolint:errcheck
	tr := tar.NewReader(gz)
	cleanDir := filepath.Clean(dir)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := extractVisualizerEntry(tr, hdr, cleanDir); err != nil {
			return err
		}
	}
}

func extractVisualizerEntry(tr *tar.Reader, hdr *tar.Header, dir string) error {
	target := filepath.Join(dir, hdr.Name) //nolint:gosec
	if target != dir && !strings.HasPrefix(target, dir+string(filepath.Separator)) {
		return errors.Errorf("archive entry %q escapes the install directory", hdr.Name)
	}
	mode := hdr.FileInfo().Mode().Perm()
	switch hdr.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(target, mode|0o700)
	case tar.TypeReg:
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode) //nolint:gosec
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close() //nolint:errcheck
			return err
		}
		return out.Close()
	case tar.TypeSymlink:
		linkTarget := hdr.Linkname
		if !filepath.IsAbs(linkTarget) {
			linkTarget = filepath.Join(filepath.Dir(target), linkTarget) //nolint:gosec
		}
		if !strings.HasPrefix(linkTarget, dir+string(filepath.Separator)) {
			return errors.Errorf("archive symlink %q escapes the install directory", hdr.Name)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return err
		}
		return os.Symlink(hdr.Linkname, target)
	default:
		return nil
	}
}

// expectedChecksum finds the sha256 for assetName in checksums.txt content (sha256sum format).
func expectedChecksum(checksums []byte, assetName string) (string, error) {
	for _, line := range strings.Split(string(checksums), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		if strings.TrimPrefix(fields[1], "*") == assetName {
			return fields[0], nil
		}
	}
	return "", errors.Errorf("no checksum listed for %s", assetName)
}
