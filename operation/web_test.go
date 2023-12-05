package operation

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"go.viam.com/test"
	"google.golang.org/grpc/metadata"

	"go.viam.com/rdk/logging"
)

func TestCreateFromIncomingContextWithoutOpid(t *testing.T) {
	logger := logging.NewTestLogger(t)
	m := NewManager(logger)

	_, done := m.CreateFromIncomingContext(context.Background(), "fake")
	defer done()

	ops := m.All()
	test.That(t, ops, test.ShouldHaveLength, 1)
}

func TestCreateFromIncomingContextWithOpid(t *testing.T) {
	logger := logging.NewTestLogger(t)
	m := NewManager(logger)

	opid := uuid.New()
	meta := metadata.New(map[string]string{opidMetadataKey: opid.String()})
	ctx := metadata.NewIncomingContext(context.Background(), meta)
	_, done := m.CreateFromIncomingContext(ctx, "fake")
	defer done()

	ops := m.All()

	test.That(t, ops, test.ShouldHaveLength, 1)
	test.That(t, ops[0].ID.String(), test.ShouldEqual, opid.String())
}
