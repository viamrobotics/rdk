// Package tunnel contains helpers for a traffic tunneling implementation
package tunnel

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"

	"go.viam.com/rdk/logging"
)

func filterError(ctx context.Context, err error, closeChan <-chan struct{}, logger logging.Logger) error {
	// if connection is expected to be closed, filter out "use of closed network connection" errors
	select {
	case <-closeChan:
		if errors.Is(err, net.ErrClosed) {
			logger.CDebugw(ctx, "expected error received", "err", err)
			return nil
		}
	default:
	}

	// EOF indicates that the connection passed in is not going to receive any more data
	// and is not expecting any more data to be written to it.
	//
	// This is expected and does not indicate an error, so filter it out.
	if errors.Is(err, io.EOF) {
		logger.CDebugw(ctx, "expected EOF received")
		return nil
	}
	return err
}

// ReaderSenderLoop implements a loop that reads bytes from the reader passed in and sends those bytes
// using sendFunc. The loop will exit for any error received or if the context errors.
func ReaderSenderLoop(
	ctx context.Context,
	r io.Reader,
	sendFunc func(buf []byte) error,
	connClosed <-chan struct{},
	logger logging.Logger,
) (retErr error) {
	var err, sendErr error
	defer func() {
		retErr = filterError(ctx, errors.Join(err, sendErr), connClosed, logger)
		if retErr != nil {
			retErr = fmt.Errorf("reader/sender loop err: %w", retErr)
		}
		logger.CDebug(ctx, "exiting reader/sender loop")
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-connClosed:
			return
		default:
		}
		// copying io.Copy's default buffer size (32kb)
		size := 32 * 1024
		buf := make([]byte, size)
		var nr int
		nr, err = r.Read(buf)
		// based on [io.Reader], callers should always process the n > 0 bytes returned before
		// considering the error
		if nr > 0 {
			if sendErr = sendFunc(buf[:nr]); sendErr != nil {
				return
			}
		}
		if err != nil {
			return
		}
	}
}

// RecvWriterLoop implements a loop that receives bytes using recvFunc and writes those bytes
// to the writer. The loop will exit for any error received or if the context errors.
func RecvWriterLoop(
	ctx context.Context,
	recvFunc func() ([]byte, error),
	w io.Writer,
	rsDone <-chan struct{},
	logger logging.Logger,
) (retErr error) {
	var err error
	defer func() {
		retErr = filterError(ctx, err, rsDone, logger)
		if retErr != nil {
			retErr = fmt.Errorf("receiver/writer loop err: %w", retErr)
		}
		logger.CDebug(ctx, "exiting receiver/writer loop")
	}()
	for {
		if ctx.Err() != nil {
			return
		}
		var data []byte
		data, err = recvFunc()
		if err != nil {
			return
		}
		// For bidi streaming, Recv should be called on the client/server until it errors.
		// See [grpc.NewStream] for related docs.
		//
		// If the reader/sender loop is done, stop copying bytes as that means the connection is no longer accepting bytes.
		select {
		case <-rsDone:
			continue
		default:
		}
		in := bytes.NewReader(data)
		_, err = io.Copy(w, in)
		if err != nil {
			logger.CDebugw(ctx, "error while copying bytes", "err", err)
			continue
		}
	}
}
