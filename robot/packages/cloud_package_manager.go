package packages

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
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

	"github.com/hashicorp/go-getter"
	errw "github.com/pkg/errors"
	"go.uber.org/multierr"
	pb "go.viam.com/api/app/packages/v1"
	"go.viam.com/utils"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	rutils "go.viam.com/rdk/utils"
	"go.viam.com/rdk/utils/diskusage"
)

const (
	allowedContentType = "application/x-gzip"
)

var (
	_ Manager       = (*cloudManager)(nil)
	_ ManagerSyncer = (*cloudManager)(nil)

	// the test suite can set this to non-zero to test resume behavior
	maxBytesForTesting int64
)

type cloudManager struct {
	resource.Named
	// we assume the config is immutable for the lifetime of the process
	resource.TriviallyReconfigurable
	client          pb.PackageServiceClient
	httpClient      http.Client
	packagesDataDir string
	packagesDir     string
	cloudConfig     config.Cloud

	managedPackages map[PackageName]*config.PackageConfig
	mu              sync.RWMutex

	// statusMu guards packageStatuses separately from mu so that statuses (including
	// download progress) remain readable while a long-running Sync holds mu.
	statusMu        sync.Mutex
	packageStatuses map[PackageName]*PackageStatus

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
		packageStatuses: make(map[PackageName]*PackageStatus),
		logger:          logger,
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

	return p.LocalDataDirectory(m.packagesDir), nil
}

// Close manager.
func (m *cloudManager) Close(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.httpClient.CloseIdleConnections()
	return nil
}

// PackageStatuses returns a snapshot of the current status for all managed packages.
func (m *cloudManager) PackageStatuses() []PackageStatus {
	m.statusMu.Lock()
	defer m.statusMu.Unlock()
	statuses := make([]PackageStatus, 0, len(m.packageStatuses))
	for _, s := range m.packageStatuses {
		statuses = append(statuses, *s)
	}
	return statuses
}

// SetPackageState updates the in-memory state for the named package. Used by local_robot
// to transition a module package through the first-run stage.
func (m *cloudManager) SetPackageState(name PackageName, state PackageState, errMsg string) {
	m.statusMu.Lock()
	defer m.statusMu.Unlock()
	if s, ok := m.packageStatuses[name]; ok {
		s.State = state
		s.Error = errMsg
		s.LastUpdated = time.Now()
	}
}

// setPackageStatus sets the full status entry for a package. Download progress is preserved
// when updating an existing entry for the same package version.
func (m *cloudManager) setPackageStatus(p config.PackageConfig, state PackageState, errMsg string) {
	m.statusMu.Lock()
	defer m.statusMu.Unlock()
	status := &PackageStatus{
		Name:        p.Name,
		Type:        p.Type,
		State:       state,
		Error:       errMsg,
		LastUpdated: time.Now(),
		Version:     p.Version,
	}
	name := PackageName(p.Name)
	if prev, ok := m.packageStatuses[name]; ok && prev.Version == p.Version {
		status.BytesDownloaded = prev.BytesDownloaded
		status.TotalBytes = prev.TotalBytes
	}
	m.packageStatuses[name] = status
}

// setDownloadProgress records how many bytes of the package tarball have been downloaded
// so far, along with the total tarball size in bytes (zero if unknown). Negative values
// (e.g. an unset Content-Length of -1) are recorded as zero rather than wrapping around
// in the conversion to uint64.
func (m *cloudManager) setDownloadProgress(name PackageName, bytesDownloaded, totalBytes int64) {
	m.statusMu.Lock()
	defer m.statusMu.Unlock()
	if s, ok := m.packageStatuses[name]; ok {
		s.BytesDownloaded = uint64(max(bytesDownloaded, 0))
		s.TotalBytes = uint64(max(totalBytes, 0))
		s.LastUpdated = time.Now()
	}
}

