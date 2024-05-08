package packages

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
	pb "go.viam.com/api/app/packages/v1"
	"go.viam.com/utils"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

const (
	allowedContentType = "application/x-gzip"
)

var (
	_ Manager       = (*cloudManager)(nil)
	_ ManagerSyncer = (*cloudManager)(nil)
)

type managedPackage struct {
	thePackage config.PackageConfig
	modtime    time.Time
}

type cloudManager struct {
	resource.Named
	// we assume the config is immutable for the lifetime of the process
	resource.TriviallyReconfigurable
	client          pb.PackageServiceClient
	httpClient      http.Client
	packagesDataDir string
	packagesDir     string
	cloudConfig     config.Cloud

	managedPackages map[PackageName]*managedPackage
	mu              sync.RWMutex

	logger logging.Logger
}

// SubtypeName is a constant that identifies the internal package manager resource subtype string.
const SubtypeName = "packagemanager"

// API is the fully qualified API for the internal package manager service.
var API = resource.APINamespaceRDKInternal.WithServiceType(SubtypeName)

// InternalServiceName is used to refer to/depend on this service internally.
var InternalServiceName = resource.NewName(API, "builtin")

// NewCloudManager creates a new manager with the given package service client and directory to sync to.
func NewCloudManager(
	cloudConfig *config.Cloud,
	client pb.PackageServiceClient,
	packagesDir string,
	logger logging.Logger,
) (ManagerSyncer, error) {
	packagesDataDir := filepath.Join(packagesDir, "data")

	if err := os.MkdirAll(packagesDir, 0o700); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(packagesDataDir, 0o700); err != nil {
		return nil, err
	}

	return &cloudManager{
		Named:           InternalServiceName.AsNamed(),
		client:          client,
		httpClient:      http.Client{Timeout: time.Minute * 30},
		cloudConfig:     *cloudConfig,
		packagesDir:     packagesDir,
		packagesDataDir: packagesDataDir,
		managedPackages: make(map[PackageName]*managedPackage),
		logger:          logger.Sublogger("package_manager"),
	}, nil
}

// PackagePath returns the package if it exists and is already downloaded. If it does not exist it returns a ErrPackageMissing error.
func (m *cloudManager) PackagePath(name PackageName) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, ok := m.managedPackages[name]
	if !ok {
		return "", ErrPackageMissing
	}

	return p.thePackage.LocalDataDirectory(m.packagesDir), nil
}

// Close manager.
func (m *cloudManager) Close(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.httpClient.CloseIdleConnections()
	return nil
}

// getPackageURL fetches package download URL from API, or creates a `file://` URL from pkg.LocalPath.
func getPackageURL(ctx context.Context, logger logging.Logger, client pb.PackageServiceClient, pkg *config.PackageConfig) (string, error) {
	if len(pkg.LocalPath) > 0 {
		return fmt.Sprintf("file://%s", pkg.LocalPath), nil
	} else {
		packageType, err := config.PackageTypeToProto(pkg.Type)
		if err != nil {
			logger.Warnw("failed to get package type", "package", pkg.Name, "error", err)
		}
		// Lookup the package's http url
		includeURL := true
		resp, err := client.GetPackage(ctx, &pb.GetPackageRequest{
			Id:         pkg.Package,
			Version:    pkg.Version,
			Type:       packageType,
			IncludeUrl: &includeURL,
		})
		if err != nil {
			logger.Errorf("Failed fetching package details for package %s:%s, %s", pkg.Package, pkg.Version, err)
			return "", errors.Wrapf(err, "failed loading package url for %s:%s", pkg.Package, pkg.Version)
		}
		return resp.Package.Url, nil
	}
}

