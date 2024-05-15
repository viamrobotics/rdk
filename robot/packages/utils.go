package packages

import (
	"context"
	"fmt"
	"os"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
)

// downloadCallback is the function signature that gets passed to downloadPackage.
type downloadCallback func(ctx context.Context, url, dstPath string) (contentType string, err error)

func downloadPackage(ctx context.Context, logger logging.Logger, packagesDir, url string, p config.PackageConfig,
	nonEmptyPaths []string, downloadFn downloadCallback,
) error {
	if dataDir := p.LocalDataDirectory(packagesDir); dirExists(dataDir) {
		if checkNonemptyPaths(p.Name, logger, nonEmptyPaths) {
			logger.Debugf("Package already downloaded at %s, skipping.", dataDir)
			return nil
		}
	}

	// Create the parent directory for the package type if it doesn't exist
	if err := os.MkdirAll(p.LocalDataParentDirectory(packagesDir), 0o700); err != nil {
		return err
	}

	// Force redownload of package archive.
	if err := cleanup(packagesDir, p); err != nil {
		logger.Debug(err)
	}

	if p.Type == config.PackageTypeMlModel {
		symlinkPath, err := safeJoin(packagesDir, p.Name)
		if err == nil {
			if err := os.Remove(symlinkPath); err != nil {
				utils.UncheckedError(err)
			}
		}
	}

	dstPath := p.LocalDownloadPath(packagesDir)
	contentType, err := downloadFn(ctx, url, dstPath)
	if err != nil {
		return err
	}

	if contentType != allowedContentType {
		utils.UncheckedError(cleanup(packagesDir, p))
		return fmt.Errorf("unknown content-type for package %s", contentType)
	}

	// unpack to temp directory to ensure we do an atomic rename once finished.
	tmpDataPath, err := os.MkdirTemp(p.LocalDataParentDirectory(packagesDir), "*.tmp")
	if err != nil {
		return errors.Wrap(err, "failed to create temp data dir path")
	}

	defer func() {
		// cleanup archive file.
		if err := os.Remove(dstPath); err != nil {
			logger.Debug(err)
		}
		if err := os.RemoveAll(tmpDataPath); err != nil {
			logger.Debug(err)
		}
	}()

	// unzip archive.
	err = unpackFile(ctx, dstPath, tmpDataPath)
	if err != nil {
		utils.UncheckedError(cleanup(packagesDir, p))
		return err
	}

	err = os.Rename(tmpDataPath, p.LocalDataDirectory(packagesDir))
	if err != nil {
		utils.UncheckedError(cleanup(packagesDir, p))
		return err
	}

	return nil
}

func cleanup(packagesDir string, p config.PackageConfig) error {
	return multierr.Combine(
		os.RemoveAll(p.LocalDataDirectory(packagesDir)),
		os.Remove(p.LocalDownloadPath(packagesDir)),
	)
}
