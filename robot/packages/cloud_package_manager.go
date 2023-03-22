package packages

import (
	"archive/tar"
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

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	pb "go.viam.com/api/app/packages/v1"
	"go.viam.com/utils"

	"go.viam.com/rdk/config"
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
	client          pb.PackageServiceClient
	httpClient      http.Client
	packagesDataDir string
	packagesDir     string

	managedPackages map[PackageName]*managedPackage
	mu              sync.RWMutex

	logger golog.Logger
}

// NewCloudManager creates a new manager with the given package service client and directory to sync to.
func NewCloudManager(client pb.PackageServiceClient, packagesDir string, logger golog.Logger) (ManagerSyncer, error) {
	packagesDataDir := filepath.Join(packagesDir, ".data")

	if err := os.MkdirAll(packagesDir, 0o700); err != nil {
		return nil, err
	}

	if err := os.MkdirAll(packagesDataDir, 0o700); err != nil {
		return nil, err
	}

	return &cloudManager{
		client:          client,
		httpClient:      http.Client{Timeout: time.Minute * 30},
		packagesDir:     packagesDir,
		packagesDataDir: packagesDataDir,
		managedPackages: make(map[PackageName]*managedPackage),
		logger:          logger.Named("package_manager"),
	}, nil
}

// PackagePath returns the package if it exists and already download. If it does not exist it returns a ErrPackageMissing error.
func (m *cloudManager) PackagePath(name PackageName) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, ok := m.managedPackages[name]
	if !ok {
		return "", ErrPackageMissing
	}

	return m.localNamedPath(p.thePackage), nil
}

func (m *cloudManager) RefPath(refPath string) (string, error) {
	ref := config.GetPackageReference(refPath)

	// If no reference just return original path.
	if ref == nil {
		return refPath, nil
	}

	packagePath, err := m.PackagePath(PackageName(ref.Package))
	if err != nil {
		return "", err
	}

	return path.Join(packagePath, path.Clean(ref.PathInPackage)), nil
}

// Close manager.
func (m *cloudManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.httpClient.CloseIdleConnections()
	return nil
}

// SyncAll syncs all given packages and removes any not in the list from the local file system.
func (m *cloudManager) Sync(ctx context.Context, packages []config.PackageConfig) error {
	var outErr error

	// Only allow one rdk process to operate on the manager at once. This is generally safe to keep locked for an extended period of time
	// since the config reconfiguration process is handled by a single thread.
	m.mu.Lock()
	defer m.mu.Unlock()

	newManagedPackages := make(map[PackageName]*managedPackage, len(packages))

	for idx, p := range packages {
		select {
		case <-ctx.Done():
			return multierr.Append(outErr, ctx.Err())
		default:
		}

		start := time.Now()
		m.logger.Debugf("Starting package sync [%d/%d] %s:%s", idx+1, len(packages), p.Package, p.Version)

		// Package exists in known cache.
		existing, ok := m.managedPackages[PackageName(p.Name)]
		if ok {
			if existing.thePackage.Package == p.Package && existing.thePackage.Version == p.Version {
				m.logger.Debug("  Package already managed, skipping")

				newManagedPackages[PackageName(p.Name)] = existing
				delete(m.managedPackages, PackageName(p.Name))
				continue
			}
			// anything left over in the m.managedPackages will be cleaned up later.
		}

		// Lookup the packages http url
		includeURL := true
		resp, err := m.client.GetPackage(ctx, &pb.GetPackageRequest{Id: p.Package, Version: p.Version, IncludeUrl: &includeURL})
		if err != nil {
			m.logger.Errorf("Failed fetching package details for package %s:%s, %s", p.Package, p.Version, err)
			outErr = multierr.Append(outErr, errors.Wrapf(err, "failed loading package url for %s:%s", p.Package, p.Version))
			continue
		}

		m.logger.Debugf("  Downloading from %s", sanitizeURLForLogs(resp.Package.Url))

		// load package from a http endpoint
		err = m.loadFile(ctx, resp.Package.Url, p)
		if err != nil {
			m.logger.Errorf("Failed downloading package %s:%s from %s, %s", p.Package, p.Version, sanitizeURLForLogs(resp.Package.Url), err)
			outErr = multierr.Append(outErr, errors.Wrapf(err, "failed downloading package %s:%s from %s",
				p.Package, p.Version, sanitizeURLForLogs(resp.Package.Url)))
			continue
		}

		err = linkFile(m.localDataPath(p), m.localNamedPath(p))
		if err != nil {
			m.logger.Errorf("Failed linking package %s:%s, %s", p.Package, p.Version, err)
			outErr = multierr.Append(outErr, err)
			continue
		}

		// add to managed packages
		newManagedPackages[PackageName(p.Name)] = &managedPackage{thePackage: p, modtime: time.Now()}

		m.logger.Debugf("  Sync complete after %dms", time.Since(start).Milliseconds())
	}

	// swap for new managed packags.
	m.managedPackages = newManagedPackages

	return outErr
}