// SyncAll syncs all given packages and removes any not in the list from the local file system.
func (m *cloudManager) Sync(ctx context.Context, packages []config.PackageConfig, modules []config.Module) error {
	var outErr error

	// Only allow one rdk process to operate on the manager at once. This is generally safe to keep locked for an extended period of time
	// since the config reconfiguration process is handled by a single thread.
	m.mu.Lock()
	defer m.mu.Unlock()

	// this map will replace m.managedPackages at the end of the function
	newManagedPackages := make(map[PackageName]*managedPackage, len(packages))

	for _, p := range packages {
		// Package exists in known cache.
		if m.packageIsManaged(p) {
			newManagedPackages[PackageName(p.Name)] = m.managedPackages[PackageName(p.Name)]
			continue
		}
	}

	// Process the packages that are new or changed
	changedPackages := m.validateAndGetChangedPackages(packages)
	if len(changedPackages) > 0 {
		m.logger.Info("Package changes have been detected, starting sync")
	}

	start := time.Now()
	for idx, p := range changedPackages {
		pkgStart := time.Now()
		if err := ctx.Err(); err != nil {
			return multierr.Append(outErr, err)
		}

		m.logger.Debugf("Starting package sync [%d/%d] %s:%s", idx+1, len(changedPackages), p.Package, p.Version)

		packageURL, err := getPackageURL(ctx, m.logger, m.client, &p)
		if err != nil {
			outErr = multierr.Append(outErr, err)
			continue
		}

		m.logger.Debugf("Downloading from %s", sanitizeURLForLogs(packageURL))

		nonEmptyPaths := make([]string, 0)
		if p.Type == config.PackageTypeModule {
			matchedModules := m.modulesForPackage(p, modules)
			if len(matchedModules) == 1 {
				nonEmptyPaths = append(nonEmptyPaths, matchedModules[0].ExePath)
			}
			if len(matchedModules) > 1 {
				m.logger.CWarnf(ctx, "package %s matched %d > 1 modules, not doing entrypoint checking", p.Name, len(matchedModules))
			}
		}

		// download package from a http endpoint
		err = m.downloadPackage(ctx, packageURL, p, nonEmptyPaths)
		if err != nil {
			m.logger.Errorf("Failed downloading package %s:%s from %s, %s", p.Package, p.Version, sanitizeURLForLogs(packageURL), err)
			outErr = multierr.Append(outErr, errors.Wrapf(err, "failed downloading package %s:%s from %s",
				p.Package, p.Version, sanitizeURLForLogs(packageURL)))
			continue
		}

		if p.Type == config.PackageTypeMlModel {
			outErr = multierr.Append(outErr, m.mLModelSymlinkCreation(p))
		}

		// add to managed packages
		newManagedPackages[PackageName(p.Name)] = &managedPackage{thePackage: p, modtime: time.Now()}

		m.logger.Debugf("Package sync complete [%d/%d] %s:%s after %v", idx+1, len(changedPackages), p.Package, p.Version, time.Since(pkgStart))
	}

	if len(changedPackages) > 0 {
		m.logger.Infof("Package sync complete after %v", time.Since(start))
	}

	// swap for new managed packags.
	m.managedPackages = newManagedPackages

	return outErr
}

// modulesForPackage returns module(s) whose ExePath is in the package's directory.
// TODO: This only works if you call it after placeholder replacement. Find a cleaner way to express this link.
func (m *cloudManager) modulesForPackage(pkg config.PackageConfig, modules []config.Module) []config.Module {
	pkgDir := pkg.LocalDataDirectory(m.packagesDir)
	ret := make([]config.Module, 0, 1)
	for _, module := range modules {
		if strings.HasPrefix(module.ExePath, pkgDir) {
			ret = append(ret, module)
		}
	}
	return ret
}

func (m *cloudManager) validateAndGetChangedPackages(packages []config.PackageConfig) []config.PackageConfig {
	changed := make([]config.PackageConfig, 0)
	for _, p := range packages {
		// don't consider invalid config as synced or unsynced
		if err := p.Validate(""); err != nil {
			m.logger.Errorw("package config validation error; skipping", "package", p.Name, "error", err)
			continue
		}
		if existing := m.packageIsManaged(p); !existing {
			newPackage := p
			changed = append(changed, newPackage)
		} else {
			m.logger.Debugf("Package %s:%s already managed, skipping", p.Package, p.Version)
		}
	}
	return changed
}

