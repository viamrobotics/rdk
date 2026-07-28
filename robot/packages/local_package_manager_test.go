package packages

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	rUtils "go.viam.com/rdk/utils"
	"go.viam.com/rdk/utils/diskusage"
)

// testTarPath points to a tarball that tests can use.
const testTarPath = "test_package.tar.gz"

// writeSingleFileTarGz writes a .tar.gz holding one regular file of the given size into a temp
// dir and returns its path. The contents are zeros (gzip compresses them away), so a large
// logical size stays cheap on disk.
func writeSingleFileTarGz(t *testing.T, name string, size int64) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "package.tar.gz")
	f, err := os.Create(path)
	test.That(t, err, test.ShouldBeNil)
	gzw := gzip.NewWriter(f)
	tw := tar.NewWriter(gzw)
	test.That(t, tw.WriteHeader(&tar.Header{Name: name, Size: size, Mode: 0o600}), test.ShouldBeNil)
	_, err = io.Copy(tw, io.LimitReader(zeroReader{}, size))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, tw.Close(), test.ShouldBeNil)
	test.That(t, gzw.Close(), test.ShouldBeNil)
	test.That(t, f.Close(), test.ShouldBeNil)
	return path
}

// zeroReader is an infinite source of zero bytes, bounded by an io.LimitReader.
type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