// Cleanup removes all unknown packages from the working directory.
func (m *cloudManager) Cleanup(ctx context.Context) error {
	m.logger.Debug("Starting package cleanup")

	// Only allow one rdk process to operate on the manager at once. This is generally safe to keep locked for an extended period of time
	// since the config reconfiguration process is handled by a single thread.
	m.mu.Lock()
	defer m.mu.Unlock()

	var allErrors error

	// packageDir will contain either symlinks to the packages or the .data directory.
	files, err := os.ReadDir(m.packagesDir)
	if err != nil {
		return err
	}

	// keep track of known packages by their hashed name from the package id and version.
	knownPackages := make(map[string]bool)

	// first remove all symlinks to the packages themself.
	for _, f := range files {
		if f.Type()&os.ModeSymlink == os.ModeSymlink {
			// if managed skip removing package
			if p, ok := m.managedPackages[PackageName(f.Name())]; ok {
				knownPackages[hashName(p.thePackage)] = true
				continue
			}

			m.logger.Infof("Cleaning up unused package link %s", f.Name())

			// Remove logical symlink to package
			if err := os.Remove(path.Join(m.packagesDir, f.Name())); err != nil {
				allErrors = multierr.Append(allErrors, err)
			}
		}
	}

	// remove any packages in the .data dir that aren't known to the manager.
	// packageDir will contain either symlinks to the packages or the .data directory.
	files, err = os.ReadDir(m.packagesDataDir)
	if err != nil {
		return err
	}

	// remove any remaining files in the .data dir that should not be there.
	for _, f := range files {
		// if managed skip removing package
		if _, ok := knownPackages[f.Name()]; ok {
			continue
		}

		m.logger.Debugf("Cleaning up unused package %s", f.Name())

		// Remove logical symlink to package
		if err := os.RemoveAll(path.Join(m.packagesDataDir, f.Name())); err != nil {
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

func (m *cloudManager) loadFile(ctx context.Context, url string, p config.PackageConfig) error {
	// TODO(): validate integrity of directory.
	if dirExists(m.localDataPath(p)) {
		m.logger.Debug("  Package already downloaded, skipping.")
		return nil
	}

	// Force redownload of package archive.
	utils.UncheckedError(m.cleanup(p))
	utils.UncheckedError(os.Remove(m.localNamedPath(p)))

	// Download from GCS
	_, contentType, err := m.downloadFileFromGCSURL(ctx, url, p)
	if err != nil {
		return err
	}

	if contentType != allowedContentType {
		utils.UncheckedError(m.cleanup(p))
		return fmt.Errorf("unknown content-type for package %s", contentType)
	}

	// unpack to temp directory to ensure we do an atomic rename once finished.
	tmpDataPath, err := os.MkdirTemp(m.packagesDataDir, "*.tmp")
	if err != nil {
		return errors.Wrap(err, "failed to create temp data dir path")
	}

	defer func() {
		// cleanup archive file.
		utils.UncheckedError(os.Remove(m.localDownloadPath(p)))
		utils.UncheckedError(os.Remove(tmpDataPath))
	}()

	// unzip archive.
	err = m.unpackFile(ctx, m.localDownloadPath(p), tmpDataPath)
	if err != nil {
		utils.UncheckedError(m.cleanup(p))
		return err
	}

	err = os.Rename(tmpDataPath, m.localDataPath(p))
	if err != nil {
		utils.UncheckedError(m.cleanup(p))
		return err
	}

	return nil
}

func (m *cloudManager) cleanup(p config.PackageConfig) error {
	return multierr.Combine(
		os.RemoveAll(m.localDataPath(p)),
		os.Remove(m.localDownloadPath(p)),
	)
}

func (m *cloudManager) downloadFileFromGCSURL(ctx context.Context, url string, p config.PackageConfig) (string, string, error) {
	downloadPath := m.localDownloadPath(p)

	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
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

	outHash := base64.StdEncoding.EncodeToString(hash.Sum(nil))
	if outHash != checksum {
		utils.UncheckedError(os.Remove(downloadPath))
		return checksum, contentType, errors.Errorf("download did not match expected hash %s != %s", checksum, outHash)
	}

	return checksum, contentType, nil
}

func (m *cloudManager) unpackFile(ctx context.Context, fromFile, toDir string) error {
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
		select {
		case <-ctx.Done():
			return errors.New("interrupted")
		default:
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
		case tar.TypeReg, tar.TypeRegA:
			//nolint:gosec // path sanitized with safeJoin
			outFile, err := os.Create(path)
			if err != nil {
				return errors.Wrapf(err, "failed to create file %s", path)
			}
			if _, err := io.CopyN(outFile, tarReader, maxPackageSize); err != nil && !errors.Is(err, io.EOF) {
				return errors.Wrapf(err, "failed to copy file %s", path)
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
		select {
		case <-ctx.Done():
			return errors.New("interrupted")
		default:
		}
		if err := linkFile(links[i].Name, links[i].Path); err != nil {
			return errors.Wrapf(err, "failed to create link %s", links[i].Path)
		}
	}

	for i := range symlinks {
		select {
		case <-ctx.Done():
			return errors.New("interrupted")
		default:
		}
		if err := linkFile(symlinks[i].Name, symlinks[i].Path); err != nil {
			return errors.Wrapf(err, "failed to create link %s", links[i].Path)
		}
	}

	return nil
}

func (m *cloudManager) localDownloadPath(p config.PackageConfig) string {
	return filepath.Join(m.packagesDataDir, fmt.Sprintf("%s.download", hashName(p)))
}

func (m *cloudManager) localDataPath(p config.PackageConfig) string {
	return filepath.Join(m.packagesDataDir, hashName(p))
}

func (m *cloudManager) localNamedPath(p config.PackageConfig) string {
	return filepath.Join(m.packagesDir, p.Name)
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

func hashName(f config.PackageConfig) string {
	// replace / to avoid a directory path in the name. This will happen with `org/package` format.
	return fmt.Sprintf("%s-%s", strings.ReplaceAll(f.Package, "/", "-"), f.Version)
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
