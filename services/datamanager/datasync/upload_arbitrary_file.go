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

	// TODO: use single error channel for these two
	errChannel := make(chan error, 1)

	activeBackgroundWorkers.Add(1)
	go func() {
		defer activeBackgroundWorkers.Done()
		for {
			recvChannel := make(chan error)
			go func() {
				defer close(recvChannel)
				_, err := stream.Recv()
				if err != nil {
					recvChannel <- err
					return
				}

			}()
			select {
			case <-ctx.Done():
				errChannel <- ctx.Err()
				return
			case e := <-recvChannel:
				errChannel <- e
				return
			}
		}
	}()

	activeBackgroundWorkers.Add(1)
	go func() {
		defer activeBackgroundWorkers.Done()
		// Do not check error of stream close send because it is always following the error check
		// of the wider function execution.
		//nolint: errcheck
		defer stream.CloseSend()
		// Loop until there is no more content to be read from file.
		for {
			select {
			case <-ctx.Done():
				if ctx.Err() != nil {
					errChannel <- ctx.Err()
				}
				return
			default:
				// Get the next UploadRequest from the file.
				uploadReq, err := getNextFileUploadRequest(ctx, f)

				// EOF means we've completed successfully.
				if errors.Is(err, io.EOF) {
					if err := stream.CloseSend(); err != nil {
						errChannel <- errors.Wrap(err, "error when closing the stream")
					}
					return
				}
				if errors.Is(err, datacapture.EmptyReadingErr(filepath.Base(f.Name()))) {
					continue
				}

				if err != nil {
					errChannel <- err
					return
				}

				if err = stream.Send(uploadReq); err != nil {
					errChannel <- err
					return
				}
			}
		}
	}()

	activeBackgroundWorkers.Wait()
	// TODO: close channel here
	close(errChannel)

	// TODO: maybe combine error?
	for err := range errChannel {
		return err
	}

	return nil
}

func getNextFileUploadRequest(ctx context.Context, f *os.File) (*v1.UploadRequest, error) {
	select {
	case <-ctx.Done():
		// TODO: is this right?
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
