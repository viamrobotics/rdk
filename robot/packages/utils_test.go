package packages

import (
	"context"
	"errors"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	rutils "go.viam.com/rdk/utils"
)

func TestInstallPackageDiskGuard(t *testing.T) {
	logger := logging.NewTestLogger(t)
	pkg := config.PackageConfig{
		Name:    "some-name",
		Package: "org1/test-model",
		Version: "v1",
		Type:    "ml_model",
	}

	// withFreeSpaceCheck swaps in a fake disk-space check for the duration of a
	// subtest and restores the real one afterwards.
	withFreeSpaceCheck := func(t *testing.T, fn func(path string, minBytes uint64) (bool, uint64, error)) {
		t.Helper()
		orig := enoughFreeSpace
		enoughFreeSpace = fn
		t.Cleanup(func() { enoughFreeSpace = orig })
	}

	t.Run("proceeds when there is free space", func(t *testing.T) {
		// t.TempDir lives on a real volume that has far more than MinFreeBytes free,
		// so the real check should let the download proceed to installFn.
		called := false
		sentinel := errors.New("install called")
		err := installPackage(context.Background(), logger, t.TempDir(), "http://example.com/pkg.tar.gz", pkg, false,
			func(ctx context.Context, url, dstPath string) (string, string, error) {
				called = true
				return "", "", sentinel
			})
		test.That(t, called, test.ShouldBeTrue)
		test.That(t, errors.Is(err, sentinel), test.ShouldBeTrue)
	})

	t.Run("refuses download when space is low and blocking is enabled", func(t *testing.T) {
		t.Setenv(rutils.ViamEnableDiskSpaceBlockEnvVar, "true")
		withFreeSpaceCheck(t, func(string, uint64) (bool, uint64, error) {
			return false, 5, nil
		})
		called := false
		err := installPackage(context.Background(), logger, t.TempDir(), "http://example.com/pkg.tar.gz", pkg, false,
			func(ctx context.Context, url, dstPath string) (string, string, error) {
				called = true
				return "", "", nil
			})
		test.That(t, called, test.ShouldBeFalse)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "not enough free disk space")
		test.That(t, err.Error(), test.ShouldContainSubstring, pkg.Name)
	})

	t.Run("proceeds when space is low but blocking is disabled (log-only default)", func(t *testing.T) {
		withFreeSpaceCheck(t, func(string, uint64) (bool, uint64, error) {
			return false, 5, nil
		})
		called := false
		sentinel := errors.New("install called")
		err := installPackage(context.Background(), logger, t.TempDir(), "http://example.com/pkg.tar.gz", pkg, false,
			func(ctx context.Context, url, dstPath string) (string, string, error) {
				called = true
				return "", "", sentinel
			})
		test.That(t, called, test.ShouldBeTrue)
		test.That(t, errors.Is(err, sentinel), test.ShouldBeTrue)
	})

	t.Run("proceeds when the check itself errors", func(t *testing.T) {
		withFreeSpaceCheck(t, func(string, uint64) (bool, uint64, error) {
			return false, 0, errors.New("statfs boom")
		})
		called := false
		sentinel := errors.New("install called")
		err := installPackage(context.Background(), logger, t.TempDir(), "http://example.com/pkg.tar.gz", pkg, false,
			func(ctx context.Context, url, dstPath string) (string, string, error) {
				called = true
				return "", "", sentinel
			})
		test.That(t, called, test.ShouldBeTrue)
		test.That(t, errors.Is(err, sentinel), test.ShouldBeTrue)
	})
}
