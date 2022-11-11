package session

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
)

func TestToFromContext(t *testing.T) {
	_, ok := FromContext(context.Background())
	test.That(t, ok, test.ShouldBeFalse)

	sess1 := New("ownerID", nil, time.Minute, func(id uuid.UUID, resourceName resource.Name) {
	})
	nextCtx := ToContext(context.Background(), sess1)
	sess2, ok := FromContext(nextCtx)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, sess2, test.ShouldEqual, sess1)
}

func TestSafetyMonitor(t *testing.T) {
	SafetyMonitor(context.Background(), nil)
	SafetyMonitor(context.Background(), 1)
	SafetyMonitor(context.Background(), myThing{})

	name := resource.NewName("foo", "bar", "baz", "barf")
	SafetyMonitor(context.Background(), myThing{name: name})

	var stored sync.Once
	var storedCount int32
	var storedID uuid.UUID
	var storedResourceName resource.Name
	sess1 := New("ownerID", nil, time.Minute, func(id uuid.UUID, resourceName resource.Name) {
		atomic.AddInt32(&storedCount, 1)
		stored.Do(func() {
			storedID = id
			storedResourceName = resourceName
		})
	})
	nextCtx := ToContext(context.Background(), sess1)

	SafetyMonitor(nextCtx, myThing{name: name})
	test.That(t, storedID, test.ShouldEqual, sess1.ID())
	test.That(t, storedResourceName, test.ShouldResemble, name)
	test.That(t, atomic.LoadInt32(&storedCount), test.ShouldEqual, 1)

	sess1 = New("ownerID", nil, 0, func(id uuid.UUID, resourceName resource.Name) {
		atomic.AddInt32(&storedCount, 1)
		stored.Do(func() {
			storedID = id
			storedResourceName = resourceName
		})
	})
	nextCtx = ToContext(context.Background(), sess1)
	SafetyMonitor(nextCtx, myThing{name: name})
	test.That(t, atomic.LoadInt32(&storedCount), test.ShouldEqual, 1)
}

func TestSafetyMonitorForMetadata(t *testing.T) {
	stream1 := &myStream{}
	streamCtx := grpc.NewContextWithServerTransportStream(context.Background(), stream1)

	sess1 := New("ownerID", nil, time.Minute, nil)
	nextCtx := ToContext(streamCtx, sess1)

	name1 := resource.NewName("foo", "bar", "baz", "barf")
	name2 := resource.NewName("woo", "war", "waz", "warf")
	SafetyMonitor(nextCtx, myThing{name: name1})
	test.That(t, stream1.md[SafetyMonitoredResourceMetadataKey], test.ShouldResemble, []string{name1.String()})
	SafetyMonitor(nextCtx, myThing{name: name2})
	test.That(t, stream1.md[SafetyMonitoredResourceMetadataKey], test.ShouldResemble, []string{name2.String()})
}

type myThing struct {
	resource.Reconfigurable
	name resource.Name
}

func (m myThing) Name() resource.Name {
	return m.name
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
