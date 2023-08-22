package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"go.uber.org/multierr"
	packagepb "go.viam.com/api/app/packages/v1"
)

const UploadChunkSize = 32 * 1024

func UploadBoardDefAction(c *cli.Context) error {
	orgIDArg := c.String(boardFlagOrgID)
	nameArg := c.String(boardFlagName)
	versionArg := c.String(boardFlagVersion)
	jsonPath := c.String(boardFlagPath)
	if c.Args().Len() > 1 {
		return errors.New("too many arguments passed to upload command. " +
			"make sure to specify flag and optional arguments before the required positional package argument")
	}
	if jsonPath == "" {
		return errors.New("no package to upload -- please provide a json path containing your board file. use --help for more information")
	}

	// Create tar file from the json file provided.
	out, err := os.Create("output.tar.gz")
	if err != nil {
		log.Fatalln("Error writing archive:", err)
	}
	defer out.Close()

	err = createTar(jsonPath, out)
	if err != nil {
		log.Fatalln("Error creating archive:", err)
	}

	client, err := newAppClient(c)
	if err != nil {
		return err
	}

	response, err := client.uploadBoardDefFile(nameArg, versionArg, orgIDArg, file)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.App.Writer, "version successfully uploaded! you can view your changes online here: %s\n", response.GetUrl())
	return nil
}

func (c *appClient) uploadBoardDefFile(
	name string,
	version string,
	orgID string,
	file *os.File,
) (*packagepb.CreatePackageResponse, error) {
	if err := c.ensureLoggedIn(); err != nil {
		return nil, err
	}
	ctx := c.c.Context

	// setup streaming client for request
	stream, err := c.packageClient.CreatePackage(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "error starting CreatePackage stream")
	}

	packageInfo := &packagepb.PackageInfo{
		OrganizationId: orgID,
		Name:           name,
		Version:        version,
		Type:           packagepb.PackageType_PACKAGE_TYPE_BOARD_DEFS,
		Files:          []*packagepb.FileInfo{},
		Metadata:       nil,
	}

	// close the stream and receive a response when finished
	var errs error
	if err := sendPackageRequests(ctx, stream, file.bytes, &packageInfo); err != nil {
		errs = multierr.Combine(errs, errors.Wrapf(err, "error syncing package"))
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		errs = multierr.Combine(errs, errors.Wrapf(err, "received error response while syncing package"))
	}
	if errs != nil {
		return nil, errs
	}

	return resp, nil
}

func sendPackageRequests(ctx context.Context, stream packagepb.PackageService_CreatePackageClient,
	f *bytes.Buffer, packageInfo *packagepb.PackageInfo,
) error {
	req := &packagepb.CreatePackageRequest{
		Package: &packagepb.CreatePackageRequest_Info{Info: packageInfo},
	}
	// first send the metadata for the package
	if err := stream.Send(req); err != nil {
		return errors.Wrapf(err, "sending metadata")
	}

	//nolint:errcheck
	defer stream.CloseSend()
	// Loop until there is no more content to be read from file.
	for {
		select {
		case <-ctx.Done():
			return context.Canceled
		default:
			// Get the next UploadRequest from the file.
			uploadReq, err := getCreatePackageRequest(ctx, f)

			// EOF means we've completed successfully.
			if errors.Is(err, io.EOF) {
				return nil
			}

			if err != nil {
				return err
			}

			if err = stream.Send(uploadReq); err != nil {
				return err
			}
		}
	}
}

func getCreatePackageRequest(ctx context.Context, f *bytes.Buffer) (*pbPackage.CreatePackageRequest, error) {
	select {
	case <-ctx.Done():
		return nil, context.Canceled
	default:
		// Get the next file data reading from file, check for an error.
		next, err := readNextFileChunk(f)
		if err != nil {
			return nil, err
		}
		// Otherwise, return an UploadRequest and no error.
		return &packagepb.CreatePackageRequest{
			Package: &packagepb.CreatePackageRequest_Contents{
				Contents: next,
			},
		}, nil
	}
}

func readNextFileChunk(f *bytes.Buffer) ([]byte, error) {
	byteArr := make([]byte, UploadChunkSize)
	numBytesRead, err := f.Read(byteArr)
	if numBytesRead < UploadChunkSize {
		byteArr = byteArr[:numBytesRead]
	}
	if err != nil {
		return nil, err
	}
	return byteArr, nil
}

func createTar(filePath string, buf io.Writer) error {
	// Create new Writers for gzip and tar
	// These writers are chained. Writing to the tar writer will
	// write to the gzip writer which in turn will write to
	// the "buf" writer
	gw := gzip.NewWriter(buf)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Get FileInfo about our file providing file size, mode, etc.
	info, err := file.Stat()
	if err != nil {
		return err
	}

	// Create a tar Header from the FileInfo data
	header, err := tar.FileInfoHeader(info, info.Name())
	if err != nil {
		return err
	}

	// Use full path as name (FileInfoHeader only takes the basename)
	// If we don't do this the directory strucuture would
	// not be preserved
	// https://golang.org/src/archive/tar/common.go?#L626
	header.Name = filePath

	// Write file header to the tar archive
	err = tw.WriteHeader(header)
	if err != nil {
		return err
	}

	// Copy file content to tar archive
	_, err = io.Copy(tw, file)
	if err != nil {
		return err
	}
	return nil
}
