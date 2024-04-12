package cli

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"go.uber.org/multierr"
	packagespb "go.viam.com/api/app/packages/v1"
	"go.viam.com/utils"
)

var boolTrue = true

const bufferSize = 512

// PackageType refers to the type of the package.
type PackageType string

// PackageType enumeration.
const (
	PackageTypeUnspecified = PackageType("unspecified")
	PackageTypeArchive     = PackageType("archive")
	PackageTypeMLModel     = PackageType("ml_model")
	PackageTypeModule      = PackageType("module")
	PackageTypeSLAMMap     = PackageType("slam_map")
)

var packageTypes = []string{
	string(PackageTypeUnspecified), string(PackageTypeArchive), string(PackageTypeMLModel),
	string(PackageTypeModule), string(PackageTypeSLAMMap),
}

// PackageExportAction is the corresponding action for 'package export'.
func PackageExportAction(c *cli.Context) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	return client.packageExportAction(
		c.String(generalFlagOrgID),
		c.String(packageFlagName),
		c.String(packageFlagVersion),
		c.String(packageFlagType),
		c.Path(packageFlagDestination),
	)
}

func convertPackageTypeToProto(packageType string) (*packagespb.PackageType, error) {
	// Convert PackageType to proto
	var packageTypeProto packagespb.PackageType
	switch PackageType(packageType) {
	case PackageTypeUnspecified:
		packageTypeProto = packagespb.PackageType_PACKAGE_TYPE_UNSPECIFIED
	case PackageTypeArchive:
		packageTypeProto = packagespb.PackageType_PACKAGE_TYPE_ARCHIVE
	case PackageTypeMLModel:
		packageTypeProto = packagespb.PackageType_PACKAGE_TYPE_ML_MODEL
	case PackageTypeModule:
		packageTypeProto = packagespb.PackageType_PACKAGE_TYPE_MODULE
	case PackageTypeSLAMMap:
		packageTypeProto = packagespb.PackageType_PACKAGE_TYPE_SLAM_MAP
	default:
		return nil, errors.New("invalid package type " + packageType)
	}
	return &packageTypeProto, nil
}

func (c *viamClient) packageExportAction(orgID, name, version, packageType, destination string) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}
	// Package ID is the <organization-ID>/<package-name>
	packageID := path.Join(orgID, name)
	packageTypeProto, err := convertPackageTypeToProto(packageType)
	if err != nil {
		return err
	}
	resp, err := c.packageClient.GetPackage(c.c.Context,
		&packagespb.GetPackageRequest{
			Id:         packageID,
			Version:    version,
			Type:       packageTypeProto,
			IncludeUrl: &boolTrue,
		},
	)
	if err != nil {
		return err
	}

	return downloadPackageFromURL(c.c.Context, c.authFlow.httpClient, destination, name, version, resp.GetPackage().GetUrl())
}

func downloadPackageFromURL(ctx context.Context, httpClient *http.Client,
	destination, name, version, packageURL string,
) error {
	// All packages are stored as .tar.gz
	packagePath := filepath.Join(destination, version, name+".tar.gz")
	if err := os.MkdirAll(filepath.Dir(packagePath), 0o700); err != nil {
		return err
	}
	//nolint:gosec
	packageFile, err := os.Create(packagePath)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, packageURL, nil)
	if err != nil {
		return errors.Wrapf(err, serverErrorMessage)
	}
	res, err := httpClient.Do(req)
	if err != nil {
		return errors.Wrapf(err, serverErrorMessage)
	}
	if res.StatusCode != http.StatusOK {
		return errors.New(serverErrorMessage)
	}
	defer func() {
		utils.UncheckedError(res.Body.Close())
	}()

	for {
		_, err := io.CopyN(packageFile, res.Body, bufferSize)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
	}

	return nil
}

// PackageUploadAction is the corresponding action for 'package upload'.
func PackageUploadAction(c *cli.Context) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	// If draft is set, cannot set visibility; automatically set to private

	return client.packageExportAction(
		c.String(generalFlagOrgID),
		c.String(packageFlagName),
		c.String(packageFlagVersion),
		c.String(packageFlagType),
		c.Path(packageFlagDestination),
	)
}

func (c *viamClient) uploadPackage(
	orgID, name, version, packageType, tarballPath string,
) (*packagespb.CreatePackageResponse, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}

	//nolint:gosec
	file, err := os.Open(tarballPath)
	if err != nil {
		return nil, err
	}
	ctx := c.c.Context

	stream, err := c.packageClient.CreatePackage(ctx)
	if err != nil {
		return nil, err
	}
	packageTypeProto, err := convertPackageTypeToProto(packageType)
	if err != nil {
		return nil, err
	}
	pkgInfo := packagespb.PackageInfo{
		OrganizationId: orgID,
		Name:           name,
		Version:        version,
		Type:           *packageTypeProto,
		// TODO: parse metadata
	}
	req := &packagespb.CreatePackageRequest{
		Package: &packagespb.CreatePackageRequest_Info{
			Info: &pkgInfo},
	}
	if err := stream.Send(req); err != nil {
		return nil, err
	}

	var errs error
	// We do not add the EOF as an error because all server-side errors trigger an EOF on the stream
	// This results in extra clutter to the error msg
	if err := sendPackageUploadRequests(ctx, stream, file, c.c.App.Writer); err != nil && !errors.Is(err, io.EOF) {
		errs = multierr.Combine(errs, errors.Wrapf(err, "could not upload %s", file.Name()))
	}

	resp, closeErr := stream.CloseAndRecv()
	errs = multierr.Combine(errs, closeErr)
	return resp, errs

}

func sendPackageUploadRequests(ctx context.Context, stream packagespb.PackageService_CreatePackageClient,
	file *os.File, stdout io.Writer) error {
	stat, err := file.Stat()
	if err != nil {
		return err
	}
	fileSize := stat.Size()
	uploadedBytes := 0
	// Close the line with the progress reading
	defer printf(stdout, "")

	//nolint:errcheck
	defer stream.CloseSend()
	// Loop until there is no more content to be read from file or the context expires.
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Get the next UploadRequest from the file.
		uploadReq, err := getNextPackageUploadRequest(file)

		// EOF means we've completed successfully.
		if errors.Is(err, io.EOF) {
			return nil
		}

		if err != nil {
			return errors.Wrap(err, "could not read file")
		}

		if err = stream.Send(uploadReq); err != nil {
			return err
		}
		uploadedBytes += len(uploadReq.GetContents())
		// Simple progress reading until we have a proper tui library
		uploadPercent := int(math.Ceil(100 * float64(uploadedBytes) / float64(fileSize)))
		fmt.Fprintf(stdout, "\rUploading... %d%% (%d/%d bytes)", uploadPercent, uploadedBytes, fileSize) // no newline
	}
}

func getNextPackageUploadRequest(file *os.File) (*packagespb.CreatePackageRequest, error) {
	// get the next chunk of bytes from the file
	byteArr := make([]byte, moduleUploadChunkSize)
	numBytesRead, err := file.Read(byteArr)
	if err != nil {
		return nil, err
	}
	if numBytesRead < moduleUploadChunkSize {
		byteArr = byteArr[:numBytesRead]
	}
	return &packagespb.CreatePackageRequest{
		Package: &packagespb.CreatePackageRequest_Contents{
			Contents: byteArr,
		},
	}, nil
}
