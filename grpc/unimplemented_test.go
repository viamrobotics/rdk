package grpc

import (
	"errors"
	"testing"

	"go.viam.com/test"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestIsUnimplementedError(t *testing.T) {
	test.That(t, IsUnimplementedError(nil), test.ShouldBeFalse)
	test.That(t, IsUnimplementedError(UnimplementedError), test.ShouldBeTrue)
	test.That(t, IsUnimplementedError(status.Error(codes.Unimplemented, "no such method")), test.ShouldBeTrue)
	test.That(t, IsUnimplementedError(errors.New("Geometries unimplemented")), test.ShouldBeTrue)
	test.That(t, IsUnimplementedError(errors.New("broken")), test.ShouldBeFalse)
	test.That(t, IsUnimplementedError(status.Error(codes.Internal, "broken")), test.ShouldBeFalse)
}
