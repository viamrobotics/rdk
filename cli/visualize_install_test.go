package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"go.viam.com/test"
)

func TestReleaseAssetName(t *testing.T) {
	for _, tc := range []struct{ goos, goarch, want string }{
		{"darwin", "arm64", "viam-visualizer-1.2.3-darwin-aarch64.app.tar.gz"},
		{"darwin", "amd64", "viam-visualizer-1.2.3-darwin-x86_64.app.tar.gz"},
		{"linux", "amd64", "viam-visualizer-1.2.3-linux-x86_64.AppImage"},
		{"windows", "amd64", "viam-visualizer-1.2.3-windows-x86_64.exe"},
	} {
		got, err := releaseAssetName("1.2.3", tc.goos, tc.goarch)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, got, test.ShouldEqual, tc.want)
	}
	for _, tc := range [][2]string{{"linux", "arm64"}, {"windows", "arm64"}, {"plan9", "amd64"}} {
		_, err := releaseAssetName("1.2.3", tc[0], tc[1])
		test.That(t, err, test.ShouldNotBeNil)
	}
}

func TestInstalledVisualizerBinary(t *testing.T) {
	root := filepath.Join("r", "x")
	test.That(t, installedVisualizerBinary(root, "1.0.0", "darwin"), test.ShouldEqual,
		filepath.Join(root, "1.0.0", "Viam Visualizer.app", "Contents", "MacOS", "viam-visualizer"))
	test.That(t, installedVisualizerBinary(root, "1.0.0", "linux"), test.ShouldEqual,
		filepath.Join(root, "1.0.0", "viam-visualizer.AppImage"))
	test.That(t, installedVisualizerBinary(root, "1.0.0", "windows"), test.ShouldEqual,
		filepath.Join(root, "1.0.0", "viam-visualizer.exe"))
}

func TestExpectedChecksum(t *testing.T) {
	sums := []byte("aaa  one.AppImage\nbbb *two.exe\n")
	got, err := expectedChecksum(sums, "one.AppImage")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got, test.ShouldEqual, "aaa")
	got, err = expectedChecksum(sums, "two.exe")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got, test.ShouldEqual, "bbb")
	_, err = expectedChecksum(sums, "three.exe")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "three.exe")
}

func TestLatestInstalledVersion(t *testing.T) {
	root := t.TempDir()
	got, err := latestInstalledVersion(filepath.Join(root, "missing"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got, test.ShouldEqual, "")
	got, err = latestInstalledVersion(root)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got, test.ShouldEqual, "")

	for _, d := range []string{"1.2.0", "1.10.0", "9.9.9.tmp", "junk"} {
		test.That(t, os.Mkdir(filepath.Join(root, d), 0o750), test.ShouldBeNil)
	}
	test.That(t, os.WriteFile(filepath.Join(root, "99.0.0"), nil, 0o600), test.ShouldBeNil)
	got, err = latestInstalledVersion(root)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got, test.ShouldEqual, "1.10.0")
}

type fakeRelease struct {
	server   *httptest.Server
	requests atomic.Int32
	latest   atomic.Value
}

