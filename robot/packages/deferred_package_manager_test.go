package packages

import (
	"context"
	"os"
	"testing"

	"github.com/pkg/errors"
	pb "go.viam.com/api/app/packages/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	putils "go.viam.com/rdk/robot/packages/testutils"
)

func TestDeferredPackageManager(t *testing.T) {
	type mockChanVal struct {
		client pb.PackageServiceClient
		err    error
	}

	// bag of random test infra
	type testBag struct {
		pm  *deferredPackageManager
		ctx context.Context
		// mockChan is used to populate the return value of establishConnection()
		mockChan chan mockChanVal
		// fake gcs stuff
		client      pb.PackageServiceClient
		fakeServer  *putils.FakePackagesClientAndGCSServer
		packagesDir string
		// cleanup fake gcs
		teardown func()
	}

	setup := func(t *testing.T) testBag {
		t.Helper()
		ctx := context.Background()
		logger := logging.NewTestLogger(t)

		fakeServer, err := putils.NewFakePackageServer(ctx, logger)
		test.That(t, err, test.ShouldBeNil)

		client, conn, err := fakeServer.Client(ctx)
		test.That(t, err, test.ShouldBeNil)

		teardown := func() {
			conn.Close()
			fakeServer.Shutdown()
		}
		cloudConfig := &config.Cloud{
			ID:     "some-id",
			Secret: "some-secret",
		}
		packagesDir := t.TempDir()
		mockChan := make(chan mockChanVal, 1)

		pm := NewDeferredPackageManager(
			ctx,
			func(c context.Context) (pb.PackageServiceClient, error) {
				v := <-mockChan
				return v.client, v.err
			},
			cloudConfig,
			packagesDir,
			logger,
		).(*deferredPackageManager)

		return testBag{
			pm,
			ctx,
			mockChan,
			client,
			fakeServer,
			packagesDir,
			teardown,
		}
	}

	pkgA := config.PackageConfig{
		Name:    "some-name",
		Package: "org1/test-model",
		Version: "v1",
		Type:    "ml_model",
	}

	pkgB := config.PackageConfig{
		Name:    "some-name-2",
		Package: "org1/test-model-2",
		Version: "v2",
		Type:    "module",
	}

	t.Run("getManagerForSync async", func(t *testing.T) {
		bag := setup(t)
		defer bag.teardown()

		// Assert that the cloud manager is nil initially
		mgr, err := bag.pm.getManagerForSync(bag.ctx, []config.PackageConfig{})
		test.That(t, err, test.ShouldBeNil)
		_, isNoop := mgr.(*noopManager)
		test.That(t, isNoop, test.ShouldBeTrue)
		// send a msg on the chan indicating a connection
		bag.mockChan <- mockChanVal{client: bag.client, err: nil}
		// this will wait until we start the new cloud manager
		mgr, err = bag.pm.getManagerForSync(bag.ctx, []config.PackageConfig{})
		test.That(t, err, test.ShouldBeNil)
		_, isCloud := mgr.(*cloudManager)
		test.That(t, isCloud, test.ShouldBeTrue)
		// test that we have cached that cloud_manager (in the async case)
		test.That(t, bag.pm.cloudManager, test.ShouldNotBeNil)
		_, err = bag.pm.getManagerForSync(bag.ctx, []config.PackageConfig{})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("getManagerForSync async will keep trying", func(t *testing.T) {
		bag := setup(t)
		defer bag.teardown()

		// we know that it is running establishConnection because it is pulling
		// from the 1-capacity channel
		// the err will still be nil because it is returning the noop manager and starting
		// the goroutine
		bag.mockChan <- mockChanVal{client: nil, err: errors.New("foo")}
		_, err := bag.pm.getManagerForSync(bag.ctx, []config.PackageConfig{})
		test.That(t, err, test.ShouldBeNil)
		bag.mockChan <- mockChanVal{client: nil, err: errors.New("foo")}
		_, err = bag.pm.getManagerForSync(bag.ctx, []config.PackageConfig{})
		test.That(t, err, test.ShouldBeNil)
		bag.mockChan <- mockChanVal{client: nil, err: errors.New("foo")}
		_, err = bag.pm.getManagerForSync(bag.ctx, []config.PackageConfig{})
		test.That(t, err, test.ShouldBeNil)
		bag.mockChan <- mockChanVal{client: nil, err: errors.New("foo")}
		_, err = bag.pm.getManagerForSync(bag.ctx, []config.PackageConfig{})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("getManagerForSync sync", func(t *testing.T) {
		bag := setup(t)
		defer bag.teardown()

		bag.mockChan <- mockChanVal{client: bag.client, err: nil}
		// Assert that missing pkgs cause sync loading
		mgr, err := bag.pm.getManagerForSync(bag.ctx, []config.PackageConfig{pkgA})
		test.That(t, err, test.ShouldBeNil)

		_, isCloud := mgr.(*cloudManager)
		test.That(t, isCloud, test.ShouldBeTrue)

		// test that we have cached that cloud_manager (in the sync case)
		test.That(t, bag.pm.cloudManager, test.ShouldNotBeNil)
		_, err = bag.pm.getManagerForSync(bag.ctx, []config.PackageConfig{pkgA})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("Sync + cleanup", func(t *testing.T) {
		bag := setup(t)
		defer bag.teardown()

		err := bag.pm.Sync(bag.ctx, []config.PackageConfig{}, []config.Module{})
		test.That(t, err, test.ShouldBeNil)
		_, isNoop := bag.pm.lastSyncedManager.(*noopManager)
		test.That(t, isNoop, test.ShouldBeTrue)

		// send a msg on the chan indicating a connection
		bag.mockChan <- mockChanVal{client: bag.client, err: nil}
		bag.fakeServer.StorePackage(pkgA)
		bag.fakeServer.StorePackage(pkgB)
		// this will wait until we start the new cloud manager
		err = bag.pm.Sync(bag.ctx, []config.PackageConfig{pkgA}, []config.Module{})
		test.That(t, err, test.ShouldBeNil)
		_, isCloud := bag.pm.lastSyncedManager.(*cloudManager)
		test.That(t, isCloud, test.ShouldBeTrue)
		// Assert that the package exists in the file system
		_, err = os.Stat(pkgA.LocalDataDirectory(bag.packagesDir))
		test.That(t, err, test.ShouldBeNil)

		// cleanup wont clean up the package yet so it should still exist
		err = bag.pm.Cleanup(bag.ctx)
		test.That(t, err, test.ShouldBeNil)
		_, err = os.Stat(pkgA.LocalDataDirectory(bag.packagesDir))
		test.That(t, err, test.ShouldBeNil)

		// sync over to pkgB and cleanup
		err = bag.pm.Sync(bag.ctx, []config.PackageConfig{pkgB}, []config.Module{})
		test.That(t, err, test.ShouldBeNil)
		err = bag.pm.Cleanup(bag.ctx)
		test.That(t, err, test.ShouldBeNil)

		// pkgA should be cleaned up and pkgB should exist
		_, err = os.Stat(pkgA.LocalDataDirectory(bag.packagesDir))
		test.That(t, err, test.ShouldNotBeNil)
		_, err = os.Stat(pkgB.LocalDataDirectory(bag.packagesDir))
		test.That(t, err, test.ShouldBeNil)
	})
}
