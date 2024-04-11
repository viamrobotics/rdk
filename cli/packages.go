package cli

import (
	"context"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
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

func (c *viamClient) packageExportAction(orgID, name, version, packageType, destination string) error {
	if err := c.ensureLoggedIn(); err != nil {
		return err
	}
	// Package ID is the <organization-ID>/<package-name>
	packageID := path.Join(orgID, name)

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
		return errors.New("invalid package type " + packageType)
	}

	resp, err := c.packageClient.GetPackage(c.c.Context,
		&packagespb.GetPackageRequest{
			Id:         packageID,
			Version:    version,
			Type:       &packageTypeProto,
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