// newFakeRelease serves /latest plus the checksums and asset for each version in assets. A non-empty
// badSum overrides the published checksum.
func newFakeRelease(t *testing.T, latest, goos, goarch string, assets map[string][]byte, badSum string) *fakeRelease {
	t.Helper()
	f := &fakeRelease{}
	f.latest.Store(latest)
	mux := http.NewServeMux()
	mux.HandleFunc("/latest", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"tag_name":"v%s"}`, f.latest.Load().(string))
	})
	for version, data := range assets {
		name, err := releaseAssetName(version, goos, goarch)
		test.That(t, err, test.ShouldBeNil)
		sum := sha256.Sum256(data)
		hexSum := hex.EncodeToString(sum[:])
		if badSum != "" {
			hexSum = badSum
		}
		mux.HandleFunc("/v"+version+"/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "%s  %s\n", hexSum, name)
		})
		mux.HandleFunc("/v"+version+"/"+name, func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write(data)
		})
	}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.requests.Add(1)
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeRelease) opts(root, goos, goarch string, out *bytes.Buffer) visualizerInstallOptions {
	return visualizerInstallOptions{
		root: root, goos: goos, goarch: goarch,
		apiURL: f.server.URL, downloadURL: f.server.URL, client: f.server.Client(), out: out,
	}
}

func buildAppTarball(t *testing.T, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	test.That(t, tw.WriteHeader(&tar.Header{
		Name: "Viam Visualizer.app/Contents/MacOS/viam-visualizer", Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg,
	}), test.ShouldBeNil)
	_, err := tw.Write([]byte(content))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, tw.Close(), test.ShouldBeNil)
	test.That(t, gz.Close(), test.ShouldBeNil)
	return buf.Bytes()
}

func TestEnsureVisualizer(t *testing.T) {
	ctx := t.Context()

	t.Run("fresh linux install", func(t *testing.T) {
		f := newFakeRelease(t, "1.0.0", "linux", "amd64", map[string][]byte{"1.0.0": []byte("appimage")}, "")
		root := t.TempDir()
		var out bytes.Buffer
		bin, err := ensureVisualizer(ctx, f.opts(root, "linux", "amd64", &out))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, bin, test.ShouldEqual, installedVisualizerBinary(root, "1.0.0", "linux"))
		info, err := os.Stat(bin)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, info.Mode().Perm(), test.ShouldEqual, os.FileMode(0o755))
		test.That(t, out.String(), test.ShouldContainSubstring,
			"Downloading Viam Visualizer 1.0.0 (viam-visualizer-1.0.0-linux-x86_64.AppImage)...")
		test.That(t, out.String(), test.ShouldContainSubstring, "Installed Viam Visualizer 1.0.0")
		entries, err := os.ReadDir(root)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(entries), test.ShouldEqual, 1)
	})

	t.Run("darwin tarball", func(t *testing.T) {
		f := newFakeRelease(t, "1.0.0", "darwin", "arm64", map[string][]byte{"1.0.0": buildAppTarball(t, "bin")}, "")
		root := t.TempDir()
		bin, err := ensureVisualizer(ctx, f.opts(root, "darwin", "arm64", &bytes.Buffer{}))
		test.That(t, err, test.ShouldBeNil)
		data, err := os.ReadFile(bin)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, string(data), test.ShouldEqual, "bin")
		info, err := os.Stat(bin)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, info.Mode().Perm(), test.ShouldEqual, os.FileMode(0o755))
	})

	t.Run("checksum mismatch", func(t *testing.T) {
		f := newFakeRelease(t, "1.0.0", "linux", "amd64", map[string][]byte{"1.0.0": []byte("appimage")}, "deadbeef")
		root := t.TempDir()
		_, err := ensureVisualizer(ctx, f.opts(root, "linux", "amd64", &bytes.Buffer{}))
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "viam-visualizer-1.0.0-linux-x86_64.AppImage")
		entries, err := os.ReadDir(root)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(entries), test.ShouldEqual, 0)
	})

	t.Run("cached install makes no request", func(t *testing.T) {
		f := newFakeRelease(t, "1.0.0", "linux", "amd64", map[string][]byte{"1.0.0": []byte("appimage")}, "")
		root := t.TempDir()
		_, err := ensureVisualizer(ctx, f.opts(root, "linux", "amd64", &bytes.Buffer{}))
		test.That(t, err, test.ShouldBeNil)
		// Fresh the throttle stamp so the hint check stays quiet, then count.
		test.That(t, os.WriteFile(filepath.Join(root, visualizerLastCheckFile),
			[]byte(time.Now().UTC().Format(time.RFC3339)), 0o600), test.ShouldBeNil)
		before := f.requests.Load()
		bin, err := ensureVisualizer(ctx, f.opts(root, "linux", "amd64", &bytes.Buffer{}))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, bin, test.ShouldEqual, installedVisualizerBinary(root, "1.0.0", "linux"))
		test.That(t, f.requests.Load(), test.ShouldEqual, before)
	})

	t.Run("update installs newer release", func(t *testing.T) {
		f := newFakeRelease(t, "1.0.0", "linux", "amd64", map[string][]byte{
			"1.0.0": []byte("old"), "1.1.0": []byte("new"),
		}, "")
		root := t.TempDir()
		_, err := ensureVisualizer(ctx, f.opts(root, "linux", "amd64", &bytes.Buffer{}))
		test.That(t, err, test.ShouldBeNil)
		f.latest.Store("1.1.0")
		opts := f.opts(root, "linux", "amd64", &bytes.Buffer{})
		opts.update = true
		bin, err := ensureVisualizer(ctx, opts)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, bin, test.ShouldEqual, installedVisualizerBinary(root, "1.1.0", "linux"))
		data, err := os.ReadFile(bin)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, string(data), test.ShouldEqual, "new")
	})

	t.Run("hint when newer release exists", func(t *testing.T) {
		f := newFakeRelease(t, "2.0.0", "linux", "amd64", nil, "")
		root := t.TempDir()
		bin := installedVisualizerBinary(root, "1.0.0", "linux")
		test.That(t, os.MkdirAll(filepath.Dir(bin), 0o750), test.ShouldBeNil)
		test.That(t, os.WriteFile(bin, []byte("x"), 0o755), test.ShouldBeNil)
		var out bytes.Buffer
		got, err := ensureVisualizer(ctx, f.opts(root, "linux", "amd64", &out))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, got, test.ShouldEqual, bin)
		test.That(t, out.String(), test.ShouldContainSubstring, "A newer Viam Visualizer (2.0.0) is available")
	})

	t.Run("pinned version already installed", func(t *testing.T) {
		f := newFakeRelease(t, "9.9.9", "linux", "amd64", nil, "")
		root := t.TempDir()
		bin := installedVisualizerBinary(root, "1.0.0", "linux")
		test.That(t, os.MkdirAll(filepath.Dir(bin), 0o750), test.ShouldBeNil)
		test.That(t, os.WriteFile(bin, []byte("x"), 0o755), test.ShouldBeNil)
		opts := f.opts(root, "linux", "amd64", &bytes.Buffer{})
		opts.version = "1.0.0"
		got, err := ensureVisualizer(ctx, opts)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, got, test.ShouldEqual, bin)
		test.That(t, f.requests.Load(), test.ShouldEqual, 0)
	})
}
