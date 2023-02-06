package packages

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/edaniels/golog"
	pb "go.viam.com/api/app/packages/v1"
	"go.viam.com/test"
	"go.viam.com/utils"

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

	pm, err := NewCloudManager(client, packageDir, logger)
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
		defer utils.UncheckedErrorFunc(pm.Close)

		input := []config.PackageConfig{{Name: "some-name", Package: "test-model", Version: "v1"}}
		err = pm.Sync(ctx, input)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "failed loading package url")

		// validate dir should be empty
		validatePackageDir(t, packageDir, []config.PackageConfig{})
	})

	t.Run("valid packages on server", func(t *testing.T) {
		packageDir, pm := newPackageManager(t, client, fakeServer, logger)
		defer utils.UncheckedErrorFunc(pm.Close)

		input := []config.PackageConfig{
			{Name: "some-name", Package: "test-model", Version: "v1"},
			{Name: "some-name-2", Package: "test-model", Version: "v2"},
		}
		fakeServer.StorePackage(input...)

		err = pm.Sync(ctx, input)
		test.That(t, err, test.ShouldBeNil)

		// validate dir should be empty
		validatePackageDir(t, packageDir, input)
	})

	t.Run("sync continues on error", func(t *testing.T) {
		packageDir, pm := newPackageManager(t, client, fakeServer, logger)
		defer utils.UncheckedErrorFunc(pm.Close)

		input := []config.PackageConfig{
			{Name: "some-name", Package: "test-model", Version: "v1"},
			{Name: "some-name-2", Package: "test-model", Version: "v2"},
		}
		fakeServer.StorePackage(input[1]) // only store second

		err = pm.Sync(ctx, input)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "failed loading package url for test-model:v1")

		// validate dir should be empty
		validatePackageDir(t, packageDir, []config.PackageConfig{input[1]})
	})

	t.Run("sync and clean should remove file", func(t *testing.T) {
		packageDir, pm := newPackageManager(t, client, fakeServer, logger)
		defer utils.UncheckedErrorFunc(pm.Close)

		input := []config.PackageConfig{
			{Name: "some-name-1", Package: "test-model", Version: "v1"},
			{Name: "some-name-2", Package: "test-model", Version: "v2"},
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
		defer utils.UncheckedErrorFunc(pm.Close)

		input := []config.PackageConfig{
			{Name: "some-name", Package: "test-model", Version: "v1"},
			{Name: "some-name-2", Package: "test-model", Version: "v2"},
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
		defer utils.UncheckedErrorFunc(pm.Close)

		input := []config.PackageConfig{
			{Name: "some-name-1", Package: "test-model", Version: "v1"},
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
		defer utils.UncheckedErrorFunc(pm.Close)

		fakeServer.SetInvalidChecksum(true)

		input := []config.PackageConfig{
			{Name: "some-name-1", Package: "test-model", Version: "v1"},
		}
		fakeServer.StorePackage(input...)

		err = pm.Sync(ctx, input)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "download did not match expected hash")

		validatePackageDir(t, packageDir, []config.PackageConfig{})
	})

	t.Run("invalid gcs download", func(t *testing.T) {
		packageDir, pm := newPackageManager(t, client, fakeServer, logger)
		defer utils.UncheckedErrorFunc(pm.Close)

		fakeServer.SetInvalidHTTPRes(true)

		input := []config.PackageConfig{
			{Name: "some-name-1", Package: "test-model", Version: "v1"},
		}
		fakeServer.StorePackage(input...)

		err = pm.Sync(ctx, input)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "invalid status code 500")

		validatePackageDir(t, packageDir, []config.PackageConfig{})
	})

	t.Run("invalid tar", func(t *testing.T) {
		packageDir, pm := newPackageManager(t, client, fakeServer, logger)
		defer utils.UncheckedErrorFunc(pm.Close)

		fakeServer.SetInvalidTar(true)

		input := []config.PackageConfig{
			{Name: "some-name-1", Package: "test-model", Version: "v1"},
		}
		fakeServer.StorePackage(input...)

		err = pm.Sync(ctx, input)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "unexpected EOF")

		validatePackageDir(t, packageDir, []config.PackageConfig{})
	})
}

