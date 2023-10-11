package packages

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/edaniels/golog"
	pb "go.viam.com/api/app/packages/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	"golang.org/x/exp/slices"

	"go.viam.com/rdk/config"
	putils "go.viam.com/rdk/robot/packages/testutils"
)

func newPackageManager(t *testing.T,
	client pb.PackageServiceClient,
	fakeServer *putils.FakePackagesClientAndGCSServer, logger golog.Logger,
) (string, ManagerSyncer) {
	fakeServer.Clear()

	packageDir := t.TempDir()
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
	logger := golog.NewTestLogger(t)

	fakeServer, err := putils.NewFakePackageServer(ctx, logger)
	test.That(t, err, test.ShouldBeNil)
	defer utils.UncheckedErrorFunc(fakeServer.Shutdown)

	client, conn, err := fakeServer.Client(ctx)
	test.That(t, err, test.ShouldBeNil)
	defer utils.UncheckedErrorFunc(conn.Close)

	t.Run("missing package on server", func(t *testing.T) {
		packageDir, pm := newPackageManager(t, client, fakeServer, logger)
		defer utils.UncheckedErrorFunc(func() error { return pm.Close(context.Background()) })

		input := []config.PackageConfig{{Name: "some-name", Package: "org1/test-model", Version: "v1", Type: "ml_model"}}
		err = pm.Sync(ctx, input)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "failed loading package url")

		// validate dir should be empty
		validatePackageDir(t, packageDir, []config.PackageConfig{})
	})

	t.Run("valid packages on server", func(t *testing.T) {
		packageDir, pm := newPackageManager(t, client, fakeServer, logger)
		defer utils.UncheckedErrorFunc(func() error { return pm.Close(context.Background()) })

		input := []config.PackageConfig{
			{Name: "some-name", Package: "org1/test-model", Version: "v1", Type: "ml_model"},
			{Name: "some-name-2", Package: "org1/test-model", Version: "v2", Type: "ml_model"},
		}
		fakeServer.StorePackage(input...)

		err = pm.Sync(ctx, input)
		test.That(t, err, test.ShouldBeNil)

		// validate dir should be empty
		validatePackageDir(t, packageDir, input)
	})

	t.Run("sync continues on error", func(t *testing.T) {
		packageDir, pm := newPackageManager(t, client, fakeServer, logger)
		defer utils.UncheckedErrorFunc(func() error { return pm.Close(context.Background()) })

		input := []config.PackageConfig{
			{Name: "some-name", Package: "org1/test-model", Version: "v1", Type: "ml_model"},
			{Name: "some-name-2", Package: "org1/test-model", Version: "v2", Type: "ml_model"},
		}
		fakeServer.StorePackage(input[1]) // only store second

		err = pm.Sync(ctx, input)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "failed loading package url for org1/test-model:v1")

		// validate dir should be empty
		validatePackageDir(t, packageDir, []config.PackageConfig{input[1]})
	})

	t.Run("sync and clean should remove file", func(t *testing.T) {
		packageDir, pm := newPackageManager(t, client, fakeServer, logger)
		defer utils.UncheckedErrorFunc(func() error { return pm.Close(context.Background()) })

		input := []config.PackageConfig{
			{Name: "some-name-1", Package: "org1/test-model", Version: "v1", Type: "ml_model"},
			{Name: "some-name-2", Package: "org1/test-model", Version: "v2", Type: "ml_model"},
		}
		fakeServer.StorePackage(input...)

		// first sync
		err = pm.Sync(ctx, input)
		test.That(t, err, test.ShouldBeNil)

		// validate dir should be empty
		validatePackageDir(t, packageDir, input)

		// second sync
		err = pm.Sync(ctx, []config.PackageConfig{input[1]})
		test.That(t, err, test.ShouldBeNil)

		validatePackageDir(t, packageDir, input)

		// clean dir
		err = pm.Cleanup(ctx)
		test.That(t, err, test.ShouldBeNil)

		// validate dir should be empty
		validatePackageDir(t, packageDir, []config.PackageConfig{input[1]})
	})

	t.Run("second sync should not call http server", func(t *testing.T) {
		packageDir, pm := newPackageManager(t, client, fakeServer, logger)
		defer utils.UncheckedErrorFunc(func() error { return pm.Close(context.Background()) })

		input := []config.PackageConfig{
			{Name: "some-name", Package: "org1/test-model", Version: "v1", Type: "ml_model"},
			{Name: "some-name-2", Package: "org1/test-model", Version: "v2", Type: "ml_model"},
		}
		fakeServer.StorePackage(input...)

		err = pm.Sync(ctx, input)
		test.That(t, err, test.ShouldBeNil)

		getCount, downloadCount := fakeServer.RequestCounts()
		test.That(t, getCount, test.ShouldEqual, 2)
		test.That(t, downloadCount, test.ShouldEqual, 2)

		// validate dir should be empty
		validatePackageDir(t, packageDir, input)

		err = pm.Sync(ctx, input)
		test.That(t, err, test.ShouldBeNil)

		// validate dir should be empty
		validatePackageDir(t, packageDir, input)

		getCount, downloadCount = fakeServer.RequestCounts()
		test.That(t, getCount, test.ShouldEqual, 2)
		test.That(t, downloadCount, test.ShouldEqual, 2)
	})

	t.Run("upgrade version", func(t *testing.T) {
		packageDir, pm := newPackageManager(t, client, fakeServer, logger)
		defer utils.UncheckedErrorFunc(func() error { return pm.Close(context.Background()) })

		input := []config.PackageConfig{
			{Name: "some-name-1", Package: "org1/test-model", Version: "v1", Type: "ml_model"},
		}
		fakeServer.StorePackage(input...)

		err = pm.Sync(ctx, input)
		test.That(t, err, test.ShouldBeNil)

		validatePackageDir(t, packageDir, input)

		input[0].Version = "v2"
		fakeServer.StorePackage(input...)

		err = pm.Sync(ctx, input)
		test.That(t, err, test.ShouldBeNil)

		err = pm.Cleanup(ctx)
		test.That(t, err, test.ShouldBeNil)

		validatePackageDir(t, packageDir, input)
	})

	t.Run("invalid checksum", func(t *testing.T) {
		packageDir, pm := newPackageManager(t, client, fakeServer, logger)
		defer utils.UncheckedErrorFunc(func() error { return pm.Close(context.Background()) })

		fakeServer.SetInvalidChecksum(true)

		input := []config.PackageConfig{
			{Name: "some-name-1", Package: "org1/test-model", Version: "v1", Type: "ml_model"},
		}
		fakeServer.StorePackage(input...)

		err = pm.Sync(ctx, input)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "download did not match expected hash")

		err = pm.Cleanup(ctx)
		test.That(t, err, test.ShouldBeNil)

		validatePackageDir(t, packageDir, []config.PackageConfig{})
	})

	t.Run("invalid gcs download", func(t *testing.T) {
		packageDir, pm := newPackageManager(t, client, fakeServer, logger)
		defer utils.UncheckedErrorFunc(func() error { return pm.Close(context.Background()) })

		fakeServer.SetInvalidHTTPRes(true)

		input := []config.PackageConfig{
			{Name: "some-name-1", Package: "org1/test-model", Version: "v1", Type: "ml_model"},
		}
		fakeServer.StorePackage(input...)

		err = pm.Sync(ctx, input)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "invalid status code 500")

		err = pm.Cleanup(ctx)
		test.That(t, err, test.ShouldBeNil)

		validatePackageDir(t, packageDir, []config.PackageConfig{})
	})

	t.Run("invalid tar", func(t *testing.T) {
		packageDir, pm := newPackageManager(t, client, fakeServer, logger)
		defer utils.UncheckedErrorFunc(func() error { return pm.Close(context.Background()) })

		fakeServer.SetInvalidTar(true)

		input := []config.PackageConfig{
			{Name: "some-name-1", Package: "org1/test-model", Version: "v1", Type: "ml_model"},
		}
		fakeServer.StorePackage(input...)

		err = pm.Sync(ctx, input)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "unexpected EOF")

		err = pm.Cleanup(ctx)
		test.That(t, err, test.ShouldBeNil)

		validatePackageDir(t, packageDir, []config.PackageConfig{})
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
		dataPath := filepath.Join(dir, ".data", string(p.Type), p.SanitizedName())

		info, err := os.Stat(logicalPath)
		test.That(t, err, test.ShouldBeNil)

		if !isSymLink(t, logicalPath) {
			t.Fatalf("found non symlink file in package dir %s at %s", info.Name(), logicalPath)
		}

		linkTarget, err := os.Readlink(logicalPath)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, linkTarget, test.ShouldEqual, dataPath)

		info, err = os.Stat(dataPath)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, info.IsDir(), test.ShouldBeTrue)
	}

	// find any dangling files in the package dir or .data sub dir
	// packageDir will contain either symlinks to the packages or the .data directory.
	files, err := os.ReadDir(dir)
	test.That(t, err, test.ShouldBeNil)

	for _, f := range files {
		if isSymLink(t, filepath.Join(dir, f.Name())) {
			if _, ok := byLogicalName[f.Name()]; !ok {
				t.Fatalf("found unknown symlink in package dir %s", f.Name())
			}
			continue
		}

		// skip over any directories including the .data
		if f.IsDir() && f.Name() == ".data" {
			continue
		}

		t.Fatalf("found unknown file in package dir %s", f.Name())
	}

	typeFolders, err := os.ReadDir(filepath.Join(dir, ".data"))
	test.That(t, err, test.ShouldBeNil)

	for _, typeFile := range typeFolders {
		expectedPackages, ok := byType[typeFile.Name()]
		if !ok {
			t.Errorf("found unknown file in package data dir %s", typeFile.Name())
		}
		foundFiles, err := os.ReadDir(filepath.Join(dir, ".data", typeFile.Name()))
		test.That(t, err, test.ShouldBeNil)
		for _, packageFile := range foundFiles {
			if !slices.Contains(expectedPackages, packageFile.Name()) {
				t.Errorf("found unknown file in package %s dir %s", typeFile.Name(), packageFile.Name())
			}
		}
	}
}

