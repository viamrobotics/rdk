package packages

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	pb "go.viam.com/api/app/packages/v1"
	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	putils "go.viam.com/rdk/robot/packages/testutils"
)

func newPackageManager(t *testing.T,
	client pb.PackageServiceClient,
	fakeServer *putils.FakePackagesClientAndGCSServer, logger logging.Logger, packageDir string,
) (string, ManagerSyncer) {
	fakeServer.Clear()

	if packageDir == "" {
		packageDir = t.TempDir()
	}
	logger.Info(packageDir)

	testCloudConfig := &config.Cloud{
		ID:     "some-id",
		Secret: "some-secret",
	}

	pm, err := NewCloudManager(testCloudConfig, client, packageDir, logger)
	test.That(t, err, test.ShouldBeNil)

	return packageDir, pm
}

func TestCloud(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	fakeServer, err := putils.NewFakePackageServer(ctx, logger)
	test.That(t, err, test.ShouldBeNil)
	defer utils.UncheckedErrorFunc(fakeServer.Shutdown)

	client, conn, err := fakeServer.Client(ctx)
	test.That(t, err, test.ShouldBeNil)
	defer utils.UncheckedErrorFunc(conn.Close)

	t.Run("missing package on server", func(t *testing.T) {
		packageDir, pm := newPackageManager(t, client, fakeServer, logger, "")
		defer utils.UncheckedErrorFunc(func() error { return pm.Close(context.Background()) })

		input := []config.PackageConfig{{Name: "some-name", Package: "org1/test-model", Version: "v1", Type: "ml_model"}}
		err = pm.Sync(ctx, input, []config.Module{})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "failed loading package url")

		// validate dir should be empty
		validatePackageDir(t, packageDir, []config.PackageConfig{})
	})

	t.Run("valid packages on server", func(t *testing.T) {
		packageDir, pm := newPackageManager(t, client, fakeServer, logger, "")
		defer utils.UncheckedErrorFunc(func() error { return pm.Close(context.Background()) })

		input := []config.PackageConfig{
			{Name: "some-name", Package: "org1/test-model", Version: "v1", Type: "ml_model"},
			{Name: "some-name-2", Package: "org1/test-model", Version: "v2", Type: "ml_model"},
		}
		fakeServer.StorePackage(input...)

		err = pm.Sync(ctx, input, []config.Module{})
		test.That(t, err, test.ShouldBeNil)

		// validate dir should be empty
		validatePackageDir(t, packageDir, input)
	})

	t.Run("sync continues on error", func(t *testing.T) {
		packageDir, pm := newPackageManager(t, client, fakeServer, logger, "")
		defer utils.UncheckedErrorFunc(func() error { return pm.Close(context.Background()) })

		input := []config.PackageConfig{
			{Name: "some-name", Package: "org1/test-model", Version: "v1", Type: "ml_model"},
			{Name: "some-name-2", Package: "org1/test-model", Version: "v2", Type: "ml_model"},
		}
		fakeServer.StorePackage(input[1]) // only store second

		err = pm.Sync(ctx, input, []config.Module{})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "failed loading package url for org1/test-model:v1")

		// validate dir should be empty
		validatePackageDir(t, packageDir, []config.PackageConfig{input[1]})
	})

	t.Run("sync re-downloads on missing status file", func(t *testing.T) {
		pkg := config.PackageConfig{Name: "some-name", Package: "org1/test-model", Version: "v1", Type: "module"}

		// create a package manager and Sync to download the package
		_, pm := newPackageManager(t, client, fakeServer, logger, "")
		defer utils.UncheckedErrorFunc(func() error { return pm.Close(context.Background()) })
		pkgDir := pkg.LocalDataDirectory(pm.(*cloudManager).packagesDir)
		module := config.Module{ExePath: pkgDir + "/some-text.txt"}
		fakeServer.StorePackage(pkg)
		err = pm.Sync(ctx, []config.PackageConfig{pkg}, []config.Module{module})
		test.That(t, err, test.ShouldBeNil)

		// grab ExePath ModTime for comparison
		info, err := os.Stat(module.ExePath)
		test.That(t, err, test.ShouldBeNil)
		modTime := info.ModTime()

		// grab sync file ModTime for comparison
		syncFileName := getSyncFileName(pkg.LocalDataDirectory(pm.(*cloudManager).packagesDir))
		_, err = os.Stat(syncFileName)
		test.That(t, err, test.ShouldBeNil)
		firstStatusFile, err := readStatusFile(pkg, pm.(*cloudManager).packagesDir)
		test.That(t, err, test.ShouldBeNil)

		// close previous package manager, make sure new PM *doesn't* re-download with intact package sync file
		pm.Close(ctx)
		_, pm = newPackageManager(t, client, fakeServer, logger, pm.(*cloudManager).packagesDir)
		defer utils.UncheckedErrorFunc(func() error { return pm.Close(context.Background()) })
		fakeServer.StorePackage(pkg)
		// sleep to make super sure modification time increments
		time.Sleep(10 * time.Millisecond)
		err = pm.Sync(ctx, []config.PackageConfig{pkg}, []config.Module{module})
		test.That(t, err, test.ShouldBeNil)

		// Ensure that files have not changed on 2nd sync
		info, err = os.Stat(module.ExePath)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, info.ModTime(), test.ShouldEqual, modTime)
		secondStatusFile, err := readStatusFile(pkg, pm.(*cloudManager).packagesDir)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, secondStatusFile.ModifiedTime, test.ShouldEqual, firstStatusFile.ModifiedTime)

		// close previous package manager, then corrupt the package sync file
		pm.Close(ctx)
		info, err = os.Stat(syncFileName)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, info.Size(), test.ShouldNotBeZeroValue)
		err = os.Remove(syncFileName)
		test.That(t, err, test.ShouldBeNil)

		// create fresh packageManager to simulate a reboot, i.e. so the system doesn't think the module is already managed.
		_, pm = newPackageManager(t, client, fakeServer, logger, pm.(*cloudManager).packagesDir)
		defer utils.UncheckedErrorFunc(func() error { return pm.Close(context.Background()) })
		fakeServer.StorePackage(pkg)
		err = pm.Sync(ctx, []config.PackageConfig{pkg}, []config.Module{module})
		test.That(t, err, test.ShouldBeNil)

		// test that Exe file exists, is non-empty, and modTime is different
		info, err = os.Stat(syncFileName)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, info.Size(), test.ShouldNotBeZeroValue)
		test.That(t, info.ModTime(), test.ShouldNotEqual, modTime)

		// test that Exe file exists, is non-empty, and modTime is different
		thirdStatusFile, err := readStatusFile(pkg, pm.(*cloudManager).packagesDir)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, thirdStatusFile.ModifiedTime, test.ShouldNotEqual, firstStatusFile.ModifiedTime)
	})

	t.Run("sync and clean should remove file", func(t *testing.T) {
		packageDir, pm := newPackageManager(t, client, fakeServer, logger, "")
		defer utils.UncheckedErrorFunc(func() error { return pm.Close(context.Background()) })

		input := []config.PackageConfig{
			{Name: "some-name-1", Package: "org1/test-model", Version: "v1", Type: "ml_model"},
			{Name: "some-name-2", Package: "org1/test-model", Version: "v2", Type: "ml_model"},
		}
		fakeServer.StorePackage(input...)

		// first sync
		err = pm.Sync(ctx, input, []config.Module{})
		test.That(t, err, test.ShouldBeNil)

		// validate dir should be empty
		validatePackageDir(t, packageDir, input)

		// second sync
		err = pm.Sync(ctx, []config.PackageConfig{input[1]}, []config.Module{})
		test.That(t, err, test.ShouldBeNil)

		validatePackageDir(t, packageDir, input)

		// clean dir
		err = pm.Cleanup(ctx)
		test.That(t, err, test.ShouldBeNil)

		// validate dir should be empty
		validatePackageDir(t, packageDir, []config.PackageConfig{input[1]})
	})

	t.Run("second sync should not call http server", func(t *testing.T) {
		packageDir, pm := newPackageManager(t, client, fakeServer, logger, "")
		defer utils.UncheckedErrorFunc(func() error { return pm.Close(context.Background()) })

		input := []config.PackageConfig{
			{Name: "some-name", Package: "org1/test-model", Version: "v1", Type: "ml_model"},
			{Name: "some-name-2", Package: "org1/test-model", Version: "v2", Type: "ml_model"},
		}
		fakeServer.StorePackage(input...)

		err = pm.Sync(ctx, input, []config.Module{})
		test.That(t, err, test.ShouldBeNil)

		getCount, downloadCount := fakeServer.RequestCounts()
		test.That(t, getCount, test.ShouldEqual, 2)
		test.That(t, downloadCount, test.ShouldEqual, 2)

		// validate dir should be empty
		validatePackageDir(t, packageDir, input)

		err = pm.Sync(ctx, input, []config.Module{})
		test.That(t, err, test.ShouldBeNil)

		// validate dir should be empty
		validatePackageDir(t, packageDir, input)

		getCount, downloadCount = fakeServer.RequestCounts()
		test.That(t, getCount, test.ShouldEqual, 2)
		test.That(t, downloadCount, test.ShouldEqual, 2)
	})

	t.Run("upgrade version", func(t *testing.T) {
		packageDir, pm := newPackageManager(t, client, fakeServer, logger, "")
		defer utils.UncheckedErrorFunc(func() error { return pm.Close(context.Background()) })

		input := []config.PackageConfig{
			{Name: "some-name-1", Package: "org1/test-model", Version: "v1", Type: "ml_model"},
		}
		fakeServer.StorePackage(input...)

		err = pm.Sync(ctx, input, []config.Module{})
		test.That(t, err, test.ShouldBeNil)

		validatePackageDir(t, packageDir, input)

		input[0].Version = "v2"
		fakeServer.StorePackage(input...)

		err = pm.Sync(ctx, input, []config.Module{})
		test.That(t, err, test.ShouldBeNil)

		err = pm.Cleanup(ctx)
		test.That(t, err, test.ShouldBeNil)

		validatePackageDir(t, packageDir, input)
	})

	t.Run("invalid checksum", func(t *testing.T) {
		packageDir, pm := newPackageManager(t, client, fakeServer, logger, "")
		defer utils.UncheckedErrorFunc(func() error { return pm.Close(context.Background()) })

		fakeServer.SetInvalidChecksum(true)

		input := []config.PackageConfig{
			{Name: "some-name-1", Package: "org1/test-model", Version: "v1", Type: "ml_model"},
		}
		fakeServer.StorePackage(input...)

		err = pm.Sync(ctx, input, []config.Module{})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "failed to decode")

		err = pm.Cleanup(ctx)
		test.That(t, err, test.ShouldBeNil)

		validatePackageDir(t, packageDir, []config.PackageConfig{})
	})

	t.Run("leading zeroes checksum", func(t *testing.T) {
		packageDir, pm := newPackageManager(t, client, fakeServer, logger, "")
		defer utils.UncheckedErrorFunc(func() error { return pm.Close(context.Background()) })

		fakeServer.SetChecksumWithLeadingZeroes(true)

		input := []config.PackageConfig{
			{Name: "some-name", Package: "org1/test-model", Version: "v1", Type: "ml_model"},
			{Name: "some-name-2", Package: "org1/test-model", Version: "v2", Type: "ml_model"},
		}
		fakeServer.StorePackage(input...)

		err = pm.Sync(ctx, input, []config.Module{})
		test.That(t, err, test.ShouldBeNil)

		// validate dir should be empty
		validatePackageDir(t, packageDir, input)
	})

	t.Run("invalid gcs download", func(t *testing.T) {
		packageDir, pm := newPackageManager(t, client, fakeServer, logger, "")
		defer utils.UncheckedErrorFunc(func() error { return pm.Close(context.Background()) })

		fakeServer.SetInvalidHTTPRes(true)

		input := []config.PackageConfig{
			{Name: "some-name-1", Package: "org1/test-model", Version: "v1", Type: "ml_model"},
		}
		fakeServer.StorePackage(input...)

		err = pm.Sync(ctx, input, []config.Module{})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "invalid status code 500")

		err = pm.Cleanup(ctx)
		test.That(t, err, test.ShouldBeNil)

		validatePackageDir(t, packageDir, []config.PackageConfig{})
	})

	t.Run("invalid tar", func(t *testing.T) {
		packageDir, pm := newPackageManager(t, client, fakeServer, logger, "")
		defer utils.UncheckedErrorFunc(func() error { return pm.Close(context.Background()) })

		fakeServer.SetInvalidTar(true)

		input := config.PackageConfig{Name: "some-name-1", Package: "org1/test-model", Version: "v1", Type: "ml_model"}
		fakeServer.StorePackage(input)

		// First sync should fail due to invalid tar
		err = pm.Sync(ctx, []config.PackageConfig{input}, []config.Module{})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "unexpected EOF")

		// Validate sync file exists and has failed status
		syncFileName := getSyncFileName(input.LocalDataDirectory(pm.(*cloudManager).packagesDir))
		_, err = os.Stat(syncFileName)
		test.That(t, err, test.ShouldBeNil)
		failedStatusFile, err := readStatusFile(input, pm.(*cloudManager).packagesDir)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, failedStatusFile.Status, test.ShouldEqual, syncStatusFailed)

		// Second sync should error and not re-download since package is present but marked as failed
		err = pm.Sync(ctx, []config.PackageConfig{input}, []config.Module{})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "failed to unzip")

		// Validate sync file exists and has failed status
		_, err = os.Stat(syncFileName)
		test.That(t, err, test.ShouldBeNil)
		statusFile, err := readStatusFile(input, pm.(*cloudManager).packagesDir)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, statusFile.Status, test.ShouldEqual, syncStatusFailed)

		// Validate only one get and one download call were made total even though we tried to sync invalid package twice
		getCount, downloadCount := fakeServer.RequestCounts()
		test.That(t, getCount, test.ShouldEqual, 1)
		test.That(t, downloadCount, test.ShouldEqual, 1)

		// Now set to valid tar and ensure next sync re-downloads and succeeds
		fakeServer.SetInvalidTar(false)

		// Change package version to mimic package update on server since tar won't un-invalidate itself
		input = config.PackageConfig{Name: "some-name-1", Package: "org1/test-model", Version: "v2", Type: "ml_model"}
		fakeServer.StorePackage(input)

		// Third sync should re-download and succeed
		err = pm.Sync(ctx, []config.PackageConfig{input}, []config.Module{})
		test.That(t, err, test.ShouldBeNil)

		// Validate sync file exists and has succeeded status
		_, err = os.Stat(syncFileName)
		test.That(t, err, test.ShouldBeNil)
		statusFile, err = readStatusFile(input, pm.(*cloudManager).packagesDir)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, statusFile.Status, test.ShouldEqual, syncStatusDone)

		// Validate two gets and two downloads were made total
		getCount, downloadCount = fakeServer.RequestCounts()
		test.That(t, getCount, test.ShouldEqual, 2)
		test.That(t, downloadCount, test.ShouldEqual, 2)

		err = pm.Cleanup(ctx)
		test.That(t, err, test.ShouldBeNil)

		validatePackageDir(t, packageDir, []config.PackageConfig{input})
	})
}

