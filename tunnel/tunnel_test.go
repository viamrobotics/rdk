package tunnel_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/tunnel"
)

func TestReaderSenderLoop(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	t.Run("closed channel", func(t *testing.T) {
		connClosed := make(chan struct{})
		close(connClosed)

		readCt := 0
		injectReader := &inject.ReadWriteCloser{
			ReadFunc: func(p []byte) (n int, err error) {
				readCt++
				return 0, io.EOF
			},
		}
		sendCt := 0
		sendFunc := func(buf []byte) error { sendCt++; return nil }
		err := tunnel.ReaderSenderLoop(ctx, injectReader, sendFunc, connClosed, logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, readCt, test.ShouldEqual, 0)
		test.That(t, sendCt, test.ShouldEqual, 0)
	})

	t.Run("one message", func(t *testing.T) {
		connClosed := make(chan struct{})
		defer close(connClosed)

		readCt := 0
		injectReader := &inject.ReadWriteCloser{
			ReadFunc: func(p []byte) (n int, err error) {
				readCt++
				p[0] = 1
				p[1] = 2
				return 2, io.EOF
			},
		}
		sendCt := 0
		sendFunc := func(buf []byte) error {
			sendCt++
			test.That(t, buf, test.ShouldResemble, []byte{1, 2})
			return nil
		}
		err := tunnel.ReaderSenderLoop(ctx, injectReader, sendFunc, connClosed, logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, readCt, test.ShouldEqual, 1)
		test.That(t, sendCt, test.ShouldEqual, 1)
	})

	t.Run("two messages", func(t *testing.T) {
		connClosed := make(chan struct{})
		defer close(connClosed)

		readCt := 0
		injectReader := &inject.ReadWriteCloser{
			ReadFunc: func(p []byte) (n int, err error) {
				readCt++
				if readCt == 1 {
					p[0] = 1
					p[1] = 2
					return 2, nil
				}
				return 0, io.EOF
			},
		}
		sendCt := 0
		sendFunc := func(buf []byte) error {
			sendCt++
			test.That(t, buf, test.ShouldResemble, []byte{1, 2})
			return nil
		}
		err := tunnel.ReaderSenderLoop(ctx, injectReader, sendFunc, connClosed, logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, readCt, test.ShouldEqual, 2)
		test.That(t, sendCt, test.ShouldEqual, 1)
	})

	//nolint:dupl
	t.Run("one message with read err", func(t *testing.T) {
		connClosed := make(chan struct{})
		defer close(connClosed)

		newErr := errors.New("oops")
		readCt := 0
		injectReader := &inject.ReadWriteCloser{
			ReadFunc: func(p []byte) (n int, err error) {
				readCt++
				p[0] = 1
				p[1] = 2
				return 2, newErr
			},
		}
		sendCt := 0
		sendFunc := func(buf []byte) error {
			sendCt++
			test.That(t, buf, test.ShouldResemble, []byte{1, 2})
			return nil
		}
		err := tunnel.ReaderSenderLoop(ctx, injectReader, sendFunc, connClosed, logger)
		test.That(t, errors.Is(err, newErr), test.ShouldBeTrue)
		test.That(t, readCt, test.ShouldEqual, 1)
		test.That(t, sendCt, test.ShouldEqual, 1)
	})

	//nolint:dupl
	t.Run("one message with send err", func(t *testing.T) {
		connClosed := make(chan struct{})
		defer close(connClosed)

		newErr := errors.New("oops")
		readCt := 0
		injectReader := &inject.ReadWriteCloser{
			ReadFunc: func(p []byte) (n int, err error) {
				readCt++
				p[0] = 1
				p[1] = 2
				return 2, nil
			},
		}
		sendCt := 0
		sendFunc := func(buf []byte) error {
			sendCt++
			test.That(t, buf, test.ShouldResemble, []byte{1, 2})
			return newErr
		}
		err := tunnel.ReaderSenderLoop(ctx, injectReader, sendFunc, connClosed, logger)
		test.That(t, errors.Is(err, newErr), test.ShouldBeTrue)
		test.That(t, readCt, test.ShouldEqual, 1)
		test.That(t, sendCt, test.ShouldEqual, 1)
	})
}