// SyncAll syncs all given packages and removes any not in the list from the local file system.
func (m *cloudManager) Sync(ctx context.Context, packages []config.PackageConfig, modules []config.Module) error {
	var outErr error

	// Only allow one rdk process to operate on the manager at once. This is generally safe to keep locked for an extended period of time
	// since the config reconfiguration process is handled by a single thread.
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Debug("Evaluating package sync...")

	newManagedPackages := make(map[PackageName]*config.PackageConfig, len(packages))

	// Process the packages that are new or changed
	changedPackages, existingPackages := m.validateAndGetChangedPackages(packages)

	// Updated managed map with existingPackages
	for _, p := range existingPackages {
		statusFile, err := readStatusFile(p, m.packagesDir)
		if err != nil {
			m.logger.Errorf("Failed reading status file for synced package %s: %v", p.Name, err)
			m.setPackageStatus(p, PackageStateFailed, fmt.Sprintf("failed to read status file: %v", err.Error()))
			return multierr.Append(outErr, err)
		}
		if statusFile.Status == syncStatusFailed {
			m.logger.Errorf("Package %s was fully downloaded but failed to unzip, please try a different version", p.Name)
			m.setPackageStatus(p, PackageStateFailed, "failed to unzip, please try a different version")
			return multierr.Append(outErr, fmt.Errorf("package %s was fully downloaded but failed to unzip, please try a different version", p.Name))
		}
		// Seed in-memory status from the on-disk status file.
		m.setPackageStatus(p, PackageStateReady, "")
		newManagedPackages[PackageName(p.Name)] = &p
	}

	if len(changedPackages) > 0 {
		m.logger.Info("Package changes have been detected, starting sync")
	}

	start := time.Now()
	for idx, p := range changedPackages {
		p := p
		pkgStart := time.Now()
		if err := ctx.Err(); err != nil {
			m.logger.Errorf("Context canceled. Canceling cloud package manager sync. Time spent: %v", time.Since(start))
			return multierr.Append(outErr, err)
		}

		m.logger.Debugf("Starting package sync [%d/%d] %s:%s", idx+1, len(changedPackages), p.Package, p.Version)
		m.setPackageStatus(p, PackageStateDownloading, "")

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
			m.logger.Errorf("Failed fetching package details for package %s:%s. Err: %v", p.Package, p.Version, err)
			m.setPackageStatus(p, PackageStateFailed, fmt.Sprintf("failed to fetch package details: %v", err.Error()))
			outErr = multierr.Append(outErr, fmt.Errorf("failed loading package url for %s:%s %w", p.Package, p.Version, err))
			continue
		}

		m.logger.Debugf("Downloading from %s", sanitizeURLForLogs(resp.Package.Url))

		// download package from a http endpoint
		err = installPackage(ctx, m.logger, m.packagesDir, resp.Package.Url, p, true,
			func(ctx context.Context, url, dstPath string) (string, string, error) {
				statusFile := packageSyncFile{
					PackageID:       p.Package,
					Version:         p.Version,
					ModifiedTime:    time.Now(),
					Status:          syncStatusDownloading,
					TarballChecksum: "",
				}

				err = writeStatusFile(p, statusFile, m.packagesDir)
				if err != nil {
					return "", "", err
				}

				checksum, contentType, err := m.downloadFileWithChecksum(ctx, url, dstPath, PackageName(p.Name))
				if err != nil {
					return checksum, contentType, err
				}

				// The tarball is fully downloaded; installPackage will now verify and
				// extract it.
				m.setPackageStatus(p, PackageStateLoading, "")
				return checksum, contentType, nil
			},
		)
		if err != nil {
			m.logger.Errorf(
				"Failed downloading/unzipping package %s:%s from %s, %s",
				p.Package,
				p.Version,
				sanitizeURLForLogs(resp.Package.Url),
				err,
			)
			m.setPackageStatus(p, PackageStateFailed, fmt.Sprintf("failed downloading/unzipping package: %v", err.Error()))
			outErr = multierr.Append(outErr, fmt.Errorf("failed downloading/unzipping package %s:%s from %s %w",
				p.Package, p.Version, sanitizeURLForLogs(resp.Package.Url), err))
			continue
		}

		if p.Type == config.PackageTypeMlModel {
			if symlinkErr := m.mLModelSymlinkCreation(p); symlinkErr != nil {
				m.logger.Errorf("Error creating ml model symlink. Err: %v", symlinkErr)
				m.setPackageStatus(p, PackageStateFailed, fmt.Sprintf("failed to create ml model symlink: %v", err.Error()))
				outErr = multierr.Append(outErr, symlinkErr)
			}
		}

		// add to managed packages
		m.setPackageStatus(p, PackageStateReady, "")
		newManagedPackages[PackageName(p.Name)] = &p

		m.logger.Debugf("Package sync complete [%d/%d] %s:%s after %v", idx+1, len(changedPackages), p.Package, p.Version, time.Since(pkgStart))
	}

	if len(changedPackages) > 0 {
		m.logger.Infof("Package sync complete after %v", time.Since(start))
	}

	// Prune packageStatuses to match the requested config so stale entries from removed
	// packages don't accumulate across reconfigures. Prune against the requested packages
	// rather than newManagedPackages: failed packages are absent from the managed set but
	// must keep their Failed status visible.
	requestedPackages := make(map[PackageName]bool, len(packages))
	for _, p := range packages {
		requestedPackages[PackageName(p.Name)] = true
	}
	m.statusMu.Lock()
	for name := range m.packageStatuses {
		if !requestedPackages[name] {
			delete(m.packageStatuses, name)
		}
	}
	m.statusMu.Unlock()

	// swap for new managed packags.
	m.managedPackages = newManagedPackages

	return outErr
}

