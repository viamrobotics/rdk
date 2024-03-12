package packages

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	pb "go.viam.com/api/app/packages/v1"
	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	putils "go.viam.com/rdk/robot/packages/testutils"
)

func TestDeferredPackageManager(t *testing.T) {
	makeFakeClient := func(t *testing.T) (pb.PackageServiceClient, *putils.FakePackagesClientAndGCSServer, func()) {
		t.Helper()
		logger := logging.NewTestLogger(t)
		ctx := context.Background()
		fakeServer, err := putils.NewFakePackageServer(ctx, logger)
		test.That(t, err, test.ShouldBeNil)

		client, conn, err := fakeServer.Client(ctx)
		test.That(t, err, test.ShouldBeNil)
		return client, fakeServer, func() {
			fakeServer.Shutdown()
			conn.Close()
		}
	}
	makePM := func(t *testing.T) (*deferredPackageManager, chan DeferredConnectionResponse, string) {
		t.Helper()
		connectionChan := make(chan DeferredConnectionResponse)
		cloudConfig := &config.Cloud{
			ID:     "some-id",
			Secret: "some-secret",
		}
		packagesDir := t.TempDir()
		logger := logging.NewTestLogger(t)

		pm := NewDeferredPackageManager(connectionChan, cloudConfig, packagesDir, logger).(*deferredPackageManager)
		return pm, connectionChan, packagesDir
	}

	t.Run("getCloudManager", func(t *testing.T) {
		client, _, closeFake := makeFakeClient(t)
		defer closeFake()
		pm, connectionChan, _ := makePM(t)

		// Assert that the cloud manager is nil initially
		test.That(t, pm.getCloudManager(false), test.ShouldBeNil)

		// Send a client to the connection channel
		utils.PanicCapturingGo(func() {
			connectionChan <- DeferredConnectionResponse{
				Client: client,
				Err:    nil,
			}
		})

		// Assert that the cloud manager is not nil after receiving a client
		test.That(t, pm.getCloudManager(true), test.ShouldNotBeNil)
	})

	t.Run("isMissingPackages", func(t *testing.T) {
		pm, _, packagesDir := makePM(t)

		// Create a package config
		packages := []config.PackageConfig{
			{
				Name:    "some-name",
				Package: "org1/test-model",
				Version: "v1",
				Type:    "ml_model",
			},
		}

		// Assert that the package is missing initially
		test.That(t, pm.isMissingPackages(packages), test.ShouldBeTrue)

		// Create a directory for the package
		err := os.MkdirAll(packages[0].LocalDataDirectory(packagesDir), os.ModePerm)
		test.That(t, err, test.ShouldBeNil)

		// Assert that the package is not missing after creating the directory
		test.That(t, pm.isMissingPackages(packages), test.ShouldBeFalse)
	})

	t.Run("sync", func(t *testing.T) {
		ctx := context.Background()

		client, fakeServer, closeFake := makeFakeClient(t)
		defer closeFake()

		pm, connectionChan, packagesDir := makePM(t)

		// Send a client to the connection channel
		utils.PanicCapturingGo(func() {
			connectionChan <- DeferredConnectionResponse{
				Client: client,
				Err:    nil,
			}
		})

		// Create a package config
		pkg := config.PackageConfig{
			Name:    "some-name",
			Package: "org1/test-model",
			Version: "v1",
			Type:    "ml_model",
		}
		fakeServer.StorePackage(pkg)

		// Sync the package
		err := pm.Sync(ctx, []config.PackageConfig{pkg})
		test.That(t, err, test.ShouldBeNil)

		// Assert that the package exists in the file system
		_, err = os.Stat(pkg.LocalDataDirectory(packagesDir))
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("cleanup", func(t *testing.T) {
		ctx := context.Background()
		client, _, closeFake := makeFakeClient(t)
		defer closeFake()

		pm, connectionChan, packagesDir := makePM(t)

		// Send a client to the connection channel
		utils.PanicCapturingGo(func() {
			connectionChan <- DeferredConnectionResponse{
				Client: client,
				Err:    nil,
			}
		})
		test.That(t, pm.getCloudManager(true), test.ShouldNotBeNil)

		badFile := filepath.Join(packagesDir, "data", "badfile.txt")
		err := os.WriteFile(badFile, []byte("hello"), 0o700)
		test.That(t, err, test.ShouldBeNil)
		// Assert that the package exists in the file system
		_, err = os.Stat(badFile)
		test.That(t, err, test.ShouldBeNil)
		err = pm.Cleanup(ctx)
		test.That(t, err, test.ShouldBeNil)
		// Assert that the package was cleaned from the file system
		_, err = os.Stat(badFile)
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("sync with failure", func(t *testing.T) {
		ctx := context.Background()
		pm, connectionChan, _ := makePM(t)

		// Simulate a connection failure
		utils.PanicCapturingGo(func() {
			connectionChan <- DeferredConnectionResponse{Err: errors.New("connection failure")}
		})

		// Test syncing with a package
		err := pm.Sync(ctx, []config.PackageConfig{{Name: "some-package", Package: "org1/test-model", Version: "v1", Type: "ml_model"}})
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("packagePath", func(t *testing.T) {
		ctx := context.Background()

		client, fakeServer, closeFake := makeFakeClient(t)
		defer closeFake()
		pm, connectionChan, packagesDir := makePM(t)

		// Send a client to the connection channel
		utils.PanicCapturingGo(func() {
			connectionChan <- DeferredConnectionResponse{
				Client: client,
				Err:    nil,
			}
		})

		// Create a package config
		pkg := config.PackageConfig{
			Name:    "some-name",
			Package: "org1/test-model",
			Version: "v1",
			Type:    "ml_model",
		}
		fakeServer.StorePackage(pkg)

		// Sync the package
		err := pm.Sync(ctx, []config.PackageConfig{pkg})
		test.That(t, err, test.ShouldBeNil)

		// Get the package path
		path, err := pm.PackagePath(PackageName(pkg.Name))
		test.That(t, err, test.ShouldBeNil)

		// Assert that the package path is correct
		test.That(t, path, test.ShouldEqual, pkg.LocalDataDirectory(packagesDir))
	})
}