func validatePackageDir(t *testing.T, dir string, input []config.PackageConfig) {
	// t.Helper()

	// create maps to make lookups easier.
	bySanitizedName := make(map[string]*config.PackageConfig)
	byLogicalName := make(map[string]*config.PackageConfig)
	byType := make(map[string][]string)
	for _, pI := range input {
		p := pI
		bySanitizedName[p.SanitizedName()] = &p
		byLogicalName[p.Name] = &p
		pType := string(p.Type)
		byType[pType] = append(byType[pType], p.SanitizedName())
	}

	// check all known packages exist and are linked to the correct package dir.
	for _, p := range input {
		logicalPath := filepath.Join(dir, p.Name)
		dataPath := filepath.Join(dir, "data", string(p.Type), p.SanitizedName())

		info, err := os.Stat(logicalPath)
		test.That(t, err, test.ShouldBeNil)

		if !isSymLink(t, logicalPath) {
			t.Fatalf("found non symlink file in package dir %s at %s", info.Name(), logicalPath)
		}

		linkTarget, err := os.Readlink(logicalPath)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, linkTarget, test.ShouldEqual, dataPath)

		_, err = os.Stat(dataPath + statusFileExt)
		test.That(t, err, test.ShouldBeNil)

		info, err = os.Stat(dataPath)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, info.IsDir(), test.ShouldBeTrue)
	}

	// find any dangling files in the package dir or data sub dir
	// packageDir will contain either symlinks to the packages or the data directory.
	files, err := os.ReadDir(dir)
	test.That(t, err, test.ShouldBeNil)

	for _, f := range files {
		if isSymLink(t, filepath.Join(dir, f.Name())) {
			if _, ok := byLogicalName[f.Name()]; !ok {
				t.Fatalf("found unknown symlink in package dir %s", f.Name())
			}
			continue
		}

		// skip over any directories including the data
		if f.IsDir() && f.Name() == "data" {
			continue
		}

		t.Fatalf("found unknown file in package dir %s", f.Name())
	}

	typeFolders, err := os.ReadDir(filepath.Join(dir, "data"))
	test.That(t, err, test.ShouldBeNil)

	for _, typeFile := range typeFolders {
		expectedPackages, ok := byType[typeFile.Name()]
		if !ok {
			t.Errorf("found unknown file in package data dir %s", typeFile.Name())
		}
		foundFiles, err := os.ReadDir(filepath.Join(dir, "data", typeFile.Name()))
		test.That(t, err, test.ShouldBeNil)
		for _, packageFile := range foundFiles {
			// skip over status files
			if !slices.Contains(expectedPackages, packageFile.Name()) &&
				!strings.HasSuffix(packageFile.Name(), ".status.json") &&
				packageFile.Name() != "part" {
				t.Errorf("found unknown file in package %s dir %s", typeFile.Name(), packageFile.Name())
			}
		}
	}
}

