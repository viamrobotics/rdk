package packages

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	putils "go.viam.com/rdk/robot/packages/testutils"
	"go.viam.com/test"
)

func TestSyntheticPackageDownload(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	fakeServer, err := putils.NewFakePackageServer(ctx, logger)
	test.That(t, err, test.ShouldBeNil)
	t.Cleanup(func() { fakeServer.Shutdown() })

	client, conn, err := fakeServer.Client(ctx)
	test.That(t, err, test.ShouldBeNil)
	t.Cleanup(func() { conn.Close() })

	remotePackage := config.PackageConfig{
		Name:    "test:remote",
		Package: "test:remote",
		Version: "0.0.1",
		Type:    config.PackageTypeModule,
	}
	syntheticPackage := config.PackageConfig{
		Name:    "test:synthetic",
		Package: "test:synthetic",
		// LocalPath: "test_package.tar.gz",
	}

	fakeServer.StorePackage(remotePackage)

	t.Run("getPackageURL", func(t *testing.T) {
		url, err := getPackageURL(ctx, logger, client, remotePackage)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, url, test.ShouldStartWith, "http://127.0.0.1:")
		test.That(t, url, test.ShouldEndWith, fmt.Sprintf("/download-file?id=%s&version=%s", remotePackage.Package, remotePackage.Version))

		url, err = getPackageURL(ctx, logger, client, syntheticPackage)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, url, test.ShouldEqual, "file://test_package.tar.gz")
	})

	t.Run("downloadPackage-synthetic", func(t *testing.T) {
		dstDir := t.TempDir()
		cm := cloudManager{
			logger:      logger,
			packagesDir: dstDir,
		}
		url, err := getPackageURL(ctx, logger, client, syntheticPackage)
		test.That(t, err, test.ShouldBeNil)
		err = downloadPackage(ctx, url, syntheticPackage, []string{})
		test.That(t, err, test.ShouldBeNil)
		dst := filepath.Join(
			syntheticPackage.LocalDataDirectory(cm.packagesDir),
			"run.sh",
		)
		_, err = os.Stat(dst)
		test.That(t, err, test.ShouldBeNil)
	})
}