func (m *cloudManager) packageIsManaged(p config.PackageConfig) bool {
	existing, ok := m.managedPackages[PackageName(p.Name)]
	if ok {
		if existing.thePackage.Package == p.Package && existing.thePackage.Version == p.Version {
			return true
		}
	}
	return false
}

// Cleanup removes all unknown packages from the working directory.
func (m *cloudManager) Cleanup(ctx context.Context) error {
	m.logger.Debug("Starting package cleanup")

	// Only allow one rdk process to operate on the manager at once. This is generally safe to keep locked for an extended period of time
	// since the config reconfiguration process is handled by a single thread.
	m.mu.Lock()
	defer m.mu.Unlock()

	var allErrors error

	expectedPackageDirectories := map[string]bool{}
	for _, pkg := range m.managedPackages {
		expectedPackageDirectories[pkg.thePackage.LocalDataDirectory(m.packagesDir)] = true
	}

	topLevelFiles, err := os.ReadDir(m.packagesDataDir)
	if err != nil {
		return err
	}
	// A packageTypeDir is a directory that contains all of the packages for the specified type. ex: data/ml_model
	for _, packageTypeDir := range topLevelFiles {
		packageTypeDirName, err := safeJoin(m.packagesDataDir, packageTypeDir.Name())
		if err != nil {
			allErrors = multierr.Append(allErrors, err)
			continue
		}

		// There should be no non-dir files in the packages/data dir. Delete any that exist
		if packageTypeDir.Type()&os.ModeDir != os.ModeDir {
			allErrors = multierr.Append(allErrors, os.Remove(packageTypeDirName))
			continue
		}
		// read all of the packages in the directory and delete those that aren't in expectedPackageDirectories
		packageDirs, err := os.ReadDir(packageTypeDirName)
		if err != nil {
			allErrors = multierr.Append(allErrors, err)
			continue
		}
		for _, packageDir := range packageDirs {
			packageDirName, err := safeJoin(packageTypeDirName, packageDir.Name())
			if err != nil {
				allErrors = multierr.Append(allErrors, err)
				continue
			}
			_, expectedToExist := expectedPackageDirectories[packageDirName]
			if !expectedToExist {
				m.logger.Debugf("Removing old package %s", packageDirName)
				allErrors = multierr.Append(allErrors, os.RemoveAll(packageDirName))
			}
		}
		// re-read the directory, if there is nothing left in it, delete the directory
		packageDirs, err = os.ReadDir(packageTypeDirName)
		if err != nil {
			allErrors = multierr.Append(allErrors, err)
			continue
		}
		if len(packageDirs) == 0 {
			allErrors = multierr.Append(allErrors, os.RemoveAll(packageTypeDirName))
		}
	}

	allErrors = multierr.Append(allErrors, m.mlModelSymlinkCleanup())
	return allErrors
}

// symlink packages/package-name to packages/data/ml_model/orgid-package-name-ver for backwards compatibility
// TODO(RSDK-4386) Preserved for backwards compatibility. Could be removed or extended to other types in the future.
func (m *cloudManager) mLModelSymlinkCreation(p config.PackageConfig) error {
	symlinkPath, err := safeJoin(m.packagesDir, p.Name)
	if err != nil {
		return err
	}

	if err := linkFile(p.LocalDataDirectory(m.packagesDir), symlinkPath); err != nil {
		return errors.Wrapf(err, "failed linking ml_model package %s:%s, %s", p.Package, p.Version, err)
	}
	return nil
}

// cleanup all symlinks in packages/ directory
// TODO(RSDK-4386) Preserved for backwards compatibility. Could be removed or extended to other types in the future.
func (m *cloudManager) mlModelSymlinkCleanup() error {
	var allErrors error
	files, err := os.ReadDir(m.packagesDir)
	if err != nil {
		return err
	}

	// The only symlinks in this directory are those created for MLModels
	for _, f := range files {
		if f.Type()&os.ModeSymlink != os.ModeSymlink {
			continue
		}
		// if managed skip removing package
		if _, ok := m.managedPackages[PackageName(f.Name())]; ok {
			continue
		}

		m.logger.Infof("Cleaning up unused package link %s", f.Name())

		symlinkPath, err := safeJoin(m.packagesDir, f.Name())
		if err != nil {
			allErrors = multierr.Append(allErrors, err)
			continue
		}
		// Remove logical symlink to package
		if err := os.Remove(symlinkPath); err != nil {
			allErrors = multierr.Append(allErrors, err)
		}
	}
	return allErrors
}