func TestPackageRefs(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	fakeServer, err := putils.NewFakePackageServer(ctx, logger)
	test.That(t, err, test.ShouldBeNil)
	defer utils.UncheckedErrorFunc(fakeServer.Shutdown)

	client, conn, err := fakeServer.Client(ctx)
	test.That(t, err, test.ShouldBeNil)
	defer utils.UncheckedErrorFunc(conn.Close)

	packageDir, pm := newPackageManager(t, client, fakeServer, logger, "")
	defer utils.UncheckedErrorFunc(func() error { return pm.Close(context.Background()) })

	input := []config.PackageConfig{{Name: "some-name", Package: "org1/test-model", Version: "v1", Type: "ml_model"}}
	fakeServer.StorePackage(input...)

	err = pm.Sync(ctx, input, []config.Module{})
	test.That(t, err, test.ShouldBeNil)

	t.Run("PackagePath", func(t *testing.T) {
		// FYI: This test passes in CI but fails when ran on MacOS
		t.Run("valid package", func(t *testing.T) {
			if runtime.GOOS == "darwin" {
				t.Skip("skip on macos because file permissions are 755 instead of 777")
			}
			pPath, err := pm.PackagePath("some-name")
			test.That(t, err, test.ShouldBeNil)
			test.That(t, pPath, test.ShouldEqual, input[0].LocalDataDirectory(packageDir))
			putils.ValidateContentsOfPPackage(t, pPath)
		})

		t.Run("missing package", func(t *testing.T) {
			_, err = pm.PackagePath("not-valid")
			test.That(t, err, test.ShouldEqual, ErrPackageMissing)
		})

		t.Run("missing package for empty", func(t *testing.T) {
			_, err = pm.PackagePath("")
			test.That(t, err, test.ShouldEqual, ErrPackageMissing)
		})
	})
}

