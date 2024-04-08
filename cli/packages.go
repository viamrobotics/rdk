package cli

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
	packagespb "go.viam.com/api/app/packages/v1"
)

func (c *viamClient) uploadPackage(
	moduleID moduleID,
	version,
	platform string,
	tarballPath string,
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
	pkgInfo := packagespb.PackageInfo{
		OrganizationId: "",
		Name:           "",
		Version:        "",
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
