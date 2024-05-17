package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"go.uber.org/multierr"
	packagespb "go.viam.com/api/app/packages/v1"
	"go.viam.com/utils"
	"google.golang.org/protobuf/types/known/structpb"
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
	PackageTypeMLTraining  = PackageType("ml_training")
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
	case PackageTypeMLTraining:
		packageTypeProto = packagespb.PackageType_PACKAGE_TYPE_ML_TRAINING
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

// PackageUploadAction is the corresponding action for "packages upload".
func PackageUploadAction(c *cli.Context) error {
	client, err := newViamClient(c)
	if err != nil {
		return err
	}

	_, err = convertPackageTypeToProto(c.String(packageFlagType))
	if err != nil {
		return err
	}

	resp, err := client.uploadPackage(
		c.String(generalFlagOrgID),
		c.String(packageFlagName),
		c.String(packageFlagVersion),
		c.String(packageFlagType),
		c.Path(packageFlagPath),
		nil,
	)
	if err != nil {
		return err
	}

	printf(c.App.Writer, "Successfully uploaded package %s, version: %s!", resp.GetId(), resp.GetVersion())
	return nil
}

func (c *viamClient) uploadPackage(
	orgID, name, version, packageType, tarballPath string,
	metadataStruct *structpb.Struct,
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
	// If version is empty, set to some default
	if version == "" {
		version = fmt.Sprint(time.Now().UnixMilli())
	}
	pkgInfo := packagespb.PackageInfo{
		OrganizationId: orgID,
		Name:           name,
		Version:        version,
		Type:           *packageTypeProto,
		Metadata:       metadataStruct,
	}
	req := &packagespb.CreatePackageRequest{
		Package: &packagespb.CreatePackageRequest_Info{
			Info: &pkgInfo,
		},
	}
	if err := stream.Send(req); err != nil {
		return nil, err
	}

	var errs error
	// We do not add the EOF as an error because all server-side errors trigger an EOF on the stream
	// This results in extra clutter to the error msg
	if err := sendUploadRequests(ctx, nil, stream, file, c.c.App.Writer); err != nil && !errors.Is(err, io.EOF) {
		errs = multierr.Combine(errs, errors.Wrapf(err, "could not upload %s", file.Name()))
	}

	resp, closeErr := stream.CloseAndRecv()
	errs = multierr.Combine(errs, closeErr)
	return resp, errs
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

func (m *moduleID) ToDetailURL(baseURL string, packageType PackageType) string {
	return fmt.Sprintf("https://%s/%s/%s/%s", baseURL, strings.ReplaceAll(string(packageType), "_", "-"), m.prefix, m.name)
}
