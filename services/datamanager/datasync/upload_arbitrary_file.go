package datasync

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/pkg/errors"
	v1 "go.viam.com/api/app/datasync/v1"
	goutils "go.viam.com/utils"
)

func uploadArbitraryFile(ctx context.Context, client v1.DataSyncServiceClient, partID string,
	f *os.File,
) error {
	stream, err := client.Upload(ctx)
	if err != nil {
		return err
	}

	md := &v1.UploadMetadata{
		PartId:   partID,
		Type:     v1.DataType_DATA_TYPE_FILE,
		FileName: filepath.Base(f.Name()),
	}

	// Send metadata upload request.
	req := &v1.UploadRequest{
		UploadPacket: &v1.UploadRequest_Metadata{
			Metadata: md,
		},
	}
	if err := stream.Send(req); err != nil {
		return err
	}

	var activeBackgroundWorkers sync.WaitGroup
	errChannel := make(chan error, 2)

	// Create cancelCtx so if either the recv or send goroutines error, we can cancel the other one.
	cancelCtx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()

	// Start a goroutine for recving errors back from the server.
	activeBackgroundWorkers.Add(1)
	goutils.PanicCapturingGo(func() {
		defer activeBackgroundWorkers.Done()
		err := recvFileUploadResponses(cancelCtx, stream)
		if err != nil {
			errChannel <- err
			cancelFn()
		}
	})

	// Start a goroutine for sending requests to the server.
	activeBackgroundWorkers.Add(1)
	goutils.PanicCapturingGo(func() {
		defer activeBackgroundWorkers.Done()
		err := sendFileUploadRequests(cancelCtx, stream, f)
		if err != nil {
			errChannel <- err
			cancelFn()
		}
	})

	activeBackgroundWorkers.Wait()
	close(errChannel)

	for err := range errChannel {
		return err
	}

	return nil
}

func getNextFileUploadRequest(ctx context.Context, f *os.File) (*v1.UploadRequest, error) {
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
		return &v1.UploadRequest{
			UploadPacket: &v1.UploadRequest_FileContents{
				FileContents: next,
			},
		}, nil
	}
}

func readNextFileChunk(f *os.File) (*v1.FileData, error) {
	byteArr := make([]byte, uploadChunkSize)
	numBytesRead, err := f.Read(byteArr)
	if numBytesRead < uploadChunkSize {
		byteArr = byteArr[:numBytesRead]
	}
	if err != nil {
		return nil, err
	}
	return &v1.FileData{Data: byteArr}, nil
}

func recvFileUploadResponses(ctx context.Context, stream v1.DataSyncService_UploadClient) error {
	for {
		recvChannel := make(chan error)
		go func() {
			defer close(recvChannel)
			_, err := stream.Recv()
			recvChannel <- err
		}()
		select {
		case <-ctx.Done():
			return context.Canceled
		case e := <-recvChannel:
			if e != nil {
				if !errors.Is(e, io.EOF) {
					return e
				}
				return nil
			}
		}
	}
}

func sendFileUploadRequests(ctx context.Context, stream v1.DataSyncService_UploadClient, f *os.File) error {
	//nolint:errcheck
	defer stream.CloseSend()
	// Loop until there is no more content to be read from file.
	for {
		select {
		case <-ctx.Done():
			return context.Canceled
		default:
			// Get the next UploadRequest from the file.
			uploadReq, err := getNextFileUploadRequest(ctx, f)

			// EOF means we've completed successfully.
			if errors.Is(err, io.EOF) {
				if err := stream.CloseSend(); err != nil {
					return err
				}
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