func sanitizeURLForLogs(u string) string {
	parsed, err := url.Parse(u)
	if err != nil {
		return ""
	}
	parsed.RawQuery = ""
	return parsed.String()
}

// checkNonemptyPaths returns true if all required paths are present and non-empty.
// This exists because we have no way to check the integrity of modules *after* they've been unpacked.
// (We do look at checksums of downloaded tarballs, though). Once we have a better integrity check
// system for unpacked modules, this should be removed.
func checkNonemptyPaths(packageName string, logger logging.Logger, absPaths []string) bool {
	missingOrEmpty := 0
	for _, nePath := range absPaths {
		if !path.IsAbs(nePath) {
			// note: we expect paths to be absolute in most cases.
			// We're okay not having corrupted files coverage for relative paths, and prefer it to
			// risking a re-download loop from an edge case.
			logger.Debugw("ignoring non-abs required path", "path", nePath, "package", packageName)
			continue
		}
		// note: os.Stat treats symlinks as their destination. os.Lstat would stat the link itself.
		info, err := os.Stat(nePath)
		if err != nil {
			if !os.IsNotExist(err) {
				logger.Warnw("ignoring non-NotExist error for required path", "path", nePath, "package", packageName, "error", err.Error())
			} else {
				logger.Warnw("a required path is missing, re-downloading", "path", nePath, "package", packageName)
				missingOrEmpty++
			}
		} else if info.Size() == 0 {
			missingOrEmpty++
			logger.Warnw("a required path is empty, re-downloading", "path", nePath, "package", packageName)
		}
	}
	return missingOrEmpty == 0
}

func (m *cloudManager) downloadPackage(ctx context.Context, url string, p config.PackageConfig, nonEmptyPaths []string) error {
	if dataDir := p.LocalDataDirectory(m.packagesDir); dirExists(dataDir) {
		if checkNonemptyPaths(p.Name, m.logger, nonEmptyPaths) {
			m.logger.Debugf("Package already downloaded at %s, skipping.", dataDir)
			return nil
		}
	}

	// Create the parent directory for the package type if it doesn't exist
	if err := os.MkdirAll(p.LocalDataParentDirectory(m.packagesDir), 0o700); err != nil {
		return err
	}

	// Delete legacy directory, silently fail if the cleanup fails
	// This can be cleaned up after a few RDK releases (APP-4066)
	if err := os.RemoveAll(p.LocalLegacyDataRootDirectory(m.packagesDir)); err != nil {
		utils.UncheckedError(err)
	}

	// Force redownload of package archive.
	if err := m.cleanup(p); err != nil {
		m.logger.Debug(err)
	}

	if p.Type == config.PackageTypeMlModel {
		symlinkPath, err := safeJoin(m.packagesDir, p.Name)
		if err == nil {
			if err := os.Remove(symlinkPath); err != nil {
				utils.UncheckedError(err)
			}
		}
	}

	var contentType string
	dstPath := p.LocalDownloadPath(m.packagesDir)
	if pathOnly, found := strings.CutPrefix(url, "file://"); found {
		src, err := os.Open(pathOnly) //nolint:gosec
		if err != nil {
			return err
		}
		defer src.Close()
		dst, err := os.Create(dstPath) //nolint:gosec
		if err != nil {
			return err
		}
		defer dst.Close()
		nBytes, err := io.Copy(dst, src)
		if err != nil {
			return err
		}
		m.logger.Debugf("copied %d bytes to %s", nBytes, dstPath)
		// note: we're relying on the assumption that this is a synthetic package which passed tarballExtensionsRegexp
		contentType = allowedContentType
	} else {
		var err error
		// Download from GCS
		_, contentType, err = m.downloadFileFromGCSURL(ctx, url, dstPath, m.cloudConfig.ID, m.cloudConfig.Secret)
		if err != nil {
			return err
		}
	}

	if contentType != allowedContentType {
		utils.UncheckedError(m.cleanup(p))
		return fmt.Errorf("unknown content-type for package %s", contentType)
	}

	// unpack to temp directory to ensure we do an atomic rename once finished.
	tmpDataPath, err := os.MkdirTemp(p.LocalDataParentDirectory(m.packagesDir), "*.tmp")
	if err != nil {
		return errors.Wrap(err, "failed to create temp data dir path")
	}

	defer func() {
		// cleanup archive file.
		if err := os.Remove(dstPath); err != nil {
			m.logger.Debug(err)
		}
		if err := os.RemoveAll(tmpDataPath); err != nil {
			m.logger.Debug(err)
		}
	}()

	// unzip archive.
	err = unpackFile(ctx, dstPath, tmpDataPath)
	if err != nil {
		utils.UncheckedError(m.cleanup(p))
		return err
	}

	err = os.Rename(tmpDataPath, p.LocalDataDirectory(m.packagesDir))
	if err != nil {
		utils.UncheckedError(m.cleanup(p))
		return err
	}

	return nil
}

