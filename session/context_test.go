package session_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.viam.com/test"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/session"
	"go.viam.com/rdk/testutils"
)

func TestToFromContext(t *testing.T) {
	_, ok := session.FromContext(context.Background())
	test.That(t, ok, test.ShouldBeFalse)

	sess1 := session.New(context.Background(), "ownerID", time.Minute, func(id uuid.UUID, resourceName resource.Name) {
	})
	nextCtx := session.ToContext(context.Background(), sess1)
	sess2, ok := session.FromContext(nextCtx)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, sess2, test.ShouldEqual, sess1)
}

func TestSafetyMonitor(t *testing.T) {
	session.SafetyMonitor(context.Background(), nil)
	name := resource.NewName(resource.APINamespace("foo").WithType("bar").WithSubtype("baz"), "barf")
	session.SafetyMonitor(context.Background(), myThing{Named: name.AsNamed()})

	var stored sync.Once
	var storedCount int32
	var storedID uuid.UUID
	var storedResourceName resource.Name
	sess1 := session.New(context.Background(), "ownerID", time.Minute, func(id uuid.UUID, resourceName resource.Name) {
		atomic.AddInt32(&storedCount, 1)
		stored.Do(func() {
			storedID = id
			storedResourceName = resourceName
		})
	})
	nextCtx := session.ToContext(context.Background(), sess1)

	session.SafetyMonitor(nextCtx, myThing{Named: name.AsNamed()})
	test.That(t, storedID, test.ShouldEqual, sess1.ID())
	test.That(t, storedResourceName, test.ShouldResemble, name)
	test.That(t, atomic.LoadInt32(&storedCount), test.ShouldEqual, 1)

	sess1 = session.New(context.Background(), "ownerID", 0, func(id uuid.UUID, resourceName resource.Name) {
		atomic.AddInt32(&storedCount, 1)
		stored.Do(func() {
			storedID = id
			storedResourceName = resourceName
		})
	})
	nextCtx = session.ToContext(context.Background(), sess1)
	session.SafetyMonitor(nextCtx, myThing{Named: name.AsNamed()})
	test.That(t, atomic.LoadInt32(&storedCount), test.ShouldEqual, 1)
}

func TestSafetyMonitorForMetadata(t *testing.T) {
	stream1 := testutils.NewServerTransportStream()
	streamCtx := grpc.NewContextWithServerTransportStream(context.Background(), stream1)

	sess1 := session.New(context.Background(), "ownerID", time.Minute, nil)
	nextCtx := session.ToContext(streamCtx, sess1)

	name1 := resource.NewName(resource.APINamespace("foo").WithType("bar").WithSubtype("baz"), "barf")
	name2 := resource.NewName(resource.APINamespace("woo").WithType("war").WithSubtype("waz"), "warf")
	session.SafetyMonitor(nextCtx, myThing{Named: name1.AsNamed()})
	test.That(t, stream1.Value(session.SafetyMonitoredResourceMetadataKey), test.ShouldResemble, []string{name1.String()})
	session.SafetyMonitor(nextCtx, myThing{Named: name2.AsNamed()})
	test.That(t, stream1.Value(session.SafetyMonitoredResourceMetadataKey), test.ShouldResemble, []string{name2.String()})
}

type myThing struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable
}

type myStream struct {
	mu sync.Mutex
	grpc.ServerTransportStream
	md metadata.MD
}

func (s *myStream) SetHeader(md metadata.MD) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.md = md.Copy()
	return nil
}