func (m *cloudManager) validateAndGetChangedPackages(
	packages []config.PackageConfig,
) ([]config.PackageConfig, []config.PackageConfig) {
	changed := make([]config.PackageConfig, 0)
	existing := make([]config.PackageConfig, 0)
	for _, p := range packages {
		// don't consider invalid config as synced or unsynced
		if err := p.Validate(""); err != nil {
			m.logger.Errorw("package config validation error; skipping", "package", p.Name, "error", err)
			continue
		}
		newPackage := p
		if packageIsSynced(p, m.packagesDir, m.logger) {
			existing = append(existing, newPackage)
			m.logger.Debugf("Package %s:%s already managed, skipping", p.Package, p.Version)
		} else {
			changed = append(changed, newPackage)
		}
	}
	return changed, existing
}

// Cleanup removes all unknown packages from the working directory.
func (m *cloudManager) Cleanup(ctx context.Context) error {
	// Only allow one rdk process to operate on the manager at once. This is generally safe to keep locked for an extended period of time
	// since the config reconfiguration process is handled by a single thread.
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Debug("Starting package cleanup...")

	var allErrors error

	expectedPackageDirectories := map[string]bool{}
	for _, pkg := range m.managedPackages {
		expectedPackageDirectories[pkg.LocalDataDirectory(m.packagesDir)] = true
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

	localDataDir := p.LocalDataDirectory(m.packagesDir)
	if err := linkFile(localDataDir, symlinkPath); err != nil {
		return fmt.Errorf("failed linking ml_model package %s:%s. localDataDir: %q symlinkPath: %q Err: %w",
			p.Package, p.Version, localDataDir, symlinkPath, err)
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

// LogProgressWriter is a writer that logs progress.
type logProgressWriter struct {
	totalBytes   int64
	name         string
	startTime    time.Time
	lastLogTime  time.Time
	logFrequency time.Duration
	logger       logging.Logger
}

func newLogProgressWriter(logger logging.Logger, jobName string, totalBytes int64) *logProgressWriter {
	return &logProgressWriter{
		totalBytes:   totalBytes,
		startTime:    time.Now(),
		lastLogTime:  time.Now(),
		name:         jobName,
		logFrequency: time.Second * 5,
		logger:       logger,
	}
}

// log progress at Info level, update lastLogTime, return logged message.
func (wc *logProgressWriter) Update(curSize int64) string {
	currentTime := time.Now()
	if currentTime.Equal(wc.startTime) {
		return ""
	}

	var msg string
	if curSize < wc.totalBytes || wc.totalBytes == 0 {
		// unknown pct if totalBytes is 0. Log what's available.
		var pctStr string
		if wc.totalBytes == 0 {
			pctStr = "?%"
		} else {
			pctStr = fmt.Sprintf("%.0f%%", float64(curSize)/float64(wc.totalBytes)*100)
		}
		// todo: more useful to compute data rate from last tick, rather than from start
		msg = fmt.Sprintf("%s: downloaded %.2f / %.2f MB (%s) [%.0f KB/s]",
			wc.name,
			float64(curSize)/1e6,
			float64(wc.totalBytes)/1e6,
			pctStr,
			float64(curSize)/currentTime.Sub(wc.startTime).Seconds()/1024)
		wc.logger.Info(msg)
		wc.lastLogTime = currentTime
	} else {
		msg = fmt.Sprintf("%s: downloaded %.2f MB (100%%) in %v",
			wc.name,
			float64(curSize)/1e6,
			currentTime.Sub(wc.startTime))
		wc.logger.Info(msg)
	}
	return msg
}

// downloader with header-based checksum logic and partials support. name is used to
// report download progress on the package's status entry.
func (m *cloudManager) downloadFileWithChecksum(
	ctx context.Context,
	rawURL string,
	downloadPath string,
	name PackageName,
) (string, string, error) {
	getReq, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)

	headers := make(http.Header)
	if m.cloudConfig.APIKey.IsFullySet() {
		headers.Add("key_id", m.cloudConfig.APIKey.ID)
		headers.Add("key", m.cloudConfig.APIKey.Key)
	} else {
		headers.Add("part_id", m.cloudConfig.ID)
		headers.Add("secret", m.cloudConfig.Secret)
	}
	getReq.Header = headers

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

	// Cheap pre-filter: refuse before downloading something that obviously won't fit (artifact +
	// reserved floor). Guard on > 0 both because a missing size can't be checked against and
	// because uint64(-1) would wrap; our GCS-backed downloads always report a size, so this
	// effectively always runs. If a size is ever absent, the unpackFile guard and ENOSPC backstop.
	if resp.ContentLength > 0 {
		// A resumable download appends a ranged GET to any partial already on disk, so only the
		// remaining bytes get written. Requiring the full Content-Length would falsely refuse a
		// download that's mostly complete (and double-count the partial, which already occupies space).
		// Safe whether or not the server honors ranges: go-getter writes in place (no O_TRUNC), so the
		// file only grows from the existing partial to Content-Length either way. The resume contract
		// is exercised by the "resumable" test in cloud_package_manager_test.go.
		remaining := uint64(resp.ContentLength)
		if stat, statErr := os.Stat(downloadPath); statErr == nil {
			if existing := uint64(stat.Size()); existing < remaining {
				remaining -= existing
			} else {
				remaining = 0 // already have the whole file (or a stale larger one); only the floor matters
			}
		}
		required := remaining + diskusage.MinFreeBytes
		if _, err := checkDiskSpace(m.logger, downloadPath, "package download", required,
			"content_size", diskusage.FormatBytes(uint64(resp.ContentLength))); err != nil {
			return "", "", err
		}
	}

	contentType := resp.Header.Get("Content-Type")
	checksum := getGoogleHash(resp.Header, "crc32c")
	expectedChecksumBytes, err := base64.StdEncoding.DecodeString(checksum)
	if err != nil {
		return "", "", fmt.Errorf("failed to decode expected checksum: %q %w", checksum, err)
	}

	totalBytes := resp.ContentLength
	if totalBytes < 0 {
		totalBytes = 0
	}

	var startBytes int64
	if stat, err := os.Stat(downloadPath); err == nil {
		m.logger.Infow("download to existing", "dest", downloadPath, "size", stat.Size())
		startBytes = stat.Size()
	}
	m.setDownloadProgress(name, startBytes, totalBytes)

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", "", err
	}

	g := getter.HttpGetter{
		MaxBytes: maxBytesForTesting,
		Header:   headers,
		Client:   &m.httpClient,
	}
	g.SetClient(&getter.Client{Ctx: ctx})
	progressCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go utils.PanicCapturingGo(func() {
		fileSizeProgress(progressCtx, m.logger, downloadPath, totalBytes, func(curBytes int64) {
			m.setDownloadProgress(name, curBytes, totalBytes)
		})
	})
	if err := g.GetFile(downloadPath, parsedURL); err != nil {
		return "", "", errw.Wrap(err, "downloading file")
	}

	hash := crc32Hash()
	destFile, err := os.Open(downloadPath) //nolint:gosec
	if err != nil {
		return "", "", err
	}
	defer utils.UncheckedErrorFunc(destFile.Close)
	downloadedBytes, err := io.Copy(hash, destFile)
	if err != nil {
		return "", "", err
	}

	// The download is complete; record the final byte counts. If the server did not
	// report a Content-Length, fall back to the actual downloaded size.
	if totalBytes == 0 {
		totalBytes = downloadedBytes
	}
	m.setDownloadProgress(name, downloadedBytes, totalBytes)

	trimmedChecksumBytes := trimLeadingZeroes(expectedChecksumBytes)
	trimmedOutHashBytes := trimLeadingZeroes(hash.Sum(nil))

	if !bytes.Equal(trimmedOutHashBytes, trimmedChecksumBytes) {
		utils.UncheckedError(os.Remove(downloadPath))
		return checksum, contentType, fmt.Errorf(
			"download did not match expected hash:\n"+
				"  pre-trimmed: %x vs. %x\n"+
				"  trimmed:     %x vs. %x",
			expectedChecksumBytes, hash.Sum(nil),
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
		return link, fmt.Errorf("cannot link '%s' to '%s', symlink target '%s' cannot be an absolute path", link, parent, link)
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

// SyncOne is a no-op for cloudManager.
func (m *cloudManager) SyncOne(ctx context.Context, mod config.Module) error {
	return nil
}
