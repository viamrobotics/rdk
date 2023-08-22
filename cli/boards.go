package cli

import (
	"io"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"go.uber.org/multierr"
	packagepb "go.viam.com/api/app/packages/v1"
	apppb "go.viam.com/api/app/v1"
)

func UploadBoardDefAction(c *cli.Context) error {
	orgIDArg := c.String(boardFlagOrgID)
	nameArg := c.String(boardFlagName)
	versionArg := c.String(boardFlagVersion)
	tarballPath := c.Args().First()
	if c.Args().Len() > 1 {
		return errors.New("too many arguments passed to upload command. " +
			"make sure to specify flag and optional arguments before the required positional package argument")
	}
	if tarballPath == "" {
		return errors.New("no package to upload -- please provide an archive containing your board file. use --help for more information")
	}

	client, err := newAppClient(c)
	if err != nil {
		return err
	}

	//nolint:gosec
	file, err := os.Open(tarballPath)
	if err != nil {
		return err
	}
	// TODO(APP-2226): support .tar.xz
	if !strings.HasSuffix(file.Name(), ".tar.gz") {
		return errors.New("you must upload your module in the form of a .tar.gz")
	}
	response, err := client.uploadBoardDefFile(nameArg, versionArg, orgIDArg, file)
	if err != nil {
		return err
	}
	return nil
}

func (c *appClient) uploadBoardDefFile(
	name string,
	version string,
	orgID string,
	file *os.File,
) (*apppb.UploadModuleFileResponse, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}
	ctx := c.c.Context

	req := &packagepb.CreatePackageRequest{
		Package: &packagepb.CreatePackageRequest_Contents{},
	}

	packageInfo := &packagepb.PackageInfo{
		OrganizationId: orgID,
		Name:           name,
		Version:        version,
		Type:           packagepb.PackageType_PACKAGE_TYPE_BOARD_DEFS,
		Files:          []*packagepb.FileInfo{},
		Metadata:       nil,
	}

	var errs error
	// We do not add the EOF as an error because all server-side errors trigger an EOF on the stream
	// This results in extra clutter to the error msg
	if err := sendModuleUploadRequests(ctx, stream, file, c.c.App.Writer); err != nil && !errors.Is(err, io.EOF) {
		errs = multierr.Combine(errs, errors.Wrapf(err, "could not upload %s", file.Name()))
	}

	//resp, closeErr := stream.CloseAndRecv()
	//errs = multierr.Combine(errs, closeErr)
	return resp, errs
}
