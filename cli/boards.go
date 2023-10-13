package cli

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"go.uber.org/multierr"
	packagepb "go.viam.com/api/app/packages/v1"
	"go.viam.com/utils"
)

// supportedVersionRegex validates that the board version is semver 2.0.0 specification.
var supportedVersionRegex = regexp.MustCompile(`^(?P<major>0|[1-9]\d*)\.(?P<minor>0|[1-9]\d*)\.(?P<patch>0|[1-9]\d*)` +
	`(?:-(?P<prerelease>(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)` +
	`(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+(?P<buildmetadata>[0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)

const boardUploadMaximumSize = 32 * 1024

// UploadBoardDefsAction is the corresponding action for "board upload".
func UploadBoardDefsAction(ctx *cli.Context) error {
	orgArg := ctx.String(organizationFlag)
	nameArg := ctx.String(boardFlagName)
	versionArg := ctx.String(boardFlagVersion)
	if ctx.Args().Len() > 1 {
		return errors.New("too many arguments passed to upload command. " +
			"make sure to specify flag and optional arguments before the required positional package argument")
	}

	jsonPath := ctx.Args().First()

	if jsonPath == "" {
		return errors.New("no package to upload -- please provide a path containing your json file. use --help for more information")
	}

	// Validate the version is valid.
	if !supportedVersionRegex.MatchString(versionArg) {
		return fmt.Errorf("invalid version %s. Must use semver 2.0.0 specification for versions", versionArg)
	}

	client, err := newViamClient(ctx)
	if err != nil {
		return err
	}

	// get the org from the name or id.
	org, err := client.getOrg(orgArg)
	if err != nil {
		return err
	}

	// check if a package with this name and version already exists.
	err = client.boardDefsVersionExists(ctx, org.Id, nameArg, versionArg)
	if err != nil {
		return err
	}

	if !strings.HasSuffix(jsonPath, ".json") {
		return errors.New("The board definition file must be a .json")
	}

	_, err = client.uploadBoardDefsFile(nameArg, versionArg, org.Id, jsonPath)
	if err != nil {
		return err
	}

	printf(ctx.App.Writer, "Board definitions file was successfully uploaded!")
	return nil
}

// DownloadBoardDefsAction is the corresponding action for "board download".
func DownloadBoardDefsAction(c *cli.Context) error {
	orgArg := c.String(organizationFlag)
	nameArg := c.String(boardFlagName)
	versionArg := c.String(boardFlagVersion)

	if versionArg == "" {
		versionArg = "latest"
	}

	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	// get the org from the name or id.
	org, err := client.getOrg(orgArg)
	if err != nil {
		return err
	}

	err = client.downloadBoardDefsFile(nameArg, versionArg, org.Id)
	if err != nil {
		return err
	}

	printf(c.App.Writer, "%s board definitions successfully downloaded!", nameArg)

	return nil
}

// ListBoardDefsAction is the corresponding action for "board list".
func ListBoardDefsAction(c *cli.Context) error {
	orgArg := c.String(organizationFlag)

	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	// get the org from the name or id.
	org, err := client.getOrg(orgArg)
	if err != nil {
		return err
	}

	resp, err := client.listBoardDefsFiles(org.Id)
	if err != nil {
		return err
	}

	if len(resp.Packages) == 0 {
		printf(c.App.Writer, "orginization %s does not have any board definitions packages", orgArg)
	}

	for i := range resp.Packages {
		printf(c.App.Writer, "%s version %s", resp.Packages[i].Info.Name, resp.Packages[i].Info.Version)
	}

	return nil
}

func (c *viamClient) uploadBoardDefsFile(
	name string,
	version string,
	orgID string,
	jsonPath string,
) (*packagepb.CreatePackageResponse, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}
	ctx := c.c.Context

	jsonFile, err := os.Open(filepath.Clean(jsonPath))
	if err != nil {
		return nil, err
	}

	// Create an archive tar.gz file (required for packages).
	file, err := createArchive(jsonFile)
	if err != nil {
		return nil, errors.Wrap(err, "error creating archive")
	}

	// The board defs packages are small and never expected to be larger than the upload chunk size,
	// so we are sending in one chunk.
	// If the file is too big, return error.
	if file.Len() > boardUploadMaximumSize {
		return nil, fmt.Errorf("file is too large, must be under %d bytes", boardUploadMaximumSize)
	}

	stream, err := c.packageClient.CreatePackage(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "error starting CreatePackage stream")
	}

	stats, err := jsonFile.Stat()
	if err != nil {
		return nil, err
	}
	boardDefsFile := []*packagepb.FileInfo{{Name: name, Size: uint64(stats.Size())}}

	packageInfo := &packagepb.PackageInfo{
		OrganizationId: orgID,
		Name:           name,
		Version:        version,
		Type:           packagepb.PackageType_PACKAGE_TYPE_BOARD_DEFS,
		Files:          boardDefsFile,
		Metadata:       nil,
	}

	// send the package requests
	var errs error
	if err := sendPackageRequests(stream, file, packageInfo); err != nil {
		errs = multierr.Combine(errs, errors.Wrapf(err, "error syncing package"))
	}

	// close the stream and receive a response when finished.
	resp, err := stream.CloseAndRecv()
	if err != nil {
		errs = multierr.Combine(errs, errors.Wrapf(err, "received error response while syncing package"))
	}
	if errs != nil {
		return nil, errs
	}

	return resp, nil
}

func (c *viamClient) downloadBoardDefsFile(
	name string,
	version string,
	orgID string,
) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}
	ctx := c.c.Context

	includeURL := true
	packageType := packagepb.PackageType_PACKAGE_TYPE_BOARD_DEFS

	// the packageID is the orgid/name.
	packageID := fmt.Sprintf("%s/%s", orgID, name)

	req := &packagepb.GetPackageRequest{
		Id:         packageID,
		Version:    version,
		Type:       &packageType,
		IncludeUrl: &includeURL,
	}

	response, err := c.packageClient.GetPackage(ctx, req)
	if err != nil {
		return errors.Wrap(err, "could not retrieve the requested package")
	}

	currentDir, err := os.Getwd()
	if err != nil {
		return err
	}

	// download the file from the gcs url into the current directory.
	err = c.downloadFile(ctx, currentDir, response.Package.Url)
	if err != nil {
		return err
	}

	return nil
}

func (c *viamClient) listBoardDefsFiles(orgID string) (*packagepb.ListPackagesResponse, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}
	ctx := c.c.Context

	packageType := packagepb.PackageType_PACKAGE_TYPE_BOARD_DEFS

	req := &packagepb.ListPackagesRequest{
		Type:           &packageType,
		OrganizationId: orgID,
	}

	response, err := c.packageClient.ListPackages(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, "could not list the requested packages")
	}
	return response, nil
}

// helper function to check if a package with this name and version already exists.
func (c *viamClient) boardDefsVersionExists(ctx *cli.Context, orgID, name, version string) error {
	// the packageID is the orgid/name
	packageID := fmt.Sprintf("%s/%s", orgID, name)

	req := packagepb.GetPackageRequest{
		Id:      packageID,
		Version: version,
	}

	_, err := c.packageClient.GetPackage(ctx.Context, &req)

	if err == nil {
		return fmt.Errorf("a package with name %s and version %s already exists", name, version)
	}
	return nil
}

func sendPackageRequests(stream packagepb.PackageService_CreatePackageClient,
	f *bytes.Buffer, packageInfo *packagepb.PackageInfo,
) error {
	defer utils.UncheckedErrorFunc(stream.CloseSend)

	req := &packagepb.CreatePackageRequest{
		Package: &packagepb.CreatePackageRequest_Info{Info: packageInfo},
	}
	if err := stream.Send(req); err != nil {
		return err
	}

	req = &packagepb.CreatePackageRequest{
		Package: &packagepb.CreatePackageRequest_Contents{Contents: f.Bytes()},
	}

	if err := stream.Send(req); err != nil {
		return err
	}
	return nil
}

// createArchive creates a tar.gz from the file provided.
func createArchive(file *os.File) (*bytes.Buffer, error) {
	// Create output buffer
	out := new(bytes.Buffer)

	// These writers are chained. Writing to the tar writer will
	// write to the gzip writer which in turn will write to
	// the "out" writer
	gw := gzip.NewWriter(out)
	defer utils.UncheckedErrorFunc(gw.Close)
	tw := tar.NewWriter(gw)
	defer utils.UncheckedErrorFunc(tw.Close)

	// Get FileInfo about our file providing file size, mode, etc.
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	// the raw file can be 100 times more than the max TAR size.
	if info.Size() > 100*boardUploadMaximumSize {
		return nil, errors.New("the json file is too large")
	}
	// Create a tar Header from the FileInfo data
	header, err := tar.FileInfoHeader(info, info.Name())
	if err != nil {
		return nil, err
	}

	// Write file header to the tar archive
	err = tw.WriteHeader(header)
	if err != nil {
		return nil, err
	}

	// Read the file into a byte slice
	bytes := make([]byte, info.Size())
	_, err = bufio.NewReader(file).Read(bytes)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}

	// Copy file content to tar archive
	if _, err := tw.Write(bytes); err != nil {
		return nil, err
	}

	return out, nil
}

// helper function to download a url to a local file.
func (c *viamClient) downloadFile(ctx context.Context, filepath, url string) error {
	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)

	// HTTP Requests to app require auth headers, varies by auth mechanism.
	if token, isToken := c.conf.Auth.(*token); isToken {
		getReq.Header.Set("Authorization", "Bearer " + token.AccessToken)
	} else if APIKey, isAPIKey := c.conf.Auth.(*apiKey); isAPIKey {
		getReq.Header.Set("key_id", APIKey.KeyID)
		getReq.Header.Set("key", APIKey.KeyCrypto)
	}

	if err != nil {
		return err
	}
	httpClient := &http.Client{Timeout: time.Second * 30}

	//nolint:bodyclose /// closed in UncheckedErrorFunc
	resp, err := httpClient.Do(getReq)
	if err != nil {
		return errors.Wrap(err, "error downloading the requested package")
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		bodyString := string(bodyBytes)
		return fmt.Errorf("invalid status code %q. Url: %q, Body: %q", resp.Status, url, bodyString)
	}

	defer utils.UncheckedErrorFunc(resp.Body.Close)

	err = untar(filepath, resp.Body)
	if err != nil {
		return errors.Wrap(err, "error extracting the tar file")
	}
	return nil
}

// untar extracts the tar.gz file, keeping file directory structure.
func untar(dst string, r io.Reader) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer utils.UncheckedErrorFunc(gzr.Close)
	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		switch {
		// if no more files are found return
		case errors.Is(err, io.EOF):
			return nil
		// return any other error
		case err != nil:
			return err
		}

		// the target location where the dir/file should be created.
		//nolint: gosec
		path := filepath.Join(dst, header.Name)

		// check the file type.
		switch header.Typeflag {
		// if its a dir and it doesn't exist create it.
		case tar.TypeDir:
			if _, err := os.Stat(path); err != nil {
				if err := os.MkdirAll(path, 0o750); err != nil {
					return err
				}
			}
		// if it's a file create it.
		case tar.TypeReg:
			f, err := os.OpenFile(filepath.Clean(path), os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			// copy over file contents.
			//nolint:gosec
			if _, err := io.Copy(f, tr); err != nil {
				return err
			}

			err = f.Close()
			if err != nil {
				return err
			}
		}
	}
}
