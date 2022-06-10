package datamanager_test

import (
	"context"
	"io/ioutil"
	"os"
	"testing"

	"time"
	//"github.com/pkg/errors"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/services/datamanager"
	"go.viam.com/rdk/services/datamanager/internal"
	"go.viam.com/rdk/testutils/inject"
	rdkutils "go.viam.com/rdk/utils"
	"go.viam.com/test"
)

func newTestDataManager(t *testing.T, captureDir string) internal.Service {
	cfg := &datamanager.Config{
		CaptureDir: captureDir,
	}
	cfgService := config.Service{
		Type:                "data_manager",
		ConvertedAttributes: cfg,
	}

	logger := golog.NewTestLogger(t)
	r := &inject.Robot{}

	svc, _ := datamanager.New(context.Background(), r, cfgService, logger)
	return svc.(internal.Service)
}

func getConfig(t *testing.T, relativePath string) *config.Config {
	logger := golog.NewTestLogger(t)
	fakeCfg, err := config.Read(context.Background(), rdkutils.ResolveFile(relativePath), logger)
	test.That(t, err, test.ShouldBeNil)
	return fakeCfg
}

// Validates that manual syncing works for a datamanager.
func TestManualSync(t *testing.T) {
	captureDir := t.TempDir()
	queueDir := t.TempDir()
	configPath := "robot/impl/data/fake.json"
	dmsvc := newTestDataManager(t, captureDir).(internal.Service)
	fakeCfg := getConfig(t, configPath)
	dmsvc.Update(nil, fakeCfg)

	// Put a couple files in captureDir.
	file1, _ := ioutil.TempFile(captureDir, "whatever")
	defer os.Remove(file1.Name())
	file2, _ := ioutil.TempFile(captureDir, "whatever2")
	defer os.Remove(file2.Name())

	// Give it a second to run and upload files.
	dmsvc.Sync(context.Background())
	time.Sleep(time.Second)

	//Verify files were enqueued and uploaded.
	filesInCaptureDir, err := ioutil.ReadDir(captureDir)
	if err != nil {
		t.Fatalf("failed to list files in captureDir")
	}
	filesInQueue, err := ioutil.ReadDir(queueDir)
	if err != nil {
		t.Fatalf("failed to list files in queueDir")
	}
	test.That(t, len(filesInCaptureDir), test.ShouldEqual, 0)
	test.That(t, len(filesInQueue), test.ShouldEqual, 0)

}

// Validates that scheduled syncing works for a datamanager.
func TestScheduledSync(t *testing.T) {
	captureDir := t.TempDir()
	queueDir := t.TempDir()
	configPath := "robot/impl/data/fake.json"
	dmsvc := newTestDataManager(t, captureDir).(internal.Service)
	fakeCfg := getConfig(t, configPath)
	dmsvc.Update(nil, fakeCfg)

	// Put a couple files in captureDir.
	file1, _ := ioutil.TempFile(captureDir, "whatever")
	defer os.Remove(file1.Name())
	file2, _ := ioutil.TempFile(captureDir, "whatever2")
	defer os.Remove(file2.Name())

	// Give it a second to run and upload files.
	dmsvc.Sync(context.Background())
	time.Sleep(time.Second)

	//Verify files were enqueued and uploaded.
	filesInCaptureDir, err := ioutil.ReadDir(captureDir)
	if err != nil {
		t.Fatalf("failed to list files in captureDir")
	}
	filesInQueue, err := ioutil.ReadDir(queueDir)
	if err != nil {
		t.Fatalf("failed to list files in queueDir")
	}
	test.That(t, len(filesInCaptureDir), test.ShouldEqual, 0)
	test.That(t, len(filesInQueue), test.ShouldEqual, 0)

}

// Validates that we can attempt a scheduled and manual sync at the same time without .
func TestManualAndScheduledSync(t *testing.T) {
	captureDir := t.TempDir()
	queueDir := t.TempDir()
	configPath := "robot/impl/data/fake.json"
	dmsvc := newTestDataManager(t, captureDir).(internal.Service)
	fakeCfg := getConfig(t, configPath)
	dmsvc.Update(nil, fakeCfg)

	// Put a couple files in captureDir.
	file1, _ := ioutil.TempFile(captureDir, "whatever")
	defer os.Remove(file1.Name())
	file2, _ := ioutil.TempFile(captureDir, "whatever2")
	defer os.Remove(file2.Name())

	// Give it a second to run and upload files.
	dmsvc.Sync(context.Background())
	time.Sleep(time.Second)

	//Verify files were enqueued and uploaded.
	filesInCaptureDir, err := ioutil.ReadDir(captureDir)
	if err != nil {
		t.Fatalf("failed to list files in captureDir")
	}
	filesInQueue, err := ioutil.ReadDir(queueDir)
	if err != nil {
		t.Fatalf("failed to list files in queueDir")
	}
	test.That(t, len(filesInCaptureDir), test.ShouldEqual, 0)
	test.That(t, len(filesInQueue), test.ShouldEqual, 0)

}

// Validates that trying to queue the same files twice leads to
func TestQueuingLock(t *testing.T) {
	captureDir := t.TempDir()
	queueDir := t.TempDir()
	configPath := "robot/impl/data/fake.json"
	dmsvc := newTestDataManager(t, captureDir).(internal.Service)
	fakeCfg := getConfig(t, configPath)
	dmsvc.Update(nil, fakeCfg)

	// Put a couple files in captureDir.
	file1, _ := ioutil.TempFile(captureDir, "whatever")
	defer os.Remove(file1.Name())
	file2, _ := ioutil.TempFile(captureDir, "whatever2")
	defer os.Remove(file2.Name())

	// Give it a second to run and upload files.
	dmsvc.Sync(context.Background())
	time.Sleep(time.Second)

	//Verify files were enqueued and uploaded.
	filesInCaptureDir, err := ioutil.ReadDir(captureDir)
	if err != nil {
		t.Fatalf("failed to list files in captureDir")
	}
	filesInQueue, err := ioutil.ReadDir(queueDir)
	if err != nil {
		t.Fatalf("failed to list files in queueDir")
	}
	test.That(t, len(filesInCaptureDir), test.ShouldEqual, 0)
	test.That(t, len(filesInQueue), test.ShouldEqual, 0)

}