func (m *cloudManager) cleanup(p config.PackageConfig) error {
	return multierr.Combine(
		os.RemoveAll(p.LocalDataDirectory(m.packagesDir)),
		os.Remove(p.LocalDownloadPath(m.packagesDir)),
	)
}

func (m *cloudManager) downloadFileFromGCSURL(
	ctx context.Context,
	url string,
	downloadPath string,
	partID string,
	partSecret string,
) (string, string, error) {
	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	getReq.Header.Add("part_id", partID)
	getReq.Header.Add("secret", partSecret)
	if err != nil {
		return "", "", err
	}

	//nolint:bodyclose /// closed in UncheckedErrorFunc
	resp, err := m.httpClient.Do(getReq)
	if err != nil {
		return "", "", err
	}
	defer utils.UncheckedErrorFunc(resp.Body.Close)

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("invalid status code %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	checksum := getGoogleHash(resp.Header, "crc32c")

	//nolint:gosec // safe
	out, err := os.Create(downloadPath)
	if err != nil {
		return checksum, contentType, err
	}
	defer utils.UncheckedErrorFunc(out.Close)

	hash := crc32Hash()
	w := io.MultiWriter(out, hash)

	_, err = io.CopyN(w, resp.Body, maxPackageSize)
	if err != nil && !errors.Is(err, io.EOF) {
		utils.UncheckedError(os.Remove(downloadPath))
		return checksum, contentType, err
	}

	checksumBytes, err := base64.StdEncoding.DecodeString(checksum)
	if err != nil {
		return "", "", errors.Wrapf(err, "failed to decode expected checksum: %s", checksum)
	}

	trimmedChecksumBytes := trimLeadingZeroes(checksumBytes)
	trimmedOutHashBytes := trimLeadingZeroes(hash.Sum(nil))

	if !bytes.Equal(trimmedOutHashBytes, trimmedChecksumBytes) {
		utils.UncheckedError(os.Remove(downloadPath))
		return checksum, contentType, errors.Errorf(
			"download did not match expected hash:\n"+
				"  pre-trimmed: %x vs. %x\n"+
				"  trimmed:     %x vs. %x",
			checksumBytes, hash.Sum(nil),
			trimmedChecksumBytes, trimmedOutHashBytes,
		)
	}

	return checksum, contentType, nil
}

func trimLeadingZeroes(data []byte) []byte {
	if len(data) == 0 {
		return []byte{}
	}

	for i, b := range data {
		if b != 0x00 {
			return data[i:]
		}
	}

	// If all bytes are zero, return a single zero byte
	return []byte{0x00}
}

func unpackFile(ctx context.Context, fromFile, toDir string) error {
	if err := os.MkdirAll(toDir, 0o700); err != nil {
		return err
	}

	//nolint:gosec // safe
	f, err := os.Open(fromFile)
	if err != nil {
		return err
	}
	defer utils.UncheckedErrorFunc(f.Close)

	archive, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer utils.UncheckedErrorFunc(archive.Close)

	type link struct {
		Name string
		Path string
	}
	links := []link{}
	symlinks := []link{}

	tarReader := tar.NewReader(archive)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		header, err := tarReader.Next()

		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return errors.Wrap(err, "read tar")
		}

		path := header.Name

		if path == "" || path == "./" {
			continue
		}

		if path, err = safeJoin(toDir, path); err != nil {
			return err
		}

		info := header.FileInfo()

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.Mkdir(path, info.Mode()); err != nil {
				return errors.Wrapf(err, "failed to create directory %s", path)
			}

		case tar.TypeReg:
			// This is required because it is possible create tarballs without a directory entry
			// but whose files names start with a new directory prefix
			// Ex: tar -czf package.tar.gz ./bin/module.exe
			parent := filepath.Dir(path)
			if err := os.MkdirAll(parent, 0o700); err != nil {
				return errors.Wrapf(err, "failed to create directory %q", parent)
			}
			//nolint:gosec // path sanitized with safeJoin
			outFile, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600|info.Mode().Perm())
			if err != nil {
				return errors.Wrapf(err, "failed to create file %s", path)
			}
			if _, err := io.CopyN(outFile, tarReader, maxPackageSize); err != nil && !errors.Is(err, io.EOF) {
				return errors.Wrapf(err, "failed to copy file %s", path)
			}
			if err := outFile.Sync(); err != nil {
				return errors.Wrapf(err, "failed to sync %s", path)
			}
			utils.UncheckedError(outFile.Close())
		case tar.TypeLink:
			name := header.Linkname

			if name, err = safeJoin(toDir, name); err != nil {
				return err
			}
			links = append(links, link{Path: path, Name: name})
		case tar.TypeSymlink:
			linkTarget, err := safeLink(toDir, header.Linkname)
			if err != nil {
				return err
			}
			symlinks = append(symlinks, link{Path: path, Name: linkTarget})
		}
	}

	// Now we make another pass creating the links
	for i := range links {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := linkFile(links[i].Name, links[i].Path); err != nil {
			return errors.Wrapf(err, "failed to create link %s", links[i].Path)
		}
	}

	for i := range symlinks {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := linkFile(symlinks[i].Name, symlinks[i].Path); err != nil {
			return errors.Wrapf(err, "failed to create link %s", links[i].Path)
		}
	}

	return nil
}

