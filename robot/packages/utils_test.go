package packages

import (
	"context"
	"errors"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
)

func TestInstallPackageDiskGuard(t *testing.T) {
	logger := logging.NewTestLogger(t)
	pkg := config.PackageConfig{
		Name:    "some-name",
		Package: "org1/test-model",
		Version: "v1",
		Type:    "ml_model",
	}

	t.Run("proceeds when there is free space", func(t *testing.T) {
		// t.TempDir lives on a real volume that has far more than MinFreeBytes free,
		// so the guard should let the download proceed to installFn.
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
