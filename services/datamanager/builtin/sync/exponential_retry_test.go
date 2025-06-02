package sync

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func TestTerminalError(t *testing.T) {
	type testCase struct {
		err        error
		isTerminal bool
	}

	tcs := []testCase{
		{err: proto.Error, isTerminal: true},
		{err: status.Error(codes.InvalidArgument, ""), isTerminal: true},
		{err: status.Error(codes.Canceled, "")},
		{err: status.Error(codes.Unknown, "")},
		{err: status.Error(codes.DeadlineExceeded, "")},
		{err: status.Error(codes.NotFound, "")},
		{err: status.Error(codes.AlreadyExists, "")},
		{err: status.Error(codes.PermissionDenied, "")},
		{err: status.Error(codes.ResourceExhausted, "")},
		{err: status.Error(codes.FailedPrecondition, "")},
		{err: status.Error(codes.Aborted, "")},
		{err: status.Error(codes.OutOfRange, "")},
		{err: status.Error(codes.Unimplemented, "")},
		{err: status.Error(codes.Internal, "")},
		{err: status.Error(codes.Unavailable, "")},
		{err: status.Error(codes.DataLoss, "")},
		{err: status.Error(codes.Unauthenticated, "")},

		{err: &fs.PathError{Op: "some op", Path: "some path", Err: errors.New("some error")}},
		{err: context.Canceled},
		{err: io.EOF},
		{err: io.ErrUnexpectedEOF},

		{err: os.ErrInvalid},
		{err: os.ErrPermission},
		{err: os.ErrExist},
		{err: os.ErrNotExist},
		{err: os.ErrClosed},
		{err: os.ErrDeadlineExceeded},
		{err: os.ErrNoDeadline},

		{err: errors.New("some new anonymous error")},
		{err: fmt.Errorf("some new anonymous %s", "error")},
	}

	for _, tc := range tcs {
		// Test that a wrapped terminal error still gets classified as a terminal error
		test.That(t, terminalError(tc.err), test.ShouldEqual, tc.isTerminal)
		// confirm wrapping the error doesn't change the result
		test.That(t, terminalError(errors.Wrap(tc.err, "some context")), test.ShouldEqual, tc.isTerminal)
		test.That(t, terminalError(errors.Wrapf(tc.err, "some context %s", "and more context")), test.ShouldEqual, tc.isTerminal)
	}
}
