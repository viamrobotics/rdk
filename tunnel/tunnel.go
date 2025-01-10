// Package tunnel contains helpers for a traffic tunneling implementation
package tunnel

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"go.viam.com/rdk/logging"
)

// ReaderSenderLoop implements a loop that reads bytes from the reader passed in and sends those bytes
// using sendFunc. The loop will exit for any error received or if the context errors.
func ReaderSenderLoop(
	ctx context.Context,
	r io.Reader,
	sendFunc func(buf []byte) error,
	logger logging.Logger,
) (retErr error) {
	var err error
	defer func() {
		if err != nil {
			// EOF indicates that the connection passed in is not going to receive any more data
			// and is not expecting any more data to be written to it. We should exit from this
			// function and clean up this connection.
			//
			// This is expected and does not indicate an error, so filter it out.
			//
			// TODO: filter out use of closed network connection error
			// if we already explicitly closed the connection already
			if errors.Is(err, io.EOF) {
				logger.CDebug(ctx, "received EOF from local connection")
			} else {
				retErr = fmt.Errorf("reader/sender loop err: %w", err)
			}
		}
		logger.CDebug(ctx, "exiting reader/sender loop")
	}()

	for {
		if ctx.Err() != nil {
			return
		}
		// copying io.Copy's default buffer size
		size := 32 * 1024
		buf := make([]byte, size)
		nr, err := r.Read(buf)
		if err != nil {
			return
		}
		if nr == 0 {
			continue
		}
		if err = sendFunc(buf[:nr]); err != nil {
			return
		}
	}
}

// RecvWriterLoop implements a loop that receives bytes using recvFunc and writes those bytes
// to the writer. The loop will exit for any error received or if the context errors.
func RecvWriterLoop(
	ctx context.Context,
	w io.Writer,
	recvFunc func() ([]byte, error),
	logger logging.Logger,
) (retErr error) {
	var err error
	defer func() {
		if err != nil {
			// EOF indicates that the server is not going to receive any more data
			// and is not expecting any more data to be written to it. We should exit from this
			// function and clean up the connection.
			//
			// This is expected and does not indicate an error, so filter it out.
			if errors.Is(err, io.EOF) {
				logger.CDebug(ctx, "received EOF from remote connection")
			} else {
				retErr = fmt.Errorf("receiver/writer loop err: %w", err)
			}
		}
		logger.CDebug(ctx, "exiting receiver/writer loop")
	}()
	for {
		if ctx.Err() != nil {
			return
		}
		data, err := recvFunc()
		if err != nil {
			return
		}
		in := bytes.NewReader(data)
		_, err = io.Copy(w, in)
		if err != nil {
			return
		}
	}
}