func validatePackageDir(t *testing.T, dir string, input []config.PackageConfig) {
	t.Helper()

	// create maps to make lookups easier.
	byPackageHash := make(map[string]*config.PackageConfig)
	byLogicalName := make(map[string]*config.PackageConfig)
	for _, pI := range input {
		p := pI
		byPackageHash[hashName(p)] = &p
		byLogicalName[p.Name] = &p
	}

	// check all known packages exist and are linked to the correct package dir.
	for _, p := range input {
		logicalPath := path.Join(dir, p.Name)
		dataPath := path.Join(dir, fmt.Sprintf(".data/%s", hashName(p)))

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
		if isSymLink(t, path.Join(dir, f.Name())) {
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

	files, err = os.ReadDir(path.Join(dir, ".data"))
	test.That(t, err, test.ShouldBeNil)

	for _, f := range files {
		if _, ok := byPackageHash[f.Name()]; ok {
			continue
		}
		t.Errorf("found unknown file in package data dir %s", f.Name())
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
	defer utils.UncheckedErrorFunc(pm.Close)

	input := []config.PackageConfig{{Name: "some-name", Package: "test-model", Version: "v1"}}
	fakeServer.StorePackage(input...)

	err = pm.Sync(ctx, input)
	test.That(t, err, test.ShouldBeNil)

	t.Run("PackagePath", func(t *testing.T) {
		t.Run("valid package", func(t *testing.T) {
			pPath, err := pm.PackagePath("some-name")
			test.That(t, err, test.ShouldBeNil)
			test.That(t, pPath, test.ShouldEqual, path.Join(packageDir, "some-name"))
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

	t.Run("RefPath", func(t *testing.T) {
		t.Run("empty path", func(t *testing.T) {
			pPath, err := pm.RefPath("")
			test.That(t, err, test.ShouldBeNil)
			test.That(t, pPath, test.ShouldEqual, "")
		})

		t.Run("non-ref absolute path", func(t *testing.T) {
			pPath, err := pm.RefPath("/some/absolute/path")
			test.That(t, err, test.ShouldBeNil)
			test.That(t, pPath, test.ShouldEqual, "/some/absolute/path")
		})

		t.Run("non-ref relative path", func(t *testing.T) {
			pPath, err := pm.RefPath("some/absolute/path")
			test.That(t, err, test.ShouldBeNil)
			test.That(t, pPath, test.ShouldEqual, "some/absolute/path")
		})

		t.Run("non-ref relative path with backtrack", func(t *testing.T) {
			pPath, err := pm.RefPath("some/../absolute/path")
			test.That(t, err, test.ShouldBeNil)
			test.That(t, pPath, test.ShouldEqual, "some/../absolute/path")
		})

		t.Run("valid ref, empty package path", func(t *testing.T) {
			pPath, err := pm.RefPath("${packages.some-name}")
			test.That(t, err, test.ShouldBeNil)
			test.That(t, pPath, test.ShouldEqual, path.Join(packageDir, "some-name"))
		})

		t.Run("valid ref, with package path", func(t *testing.T) {
			pPath, err := pm.RefPath("${packages.some-name}/some/path")
			test.That(t, err, test.ShouldBeNil)
			test.That(t, pPath, test.ShouldEqual, path.Join(packageDir, "some-name", "some/path"))
		})

		t.Run("valid ref, ensure no escape from package path", func(t *testing.T) {
			pPath, err := pm.RefPath("${packages.some-name}/../../../some-other-package/some/path")
			test.That(t, err, test.ShouldBeNil)
			test.That(t, pPath, test.ShouldEqual, path.Join(packageDir, "some-name", "some-other-package/some/path"))
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