func TestRecvWriterLoop(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	t.Run("cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		rsDone := make(chan struct{})
		defer close(rsDone)

		recvCt := 0
		recvFunc := func() ([]byte, error) {
			recvCt++
			if recvCt == 1 {
				return []byte{1, 2}, nil
			}
			return nil, io.EOF
		}
		writeCt := 0
		injectWriter := &inject.ReadWriteCloser{
			WriteFunc: func(p []byte) (n int, err error) {
				writeCt++
				return 0, nil
			},
		}
		err := tunnel.RecvWriterLoop(ctx, recvFunc, injectWriter, rsDone, logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, recvCt, test.ShouldEqual, 0)
		test.That(t, writeCt, test.ShouldEqual, 0)
	})

	t.Run("closed channel", func(t *testing.T) {
		rsDone := make(chan struct{})
		close(rsDone)

		recvCt := 0
		recvFunc := func() ([]byte, error) {
			recvCt++
			if recvCt == 1 {
				return []byte{1, 2}, nil
			}
			return nil, io.EOF
		}
		writeCt := 0
		injectWriter := &inject.ReadWriteCloser{
			WriteFunc: func(p []byte) (n int, err error) {
				writeCt++
				return 0, nil
			},
		}
		err := tunnel.RecvWriterLoop(ctx, recvFunc, injectWriter, rsDone, logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, recvCt, test.ShouldEqual, 2)
		test.That(t, writeCt, test.ShouldEqual, 0)
	})

	t.Run("one message", func(t *testing.T) {
		rsDone := make(chan struct{})
		defer close(rsDone)

		recvCt := 0
		recvFunc := func() ([]byte, error) {
			recvCt++
			if recvCt == 1 {
				return []byte{1, 2}, nil
			}
			return nil, io.EOF
		}
		writeCt := 0
		injectWriter := &inject.ReadWriteCloser{
			WriteFunc: func(p []byte) (n int, err error) {
				writeCt++
				return 0, nil
			},
		}
		err := tunnel.RecvWriterLoop(ctx, recvFunc, injectWriter, rsDone, logger)
		test.That(t, err, test.ShouldBeNil)
		// there is a second call to recvFunc because recvFunc should be called until it errors
		test.That(t, recvCt, test.ShouldEqual, 2)
		test.That(t, writeCt, test.ShouldEqual, 1)
	})

	t.Run("one message with recv err", func(t *testing.T) {
		rsDone := make(chan struct{})
		defer close(rsDone)

		newErr := errors.New("oops")
		recvCt := 0
		recvFunc := func() ([]byte, error) {
			recvCt++
			return nil, newErr
		}
		writeCt := 0
		injectWriter := &inject.ReadWriteCloser{
			WriteFunc: func(p []byte) (n int, err error) {
				writeCt++
				return 0, nil
			},
		}
		err := tunnel.RecvWriterLoop(ctx, recvFunc, injectWriter, rsDone, logger)
		test.That(t, errors.Is(err, newErr), test.ShouldBeTrue)
		test.That(t, recvCt, test.ShouldEqual, 1)
		test.That(t, writeCt, test.ShouldEqual, 0)
	})

	t.Run("two messages with write err", func(t *testing.T) {
		rsDone := make(chan struct{})
		defer close(rsDone)

		newErr := errors.New("oops")
		recvCt := 0
		recvFunc := func() ([]byte, error) {
			recvCt++
			if recvCt < 3 {
				return []byte{1, 2}, nil
			}
			return nil, io.EOF
		}
		writeCt := 0
		injectWriter := &inject.ReadWriteCloser{
			WriteFunc: func(p []byte) (n int, err error) {
				writeCt++
				return 0, newErr
			},
		}
		err := tunnel.RecvWriterLoop(ctx, recvFunc, injectWriter, rsDone, logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, recvCt, test.ShouldEqual, 3)
		// write errors are ignored, so Write should be called as many times as there
		// are actual messages.
		test.That(t, writeCt, test.ShouldEqual, 2)
	})
}
