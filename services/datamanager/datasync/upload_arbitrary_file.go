package datasync

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/pkg/errors"
	v1 "go.viam.com/api/proto/viam/datasync/v1"

	"go.viam.com/rdk/services/datamanager/datacapture"
)

func uploadArbitraryFile(ctx context.Context, client v1.DataSyncServiceClient, md *v1.UploadMetadata,
	f *os.File,
) error {
	stream, err := client.Upload(ctx)
	if err != nil {
		return err
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

	retRecv := make(chan error, 1)
	activeBackgroundWorkers.Add(1)
	go func() {
		defer activeBackgroundWorkers.Done()
		defer close(retRecv)
		for {
			select {
			case <-ctx.Done():
				retRecv <- ctx.Err()
				return
			default:
				_, err := stream.Recv()
				if err != nil {
					retRecv <- err
					return
				}
			}
		}
	}()

	retSend := make(chan error, 1)
	activeBackgroundWorkers.Add(1)
	go func() {
		defer activeBackgroundWorkers.Done()
		defer close(retSend)
		// Do not check error of stream close send because it is always following the error check
		// of the wider function execution.
		//nolint: errcheck
		defer stream.CloseSend()
		// Loop until there is no more content to be read from file.
		for {
			select {
			case <-ctx.Done():
				if ctx.Err() != nil {
					retSend <- ctx.Err()
				}
				return
			default:
				// Get the next UploadRequest from the file.
				uploadReq, err := getNextFileUploadRequest(ctx, f)

				// EOF means we've completed successfully.
				if errors.Is(err, io.EOF) {
					if err := stream.CloseSend(); err != nil {
						retSend <- errors.Wrap(err, "error when closing the stream")
					}
					return
				}
				if errors.Is(err, datacapture.EmptyReadingErr(filepath.Base(f.Name()))) {
					continue
				}

				if err != nil {
					retSend <- err
					return
				}

				if err = stream.Send(uploadReq); err != nil {
					retSend <- err
					return
				}
			}
		}
	}()

	activeBackgroundWorkers.Wait()

	if err := <-retRecv; err != nil && !errors.Is(err, io.EOF) {
		return errors.Errorf("Error when trying to recv from server: %v", err)
	}
	if err := <-retSend; err != nil {
		return errors.Errorf("Error when trying to send to server: %v", err)
	}

	return nil
}

func getNextFileUploadRequest(ctx context.Context, f *os.File) (*v1.UploadRequest, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
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