func TestLocalManagerUtils(t *testing.T) {
	tmp := t.TempDir()
	mgr, err := NewLocalManager(
		filepath.Join(tmp, "pkg"),
		logging.NewTestLogger(t),
	)
	test.That(t, err, test.ShouldBeNil)
	local := mgr.(*localManager)

	t.Run("fileCopyHelper", func(t *testing.T) {
		f, err := os.Create(filepath.Join(tmp, "source"))
		test.That(t, err, test.ShouldBeNil)
		_, err = f.WriteString("hello")
		test.That(t, err, test.ShouldBeNil)
		err = f.Close()
		test.That(t, err, test.ShouldBeNil)
		dest := filepath.Join(tmp, "dest")
		test.That(t, err, test.ShouldBeNil)
		_, _, err = local.fileCopyHelper(context.Background(), f.Name(), dest)
		test.That(t, err, test.ShouldBeNil)
		stat, err := os.Stat(dest)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, stat.Size(), test.ShouldEqual, 5)
	})

	t.Run("fileCopyHelper disk guard", func(t *testing.T) {
		// fileCopyHelper pre-filters local copies by sizing the check off the source file (its
		// size plus the MinFreeBytes floor) and refuses when blocking is on.
		src := filepath.Join(tmp, "guard-source")
		size := int64(diskusage.MinFreeBytes) + 4242
		test.That(t, os.WriteFile(src, make([]byte, size), 0o600), test.ShouldBeNil)

		orig := enoughFreeSpace
		t.Cleanup(func() { enoughFreeSpace = orig })

		var gotRequired uint64
		enoughFreeSpace = func(_ string, minBytes uint64) (bool, uint64, error) {
			gotRequired = minBytes
			return false, 5, nil
		}

		t.Run("refuses when low and blocking enabled", func(t *testing.T) {
			t.Setenv(rUtils.ViamEnableDiskSpaceBlockEnvVar, "true")
			dest := filepath.Join(t.TempDir(), "guard-dest")
			_, _, err := local.fileCopyHelper(context.Background(), src, dest)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "not enough free disk space")
			test.That(t, gotRequired, test.ShouldEqual, uint64(size)+diskusage.MinFreeBytes)
			_, statErr := os.Stat(dest)
			test.That(t, os.IsNotExist(statErr), test.ShouldBeTrue)
		})

		t.Run("proceeds when low but blocking disabled (log-only default)", func(t *testing.T) {
			dest := filepath.Join(t.TempDir(), "guard-dest")
			_, _, err := local.fileCopyHelper(context.Background(), src, dest)
			test.That(t, err, test.ShouldBeNil)
			stat, err := os.Stat(dest)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, stat.Size(), test.ShouldEqual, size)
		})
	})

	t.Run("unpackFile disk guard", func(t *testing.T) {
		// unpackFile checks free space incrementally as it extracts and aborts when low and
		// blocking is on. The seeded counter fires the check before the first file, so even this
		// tiny tarball exercises it.
		orig := enoughFreeSpace
		t.Cleanup(func() { enoughFreeSpace = orig })
		enoughFreeSpace = func(_ string, _ uint64) (bool, uint64, error) {
			return false, 5, nil // always low
		}

		t.Run("refuses when low and blocking enabled", func(t *testing.T) {
			t.Setenv(rUtils.ViamEnableDiskSpaceBlockEnvVar, "true")
			dest := t.TempDir()
			err := unpackFile(context.Background(), logging.NewTestLogger(t), testTarPath, dest)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldContainSubstring, "not enough free disk space")
			// installPackage relies on errors.Is to avoid mislabeling this as a corrupt-archive
			// "try a different version" failure, so the sentinel must propagate.
			test.That(t, errors.Is(err, errInsufficientDiskSpace), test.ShouldBeTrue)
		})

		t.Run("proceeds when low but blocking disabled (log-only default)", func(t *testing.T) {
			dest := t.TempDir()
			err := unpackFile(context.Background(), logging.NewTestLogger(t), testTarPath, dest)
			test.That(t, err, test.ShouldBeNil)
			_, statErr := os.Stat(filepath.Join(dest, "run.sh"))
			test.That(t, statErr, test.ShouldBeNil)
		})

		t.Run("required space includes the upcoming file size", func(t *testing.T) {
			// Regression guard for the per-file requirement: the check must ask for the member's
			// size plus the reserved floor, not the floor alone, so a single large file can't push
			// the volume below MinFreeBytes. Use a member larger than unpackDiskCheckInterval so it
			// trips the check on its own.
			fileSize := int64(unpackDiskCheckInterval) + 4242
			tarPath := writeSingleFileTarGz(t, "big-member", fileSize)

			var gotRequired uint64
			enoughFreeSpace = func(_ string, minBytes uint64) (bool, uint64, error) {
				gotRequired = minBytes
				return false, 5, nil
			}
			t.Setenv(rUtils.ViamEnableDiskSpaceBlockEnvVar, "true")

			err := unpackFile(context.Background(), logging.NewTestLogger(t), tarPath, t.TempDir())
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, errors.Is(err, errInsufficientDiskSpace), test.ShouldBeTrue)
			test.That(t, gotRequired, test.ShouldEqual, uint64(fileSize)+diskusage.MinFreeBytes)
		})
	})

	t.Run("getAddedAndChanged", func(t *testing.T) {
		tmp := t.TempDir()
		logger := logging.NewTestLogger(t)
		mgr, err := NewLocalManager(filepath.Join(tmp, "pkg"), logger)
		test.That(t, err, test.ShouldBeNil)
		local := mgr.(*localManager)

		mod1 := config.Module{Name: "stays-the-same", Type: config.ModuleTypeLocal}
		mod2 := config.Module{Name: "gets-changed", Type: config.ModuleTypeLocal}
		mod3 := config.Module{Name: "gets-added", Type: config.ModuleTypeLocal}
		m := managedModuleMap{
			mod1.Name:      &managedModule{module: mod1},
			mod2.Name:      &managedModule{module: mod2},
			"gets-removed": &managedModule{module: config.Module{Name: "gets-removed"}},
		}
		mod2.ExePath = "changed"

		pkg1, err := mod1.SyntheticPackage()
		test.That(t, err, test.ShouldBeNil)
		pkg1StatusFile := packageSyncFile{
			PackageID:       pkg1.Package,
			Version:         pkg1.Version,
			ModifiedTime:    time.Now(),
			Status:          syncStatusDone,
			TarballChecksum: "",
		}

		// Create the parent directory for the package type if it doesn't exist
		err = os.MkdirAll(pkg1.LocalDataParentDirectory(local.packagesDir), 0o700)
		test.That(t, err, test.ShouldBeNil)

		err = writeStatusFile(pkg1, pkg1StatusFile, local.packagesDir)
		test.That(t, err, test.ShouldBeNil)

		existing, changed := m.getAddedAndChanged([]config.Module{
			mod1, mod2, mod3,
		}, local.packagesDir, logging.NewTestLogger(t))
		test.That(t, existing, test.ShouldHaveLength, 1)
		test.That(t, existing[mod1.Name], test.ShouldNotBeNil)
		test.That(t, changed, test.ShouldResemble, []config.Module{mod2, mod3})
	})

	t.Run("newerOrMissing", func(t *testing.T) {
		tmp := t.TempDir()
		for _, name := range []string{"one", "two", "three", "four", "five", "six"} {
			path := filepath.Join(tmp, name)
			f, err := os.Create(path)
			test.That(t, err, test.ShouldBeNil)
			_, err = f.WriteString(name)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, f.Close(), test.ShouldBeNil)
			time.Sleep(time.Millisecond * 10)
		}

		t.Run("both-missing", func(t *testing.T) {
			_, err := newerOrMissing(filepath.Join(tmp, "missing1"), filepath.Join(tmp, "missing2"))
			test.That(t, err, test.ShouldNotBeNil)
		})

		t.Run("source-missing", func(t *testing.T) {
			_, err = newerOrMissing(filepath.Join(tmp, "missing1"), filepath.Join(tmp, "one"))
			test.That(t, err, test.ShouldNotBeNil)
		})

		t.Run("dest-missing", func(t *testing.T) {
			newer, err := newerOrMissing(filepath.Join(tmp, "two"), filepath.Join(tmp, "missing2"))
			test.That(t, err, test.ShouldBeNil)
			test.That(t, newer, test.ShouldBeTrue)
		})

		t.Run("source-newer", func(t *testing.T) {
			newer, err := newerOrMissing(filepath.Join(tmp, "four"), filepath.Join(tmp, "three"))
			test.That(t, err, test.ShouldBeNil)
			test.That(t, newer, test.ShouldBeTrue)
		})

		t.Run("dest-newer", func(t *testing.T) {
			newer, err := newerOrMissing(filepath.Join(tmp, "five"), filepath.Join(tmp, "six"))
			test.That(t, err, test.ShouldBeNil)
			test.That(t, newer, test.ShouldBeFalse)
		})
	})

	t.Run("RecopyIfChanged", func(t *testing.T) {
		mod := config.Module{Name: "tester", Type: config.ModuleTypeLocal, ExePath: "test_package.tar.gz"}
		missingMod := config.Module{Name: mod.Name, Type: config.ModuleTypeLocal, ExePath: "/no/such/path.tgz"}
		pkg, err := mod.SyntheticPackage()
		test.That(t, err, test.ShouldBeNil)
		destDir := pkg.LocalDataDirectory(local.packagesDir)

		// case: both missing
		err = mgr.SyncOne(context.Background(), missingMod)
		test.That(t, err, test.ShouldNotBeNil)

		// case: dest missing
		err = mgr.SyncOne(context.Background(), mod)
		test.That(t, err, test.ShouldBeNil)

		// case: source missing
		err = mgr.SyncOne(context.Background(), missingMod)
		test.That(t, os.IsNotExist(err), test.ShouldBeTrue)

		// case: dest newer
		prevModTime := modTime(t, destDir)
		err = mgr.SyncOne(context.Background(), mod)
		test.That(t, err, test.ShouldBeNil)
		newModTime := modTime(t, destDir)
		test.That(t, prevModTime, test.ShouldEqual, newModTime)

		// case: source newer
		prevModTime = newModTime
		newTar := filepath.Join(tmp, "newer.tar.gz")
		time.Sleep(time.Millisecond * 10)
		copyFile(t, "test_package.tar.gz", newTar)
		err = mgr.SyncOne(context.Background(), config.Module{Name: mod.Name, Type: config.ModuleTypeLocal, ExePath: newTar})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, prevModTime.Before(modTime(t, destDir)), test.ShouldBeTrue)
	})
}