func getGoogleHash(headers http.Header, hashType string) string {
	hashes := headers.Values("x-goog-hash")
	hashesMap := make(map[string]string, len(hashes))
	for _, v := range hashes {
		split := strings.SplitN(v, "=", 2)
		if len(split) != 2 {
			continue
		}
		hashesMap[split[0]] = split[1]
	}

	return hashesMap[hashType]
}

func crc32Hash() hash.Hash32 {
	return crc32.New(crc32.MakeTable(crc32.Castagnoli))
}

// safeJoin performs a filepath.Join of 'parent' and 'subdir' but returns an error
// if the resulting path points outside of 'parent'.
func safeJoin(parent, subdir string) (string, error) {
	res := filepath.Join(parent, subdir)
	if !strings.HasSuffix(parent, string(os.PathSeparator)) {
		parent += string(os.PathSeparator)
	}
	if !strings.HasPrefix(res, parent) {
		return res, errors.Errorf("unsafe path join: '%s' with '%s'", parent, subdir)
	}
	return res, nil
}

func safeLink(parent, link string) (string, error) {
	if filepath.IsAbs(link) {
		return link, errors.Errorf("unsafe path link: '%s' with '%s', cannot be absolute path", parent, link)
	}

	_, err := safeJoin(parent, link)
	if err != nil {
		return link, err
	}
	return link, nil
}

func linkFile(from, to string) error {
	link, err := os.Readlink(to)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if link == from {
		return nil
	}

	// remove any existing link or SymLink will fail.
	if link != "" {
		utils.UncheckedError(os.Remove(from))
	}

	return os.Symlink(from, to)
}

func dirExists(dir string) bool {
	info, err := os.Stat(dir)
	if err != nil {
		return false
	}

	return info.IsDir()
}
