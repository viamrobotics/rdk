package packages

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/pkg/errors"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
)

type syncStatus string

const (
	syncStatusDownloading syncStatus = "downloading"
	syncStatusUnpacking = "unpacking"
	syncStatusDone = "done"
)

type packageSyncFile struct {
	packageId 		string
	modifiedTime    time.Time
	status      	syncStatus			
	tarChecksum		string
}

func packageIsSynced(pkg config.PackageConfig, packagesDir string) bool {
	syncFile, err := loadStatusFile(pkg, packagesDir)
	if err != nil {
		logger.Errorf("Failed to determine status of package sync for %s:%s", err)
	}

	if syncFile.status == syncStatusDownloading {
		return true
	}
	return false
}

func packagesAreSynced(packages []config.PackageConfig, packagesDir string) bool {
	for _, pkg := range packages {
		if packageIsSynced(pkg, packagesDir) {
			return false
		}
	}
	return true
}

func getSyncFileName(packageLocalDataDirectory string) string {
	return fmt.Sprintf("%s.status.json", packageLocalDataDirectory)
}

func loadStatusFile(pkg config.PackageConfig, packagesDir string) (packageSyncFile, error) {
	//nolint:gosec
	syncFileName := getSyncFileName(pkg.LocalDataDirectory(packagesDir))
	syncFileBytes, err := os.ReadFile(syncFileName)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return packageSyncFile{}, errors.Wrapf(err, "cannot find %s", syncFileName)
		}
		return packageSyncFile{}, err
	}
	var syncFile packageSyncFile
	if err := json.Unmarshal(syncFileBytes, &syncFile); err != nil {
		return packageSyncFile{}, err
	}
	return syncFile, nil
}

func writeStatusFile(pkg config.PackageConfig, statusFile packageSyncFile, packagesDir string) error {
	return nil
}