func copyFile(t *testing.T, src, dest string) {
	t.Helper()
	fSrc, err := os.Open(src)
	test.That(t, err, test.ShouldBeNil)
	defer fSrc.Close()
	fDest, err := os.Create(dest)
	test.That(t, err, test.ShouldBeNil)
	defer fDest.Close()
	_, err = io.Copy(fDest, fSrc)
	test.That(t, err, test.ShouldBeNil)
}

// modTime is a t.Helper that stats a path and returns ModTime().
func modTime(t *testing.T, path string) time.Time {
	t.Helper()
	stat, err := os.Stat(path)
	test.That(t, err, test.ShouldBeNil)
	return stat.ModTime()
}

func TestLocalManagerSync(t *testing.T) {
	tmp := t.TempDir()
	mgr, err := NewLocalManager(filepath.Join(tmp, "pkg"), logging.NewTestLogger(t))
	test.That(t, err, test.ShouldBeNil)
	local := mgr.(*localManager)

	mod1 := config.Module{Name: "mod1", Type: config.ModuleTypeLocal, ExePath: testTarPath}
	mod2 := config.Module{Name: "mod2", Type: config.ModuleTypeLocal, ExePath: testTarPath}

	err = mgr.Sync(context.Background(), []config.PackageConfig{}, []config.Module{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, local.managedModules, test.ShouldHaveLength, 0)

	// first module
	err = mgr.Sync(context.Background(), []config.PackageConfig{}, []config.Module{mod1})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, local.managedModules, test.ShouldHaveLength, 1)
	test.That(t, moduleDirExists(t, local.packagesDir, mod1), test.ShouldBeTrue)

	// second module
	err = mgr.Sync(context.Background(), []config.PackageConfig{}, []config.Module{mod1, mod2})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, local.managedModules, test.ShouldHaveLength, 2)
	test.That(t, moduleDirExists(t, local.packagesDir, mod1), test.ShouldBeTrue)
	test.That(t, moduleDirExists(t, local.packagesDir, mod2), test.ShouldBeTrue)

	// change second module
	time.Sleep(time.Millisecond * 10)
	tar2 := filepath.Join(tmp, "tar2.tgz")
	copyFile(t, testTarPath, tar2)
	mod2.ExePath = tar2
	pkg, err := mod2.SyntheticPackage()
	test.That(t, err, test.ShouldBeNil)
	prevModTime := modTime(t, pkg.LocalDataDirectory(local.packagesDir))
	err = mgr.Sync(context.Background(), []config.PackageConfig{}, []config.Module{mod1, mod2})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, local.managedModules, test.ShouldHaveLength, 2)
	newModTime := modTime(t, pkg.LocalDataDirectory(local.packagesDir))
	// Careful! This is subtle.
	// Normal download flow *isn't* supposed to recopy if newer.
	// Because local modules don't have versions to increment, we only do this when the
	// user requests a restart; not during some random other reconfigure.
	test.That(t, prevModTime.Before(newModTime), test.ShouldBeFalse)

	// make sure Cleanup doesn't remove anything at this point
	mgr.Cleanup(context.Background())
	test.That(t, moduleDirExists(t, local.packagesDir, mod1), test.ShouldBeTrue)
	test.That(t, moduleDirExists(t, local.packagesDir, mod2), test.ShouldBeTrue)

	// remove second module, then test mgr.Cleanup
	err = mgr.Sync(context.Background(), []config.PackageConfig{}, []config.Module{mod1})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, local.managedModules, test.ShouldHaveLength, 1)
	test.That(t, moduleDirExists(t, local.packagesDir, mod1), test.ShouldBeTrue)
	test.That(t, moduleDirExists(t, local.packagesDir, mod2), test.ShouldBeTrue)

	err = mgr.Cleanup(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, moduleDirExists(t, local.packagesDir, mod1), test.ShouldBeTrue)
	test.That(t, moduleDirExists(t, local.packagesDir, mod2), test.ShouldBeFalse)
}

// assertModuleDirExists is a t.Helper that returns true if the module's unpack folder is present.
func moduleDirExists(t *testing.T, packagesDir string, mod config.Module) bool {
	t.Helper()
	pkg, err := mod.SyntheticPackage()
	test.That(t, err, test.ShouldBeNil)
	_, err = os.Stat(pkg.LocalDataDirectory(packagesDir))
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	t.Error("other error in moduleDirExists", err)
	return false // can't get here
}