func isSymLink(t *testing.T, file string) bool {
	fileInfo, err := os.Lstat(file)
	test.That(t, err, test.ShouldBeNil)

	return fileInfo.Mode()&os.ModeSymlink != 0
}

func TestSafeLink(t *testing.T) {
	parentDir := "/some/parent"

	validate := func(in, expectedOut string, expectedErr error) {
		t.Helper()
		out, err := safeLink(parentDir, in)
		if expectedErr == nil {
			test.That(t, err, test.ShouldBeNil)
			test.That(t, out, test.ShouldEqual, expectedOut)
		} else {
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, expectedErr.Error())
		}
	}

	validate("sub/dir", "sub/dir", nil)
	validate("sub/../dir", "sub/../dir", nil)
	validate("sub/../../dir", "", errors.New("unsafe path join"))
	validate("/root", "", errors.New("unsafe path link"))
}

func TestMissingDirEntry(t *testing.T) {
	file, err := os.CreateTemp("", "missing-dir-entry")
	test.That(t, err, test.ShouldBeNil)
	head := tar.Header{
		Name: "subdir/file.md",
		Size: 1,
		Mode: 0o600,
		Uid:  1000,
		Gid:  1000,
	}
	gzipWriter := gzip.NewWriter(file)
	writer := tar.NewWriter(gzipWriter)
	writer.WriteHeader(&head)
	writer.Write([]byte("x"))
	writer.Close()
	gzipWriter.Close()
	file.Close()
	defer os.Remove(file.Name())
	dest, err := os.MkdirTemp("", "missing-dir-entry")
	defer os.RemoveAll(dest)
	test.That(t, err, test.ShouldBeNil)
	// The inner MkdirAll in unpackFile will fail with 'permission denied' if we
	// create the subdirectory with the wrong permissions.
	err = unpackFile(context.Background(), file.Name(), dest)
	test.That(t, err, test.ShouldBeNil)
}

