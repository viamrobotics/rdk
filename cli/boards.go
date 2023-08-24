package cli

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"go.uber.org/multierr"
	packagepb "go.viam.com/api/app/packages/v1"
)

// supportedVersionRegex validates that the board version is semver 2.0.0 specification.
var supportedVersionRegex = regexp.MustCompile(`^(?P<major>0|[1-9]\d*)\.(?P<minor>0|[1-9]\d*)\.(?P<patch>0|[1-9]\d*)` +
	`(?:-(?P<prerelease>(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)` +
	`(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+(?P<buildmetadata>[0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)

const boardUploadChunkSize = 32 * 1024

// UploadBoardDefsAction is the corresponding action for "board upload".
func UploadBoardDefsAction(c *cli.Context) error {
	orgArg := c.String(organizationFlag)
	nameArg := c.String(boardFlagName)
	versionArg := c.String(boardFlagVersion)
	jsonPath := c.Args().First()
	if c.Args().Len() > 1 {
		return errors.New("too many arguments passed to upload command. " +
			"make sure to specify flag and optional arguments before the required positional package argument")
	}
	if jsonPath == "" {
		return errors.New("no package to upload -- please provide a path containing your json file. use --help for more information")
	}

	// Validate the version is valid.
	if !supportedVersionRegex.MatchString(versionArg) {
		return fmt.Errorf("invalid version %s. Must use semver 2.0.0 specification for versions", versionArg)
	}

	client, err := newAppClient(c)
	if err != nil {
		return err
	}
	ctx := client.c.Context

	// get the org from the name or id.
	org, err := client.getOrg(orgArg)
	if err != nil {
		return err
	}

	// check if the package already exists.
	err = client.boardDefsPackageExists(ctx, org.Id, nameArg, versionArg)
	if err != nil {
		return err
	}

	jsonFile, err := os.Open(filepath.Clean(jsonPath))
	if err != nil {
		return err
	}

	if !strings.HasSuffix(jsonFile.Name(), ".json") {
		return errors.New("The board definition file must be a .json")
	}

	_, err = client.uploadBoardDefsFile(nameArg, versionArg, org.Id, jsonFile)
	if err != nil {
		return err
	}

	fmt.Fprintf(c.App.Writer, "Board definitions file was successfully uploaded!\n")
	return nil
}

func (c *appClient) uploadBoardDefsFile(
	name string,
	version string,
	orgID string,
	jsonFile *os.File,
) (*packagepb.CreatePackageResponse, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}
	ctx := c.c.Context

	// Create an archive tar.gz file (required for packages).
	file, err := CreateArchive(jsonFile)
	if err != nil {
		return nil, errors.Wrap(err, "error creating archive")
	}

	// The board defs packages are small and never expected to be larger than the upload chunk size,
	// so we are sending in one chunk.
	// If the file is too big, return error.
	if file.Len() > boardUploadChunkSize {
		return nil, fmt.Errorf("file is too large, must be under %d bytes", boardUploadChunkSize)
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

// helper function to check if a package with this name and version already exists.
func (c *appClient) boardDefsPackageExists(ctx context.Context, orgID, name, version string) error {
	// the packageID is the orgid/name
	packageID := fmt.Sprintf("%s/%s", orgID, name)

	req := packagepb.GetPackageRequest{
		Id:      packageID,
		Version: version,
	}

	_, err := c.packageClient.GetPackage(ctx, &req)

	if err == nil {
		return fmt.Errorf("a package with name %s and version %s already exists", name, version)
	}
	return nil
}

func sendPackageRequests(stream packagepb.PackageService_CreatePackageClient,
	f *bytes.Buffer, packageInfo *packagepb.PackageInfo,
) error {
	req := &packagepb.CreatePackageRequest{
		Package: &packagepb.CreatePackageRequest_Info{Info: packageInfo},
	}
	// first send the packageinfo.
	if err := stream.Send(req); err != nil {
		return err
	}

	//nolint:errcheck
	defer stream.CloseSend()

	// Now send the file contents.
	req = &packagepb.CreatePackageRequest{
		Package: &packagepb.CreatePackageRequest_Contents{Contents: f.Bytes()},
	}

	if err := stream.Send(req); err != nil {
		return err
	}
	return nil
}

// CreateArchive creates a tar.gz from the file provided.
func CreateArchive(file *os.File) (*bytes.Buffer, error) {
	// Create output buffer
	out := new(bytes.Buffer)

	// These writers are chained. Writing to the tar writer will
	// write to the gzip writer which in turn will write to
	// the "out" writer
	gw := gzip.NewWriter(out)
	//nolint:errcheck
	defer gw.Close()
	tw := tar.NewWriter(gw)
	//nolint:errcheck
	defer tw.Close()

	// Get FileInfo about our file providing file size, mode, etc.
	info, err := file.Stat()
	if err != nil {
		return nil, err
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
