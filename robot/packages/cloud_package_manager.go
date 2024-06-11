package packages

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"net/http"
	"net/url"
	"os"
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
	rutils "go.viam.com/rdk/utils"
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

// SyncAll syncs all given packages and removes any not in the list from the local file system.
func (m *cloudManager) Sync(ctx context.Context, packages []config.PackageConfig, modules []config.Module) error {
	var outErr error

	// Only allow one rdk process to operate on the manager at once. This is generally safe to keep locked for an extended period of time
	// since the config reconfiguration process is handled by a single thread.
	m.mu.Lock()
	defer m.mu.Unlock()

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

		// Lookup the packages http url
		includeURL := true

		packageType, err := config.PackageTypeToProto(p.Type)
		if err != nil {
			m.logger.Warnw("failed to get package type", "package", p.Name, "error", err)
		}
		resp, err := m.client.GetPackage(ctx, &pb.GetPackageRequest{
			Id:         p.Package,
			Version:    p.Version,
			Type:       packageType,
			IncludeUrl: &includeURL,
		})
		if err != nil {
			m.logger.Errorf("Failed fetching package details for package %s:%s, %s", p.Package, p.Version, err)
			outErr = multierr.Append(outErr, errors.Wrapf(err, "failed loading package url for %s:%s", p.Package, p.Version))
			continue
		}

		m.logger.Debugf("Downloading from %s", sanitizeURLForLogs(resp.Package.Url))

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
		err = installPackage(ctx, m.logger, m.packagesDir, resp.Package.Url, p, nonEmptyPaths,
			func(ctx context.Context, url, dstPath string) (string, error) {
				_, contentType, err := m.downloadFileFromGCSURL(ctx, url, dstPath, m.cloudConfig.ID, m.cloudConfig.Secret)
				return contentType, err
			},
		)
		if err != nil {
			m.logger.Errorf("Failed downloading package %s:%s from %s, %s", p.Package, p.Version, sanitizeURLForLogs(resp.Package.Url), err)
			outErr = multierr.Append(outErr, errors.Wrapf(err, "failed downloading package %s:%s from %s",
				p.Package, p.Version, sanitizeURLForLogs(resp.Package.Url)))
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

	allErrors = commonCleanup(m.logger, expectedPackageDirectories, m.packagesDataDir)
	if allErrors != nil {
		return allErrors
	}

	allErrors = multierr.Append(allErrors, m.mlModelSymlinkCleanup())
	return allErrors
}

// symlink packages/package-name to packages/data/ml_model/orgid-package-name-ver for backwards compatibility
// TODO(RSDK-4386) Preserved for backwards compatibility. Could be removed or extended to other types in the future.
func (m *cloudManager) mLModelSymlinkCreation(p config.PackageConfig) error {
	symlinkPath, err := rutils.SafeJoinDir(m.packagesDir, p.Name)
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

		symlinkPath, err := rutils.SafeJoinDir(m.packagesDir, f.Name())
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

func safeLink(parent, link string) (string, error) {
	if filepath.IsAbs(link) {
		return link, errors.Errorf("unsafe path link: '%s' with '%s', cannot be absolute path", parent, link)
	}

	_, err := rutils.SafeJoinDir(parent, link)
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

// SyncOne is a no-op for cloudManager.
func (m *cloudManager) SyncOne(ctx context.Context, mod config.Module) error {
	return nil
}