func TestTrimLeadingZeroes(t *testing.T) {
	testCases := []struct {
		name     string
		input    []byte
		expected []byte
	}{
		{
			name:     "Empty slice",
			input:    []byte{},
			expected: []byte{},
		},
		{
			name:     "Single zero byte",
			input:    []byte{0x00},
			expected: []byte{0x00},
		},
		{
			name:     "Leading zeroes trimmed",
			input:    []byte{0x00, 0x00, 0x03, 0x04, 0x05},
			expected: []byte{0x03, 0x04, 0x05},
		},
		{
			name:     "All zero bytes",
			input:    []byte{0x00, 0x00, 0x00},
			expected: []byte{0x00},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			test.That(t, trimLeadingZeroes(tc.input), test.ShouldResemble, tc.expected)
		})
	}
}

type testHandler struct {
	modTime time.Time
	content string
	lengths []string // history of Content-Length
}

func (th *testHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	hash := crc32Hash()
	hash.Write([]byte(th.content))
	w.Header().Add("x-goog-hash", "crc32c="+base64.StdEncoding.EncodeToString(hash.Sum(nil)))
	http.ServeContent(w, req, "mydownload", th.modTime, strings.NewReader(th.content))
	if req.Method == http.MethodGet {
		th.lengths = append(th.lengths, w.Header().Get("Content-Length"))
	}
}