func TestPackageRefs(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	fakeServer, err := putils.NewFakePackageServer(ctx, logger)
	test.That(t, err, test.ShouldBeNil)
	defer utils.UncheckedErrorFunc(fakeServer.Shutdown)

	client, conn, err := fakeServer.Client(ctx)
	test.That(t, err, test.ShouldBeNil)
	defer utils.UncheckedErrorFunc(conn.Close)

	packageDir, pm := newPackageManager(t, client, fakeServer, logger)
	defer utils.UncheckedErrorFunc(func() error { return pm.Close(context.Background()) })

	input := []config.PackageConfig{{Name: "some-name", Package: "org1/test-model", Version: "v1", Type: "ml_model"}}
	fakeServer.StorePackage(input...)

	err = pm.Sync(ctx, input)
	test.That(t, err, test.ShouldBeNil)

	t.Run("PackagePath", func(t *testing.T) {
		t.Run("valid package", func(t *testing.T) {
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

func TestSafeJoin(t *testing.T) {
	parentDir := "/some/parent"

	validate := func(in, expectedOut string, expectedErr error) {
		t.Helper()

		out, err := safeJoin(parentDir, in)
		if expectedErr == nil {
			test.That(t, err, test.ShouldBeNil)
			test.That(t, out, test.ShouldEqual, expectedOut)
		} else {
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, expectedErr.Error())
		}
	}

	validate("sub/dir", "/some/parent/sub/dir", nil)
	validate("/other/parent", "/some/parent/other/parent", nil)
	validate("../../../root", "", errors.New("unsafe path join"))
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
