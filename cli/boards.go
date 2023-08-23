package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"

	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
	"go.uber.org/multierr"
	packagepb "go.viam.com/api/app/packages/v1"
)

const UploadChunkSize = 32 * 1024

func UploadBoardDefsAction(c *cli.Context) error {
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

	jsonFile, err := os.Open(jsonPath)

	if err != nil {
		return err
	}

	stats, err := json.Stat()
	if err != nil {
		return err
	}

	size := stats.Size()

	file, err := CreateArchive(json)
	if err != nil {
		log.Fatalln("Error creating archive:", err)
	}

	client, err := newAppClient(c)
	if err != nil {
		return err
	}

	response, err := client.uploadBoardDefsFile(nameArg, versionArg, orgIDArg, jsonFile)
	if err != nil {
		return err
	}

	fmt.Printf(response.Id)
	fmt.Printf(response.Version)

	fmt.Printf("Board definitions file was successfully uploaded!")
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

	stats, err := json.Stat()
	if err != nil {
		return err
	}

	size := stats.Size()

	file, err := CreateArchive(json)
	if err != nil {
		log.Fatalln("Error creating archive:", err)
	}

	//setup streaming client for request

	stream, err := c.packageClient.CreatePackage(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "error starting CreatePackage stream")
	}

	boardDefsFile := []*packagepb.FileInfo{{Name: name, Size: uint64(size)}}

	packageInfo := &packagepb.PackageInfo{
		OrganizationId: orgID,
		Name:           name,
		Version:        version,
		Type:           packagepb.PackageType_PACKAGE_TYPE_BOARD_DEFS,
		Files:          boardDefsFile,
		Metadata:       nil,
	}

	// send the package request!!!!
	var errs error
	if err := sendPackageRequests(ctx, stream, file, packageInfo); err != nil {
		errs = multierr.Combine(errs, errors.Wrapf(err, "error syncing package"))
	}

	// close the stream and recievea respoinse when finished.
	resp, err := stream.CloseAndRecv()
	if err != nil {
		errs = multierr.Combine(errs, errors.Wrapf(err, "received error response while syncing package"))
	}
	if errs != nil {
		return nil, errs
	}

	includeURL := true
	packageType := packagepb.PackageType_PACKAGE_TYPE_BOARD_DEFS

	fmt.Println(c.packageClient)

	response, err := c.packageClient.GetPackage(ctx, &packagepb.GetPackageRequest{
		Id:         resp.Id,
		Version:    resp.Version,
		Type:       &packageType,
		IncludeUrl: &includeURL,
	})

	if err != nil {
		fmt.Println(err)
	}

	fmt.Println(response.Package.Url)

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

func getCreatePackageRequest(ctx context.Context, f *bytes.Buffer) (*packagepb.CreatePackageRequest, error) {
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

// CreateArchive creates a tar.gz with the desired set of files.
func CreateArchive(file *os.File) (*bytes.Buffer, error) {
	// Create output buffer
	out := new(bytes.Buffer)

	// Create the archive and write the output to the "out" Writer
	// Create new Writers for gzip and tar
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

	fileContents, err := ioutil.ReadFile(file.Name()) //read the content of file
	if err != nil {
		return nil, err
	}

	// Copy file content to tar archive
	if _, err := tw.Write(fileContents); err != nil {
		return nil, err
	}

	return out, nil
}