func TestDownloadFileWithChecksum(t *testing.T) {
	logger := logging.NewTestLogger(t)
	packagesDir := t.TempDir()
	pm := &cloudManager{
		Named:           InternalServiceName.AsNamed(),
		httpClient:      *http.DefaultClient,
		packagesDir:     packagesDir,
		packagesDataDir: filepath.Join(packagesDir, "data"),
		logger:          logger,
	}

	handler := &testHandler{
		modTime: time.Now(),
		content: strings.Repeat("hello ", 10),
	}
	server := httptest.NewServer(handler)
	defer server.Close()

	t.Run("complete", func(t *testing.T) {
		dest := filepath.Join(packagesDir, "download1")
		_, _, err := pm.downloadFileWithChecksum(t.Context(), server.URL+"/download1", dest, "id", "secret")
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("resumable", func(t *testing.T) {
		maxBytesForTesting = int64(len(handler.content)/2) + 1
		t.Cleanup(func() { maxBytesForTesting = 0 })

		dest := filepath.Join(packagesDir, "download2")

		// first attempt fails midway because of maxBytesForTesting
		_, _, err := pm.downloadFileWithChecksum(t.Context(), server.URL+"/download2", dest, "id", "secret")
		test.That(t, err.Error(), test.ShouldContainSubstring, "short write")

		// second attempt finishes
		_, _, err = pm.downloadFileWithChecksum(t.Context(), server.URL+"/download2", dest, "id", "secret")
		test.That(t, err, test.ShouldBeNil)
		// check the length
		test.That(t, handler.lengths[len(handler.lengths)-1], test.ShouldEqual,
			strconv.Itoa(len(handler.content)-int(maxBytesForTesting)))
	})

	t.Run("partial-cleanups", func(t *testing.T) {
		// create two partial download folders, one over the max age limit, the other under
		p1 := filepath.Join(pm.packagesDataDir, "packagetype", partialsDirName, "partial1")
		p2 := filepath.Join(pm.packagesDataDir, "packagetype", partialsDirName, "partial2")
		err := errors.Join(
			os.MkdirAll(p1, 0o750),
			os.MkdirAll(p2, 0o750),
		)
		test.That(t, err, test.ShouldBeNil)
		now := time.Now()
		err = os.Chtimes(p1, now.Add(-maxPartialAge-time.Hour), now.Add(-maxPartialAge-time.Hour))
		test.That(t, err, test.ShouldBeNil)

		err = commonCleanup(logger, nil, pm.packagesDataDir)
		test.That(t, err, test.ShouldBeNil)
		_, err = os.Stat(p1)
		var pathError *os.PathError
		test.That(t, errors.As(err, &pathError), test.ShouldBeTrue)
		_, err = os.Stat(p2)
		test.That(t, err, test.ShouldBeNil)
	})
}
