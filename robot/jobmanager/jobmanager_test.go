package jobmanager

import (
	"context"
	"net"
	"sync/atomic"
	"testing"

	"github.com/viamrobotics/webrtc/v3"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/sensor/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

// The stream grpcreflect.NewClientV1Alpha opens; leaked one-per-invocation without Reset.
const reflectionInfoMethod = "/grpc.reflection.v1alpha.ServerReflection/ServerReflectionInfo"

// streamCounter tracks the high-water mark of concurrently-open reflection streams.
type streamCounter struct {
	current       atomic.Int64
	maxConcurrent atomic.Int64
}

func (sc *streamCounter) intercept(
	srv any,
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	if info.FullMethod != reflectionInfoMethod {
		return handler(srv, ss)
	}
	cur := sc.current.Add(1)
	for {
		observed := sc.maxConcurrent.Load()
		if cur <= observed || sc.maxConcurrent.CompareAndSwap(observed, cur) {
			break
		}
	}
	defer sc.current.Add(-1)
	return handler(srv, ss)
}

// sensorServer lets reflection resolve the service so GetReadings runs the real job code path.
type sensorServer struct {
	pb.UnimplementedSensorServiceServer
}

func (sensorServer) GetReadings(context.Context, *commonpb.GetReadingsRequest) (*commonpb.GetReadingsResponse, error) {
	return &commonpb.GetReadingsResponse{}, nil
}

// grpcConn adapts *grpc.ClientConn to the rpc.ClientConn the JobManager expects.
type grpcConn struct {
	*grpc.ClientConn
}

func (grpcConn) PeerConn() *webrtc.PeerConnection { return nil }

// TestReflectionStreamNotLeaked guards APP-16939: each job invocation opens a reflection stream,
// and without refClient.Reset() they pile up on the shared conn until it hits MaxConcurrentStreams
// and jobs stop running. Reverting the Reset makes maxConcurrent climb to `iterations`.
func TestReflectionStreamNotLeaked(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	counter := &streamCounter{}
	server := grpc.NewServer(grpc.StreamInterceptor(counter.intercept))
	pb.RegisterSensorServiceServer(server, sensorServer{})
	reflection.Register(server)

	lis, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	go func() { utils.UncheckedError(server.Serve(lis)) }()
	defer server.Stop()

	rawConn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	test.That(t, err, test.ShouldBeNil)
	defer func() { utils.UncheckedError(rawConn.Close()) }()

	injectSensor := inject.NewSensor("sensor")

	jm := &JobManager{
		logger:      logger.Sublogger("job_manager"),
		getResource: func(string) (resource.Resource, error) { return injectSensor, nil },
		ctx:         ctx,
		conn:        grpcConn{rawConn},
	}

	jc := config.JobConfig{
		JobConfigData: config.JobConfigData{
			Name:     "leak-check",
			Schedule: "continuous",
			Resource: "sensor",
			Method:   "GetReadings",
		},
	}

	// non-continuous: each call is one invocation, as the scheduler fires a duration/cron job.
	runJob := jm.createJobFunction(jc, false)

	const iterations = 25
	for i := 0; i < iterations; i++ {
		test.That(t, runJob(ctx), test.ShouldBeNil)
	}

	test.That(t, counter.maxConcurrent.Load(), test.ShouldBeLessThanOrEqualTo, int64(1))
}
