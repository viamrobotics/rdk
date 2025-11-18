package web

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	rutils "go.viam.com/rdk/utils"
	"go.viam.com/rdk/utils/ssync"
)

// ReqLimitExceededURL is the URL for the troubleshooting steps for request limit exceeded errors.
const ReqLimitExceededURL = "https://docs.viam.com/dev/tools/common-errors/#req-limit-exceeded"

// RequestLimitExceededError is an error returned when a request is rejected
// because it would exceed the limit for concurrent requests to a given
// resource.
type RequestLimitExceededError struct {
	resource string
	limit    int64
}

func (e RequestLimitExceededError) Error() string {
	return fmt.Sprintf("exceeded request limit %v on resource %v, see %v for troubleshooting steps",
		e.limit, e.resource, ReqLimitExceededURL)
}

// GRPCStatus allows this error to be converted to a [status.Status].
func (e RequestLimitExceededError) GRPCStatus() *status.Status {
	return status.New(codes.ResourceExhausted, e.Error())
}

// apiMethod is used to store information about an api path in a denormalized
// way to avoid repeatedly parsing the same string.
type apiMethod struct {
	full      string
	service   string
	name      string
	shortPath string
}

func (m apiMethod) getResourceName(msg any) string {
	return resource.GetResourceNameFromRequest(m.service, m.name, msg)
}

type requestStats struct {
	count     atomic.Int64
	errorCnt  atomic.Int64
	timeSpent atomic.Int64
	dataSent  atomic.Int64
}

// RequestCounter is used to track and limit incoming requests. It instruments
// every unary and streaming request coming in from both external clients and
// internal modules.
type RequestCounter struct {
	logger logging.Logger

	// requestKeyToStats maps individual API calls for each resource to a set of
	// metrics. E.g: `motor-foo.IsPowered` and `motor-foo.GoFor` would each have
	// their own set of stats.
	requestKeyToStats ssync.Map[string, *requestStats]

	// inFlightRequests maps resource names to how many in flight requests are
	// currently targeting that resource name. There can only be `limit` API
	// calls for any resource. E.g: `motor-foo` can have 50 `IsPowered`
	// concurrent calls with 50 more `GoFor` calls, or instead 100 `IsPowered`
	// calls before it starts to reject new incoming requests. Unary and
	// streaming RPCs both count against the limit.`limit` defaults to 100 but
	// can be configured with the `VIAM_RESOURCE_REQUESTS_LIMIT`
	// environment variable.
	inFlightRequests ssync.Map[string, *atomic.Int64]
	inFlightLimit    int64
}

// decrInFlight decrements the in flight request counter for a given resource.
func (rc *RequestCounter) decrInFlight(resource string) {
	rc.ensureInFlightCounterForResource(resource).Add(-1)
}

func (rc *RequestCounter) preRequestIncrement(key string) {
	if stats, ok := rc.requestKeyToStats.Load(key); ok {
		stats.count.Add(1)
	} else {
		// If a key for the request did not yet exist, create a new `requestStats` to add to the
		// map.
		newStats := new(requestStats)
		newStats.count.Add(1)

		// However, it is also possible that our store into the map races with another concurrent
		// store for the "first" request.
		storedStats, exists := rc.requestKeyToStats.LoadOrStore(key, newStats)
		// If we lost that race, instead bump the counter of the `requestStats` object that was
		// inserted.
		if exists {
			storedStats.count.Add(1)
		}
	}
}

func (rc *RequestCounter) postRequestIncrement(key string, timeSpent time.Duration, dataSent int, wasError bool) {
	if stats, ok := rc.requestKeyToStats.Load(key); ok {
		stats.timeSpent.Add(timeSpent.Milliseconds())
		stats.dataSent.Add(int64(dataSent))
		if wasError {
			stats.errorCnt.Add(1)
		}
	} else if testing.Testing() {
		panic(fmt.Sprintf("Invariant failed. Key must exist in `postRequestIncrement`. Key: %v", key))
	}
}

// Stats satisfies the ftdc.Statser interface and will return a copy of the counters.
func (rc *RequestCounter) Stats() any {
	ret := make(map[string]int64)
	for requestKey, requestStats := range rc.requestKeyToStats.Range {
		ret[fmt.Sprintf("%v", requestKey)] = requestStats.count.Load()
		ret[fmt.Sprintf("%v.errorCnt", requestKey)] = requestStats.errorCnt.Load()
		ret[fmt.Sprintf("%v.timeSpent", requestKey)] = requestStats.timeSpent.Load()
		ret[fmt.Sprintf("%v.dataSentBytes", requestKey)] = requestStats.dataSent.Load()
	}

	for k, v := range rc.inFlightRequests.Range {
		ret[fmt.Sprintf("%v.inFlightRequests", k)] = v.Load()
	}

	return ret
}

// UnaryInterceptor returns an incoming server interceptor that will pull method information and
// optionally resource information to bump the request counters.
func (rc *RequestCounter) UnaryInterceptor(
	ctx context.Context, req any, info *googlegrpc.UnaryServerInfo, handler googlegrpc.UnaryHandler,
) (resp any, err error) {
	apiMethod := extractViamAPI(info.FullMethod)
	if resource := buildResourceLimitKey(req, apiMethod); resource != "" {
		if ok := rc.incrInFlight(resource); !ok {
			rc.logger.Warnw(fmt.Sprintf("Request limit exceeded for resource. See %s for troubleshooting steps", ReqLimitExceededURL),
				"method", apiMethod.full,
				"resource", resource)
			return nil, &RequestLimitExceededError{
				resource: resource,
				limit:    rc.inFlightLimit,
			}
		}
		defer rc.decrInFlight(resource)
	}
	requestCounterKey := buildRCKey(req, apiMethod)
	// Storing in FTDC: `web.motor-name.MotorService/IsMoving: <count>`.
	if apiMethod.shortPath != "" {
		rc.preRequestIncrement(requestCounterKey)

		start := time.Now()
		defer func() {
			// Dan: Some metrics want to take the difference of "time spent" between two recordings
			// (spaced by some "window size") and divide by the "number of calls". Doing the
			// `incrementCounter` at the RPC call start and `incrementTimeSpent` at the end creates
			// an odd skew. Where at some later point there will be an increase in time spent not
			// immediately accompanied by an increase in calls.
			//
			// This can create difficult to parse data when requests start taking a "window size"
			// amount of time to complete. We may want to consider calling `incrementCounter` in the
			// defer. But that could lead to a scenario where, if an RPC call causes a deadlock,
			// FTDC wouldn't have any record of that RPC call being invoked.
			//
			// Perhaps the "perfect" solution is to track both "request started" and "request
			// finished". And have latency graphs use "request finished".
			respSize := 0
			if protoMsg, ok := resp.(proto.Message); ok {
				respSize = proto.Size(protoMsg)
			}
			rc.postRequestIncrement(
				requestCounterKey,
				time.Since(start),
				respSize,
				err != nil)
		}()
	}

	resp, err = handler(ctx, req)
	return resp, err
}

func (rc *RequestCounter) ensureLimit() {
	if rc.inFlightLimit == 0 {
		if limitVar, err := strconv.Atoi(os.Getenv(rutils.ViamResourceRequestsLimitEnvVar)); err == nil && limitVar > 0 {
			rc.inFlightLimit = int64(limitVar)
		} else {
			rc.inFlightLimit = 100
		}
	}
}

func (rc *RequestCounter) ensureInFlightCounterForResource(resource string) *atomic.Int64 {
	// This could be a single call to LoadOrStore, but doing so would create GC
	// pressure as every call would heap-allocate an atomic.Int64, only the first
	// of which is actually used. Checking if the counter already exists with
	// Load first avoids those unnecessary allocations at the cost of making the
	// initial call for each resource slower.
	counter, ok := rc.inFlightRequests.Load(resource)
	if !ok {
		counter, _ = rc.inFlightRequests.LoadOrStore(resource, &atomic.Int64{})
	}
	return counter
}

// incrInFlight attempts to increment the in flight request counter for a given
// resource. It returns true if it was successful and false if an additional
// request would exceed the configured limit.
func (rc *RequestCounter) incrInFlight(resource string) bool {
	counter := rc.ensureInFlightCounterForResource(resource)
	if newCount := counter.Add(1); newCount > rc.inFlightLimit {
		counter.Add(-1)
		return false
	}
	return true
}

// StreamInterceptor extracts the service and method names before invoking the handler to complete the RPC.
// It is called once per stream and will run on:
// Client streaming: rpc Method (stream a) returns (b)
// Server streaming: rpc Method (a) returns (stream b)
// Bidirectional streaming: rpc Method (stream a) returns (stream b).
func (rc *RequestCounter) StreamInterceptor(
	srv any,
	ss googlegrpc.ServerStream,
	info *googlegrpc.StreamServerInfo,
	handler googlegrpc.StreamHandler,
) error {
	apiMethod := extractViamAPI(info.FullMethod)

	// Only count Viam apiMethods
	if apiMethod.shortPath != "" {
		wrappedStream := wrappedStreamWithRC{
			ServerStream: ss,
			apiMethod:    apiMethod,
			rc:           rc,
			requestKey:   atomic.Pointer[string]{},
		}
		return handler(srv, &wrappedStream)
	}
	return handler(srv, ss)
}

type wrappedStreamWithRC struct {
	googlegrpc.ServerStream
	apiMethod apiMethod
	rc        *RequestCounter

	// Set on the initial client request.
	requestKey atomic.Pointer[string]
}

// RecvMsg increments the reference counter upon receiving the first message from the client.
// It is called on every message the client streams to the server (potentially many times per stream).
func (w *wrappedStreamWithRC) RecvMsg(m any) error {
	// Unmarshalls into m (to populate fields).
	err := w.ServerStream.RecvMsg(m)

	if w.requestKey.Load() == nil {
		requestKey := buildRCKey(m, w.apiMethod)
		w.requestKey.Store(&requestKey)
		// Dan: As above, we have to call the underlying handler first before
		// `preRequestIncrement`. Because the message object has not been initialized yet. It's not
		// clear to me what options we have to pull out the message's `name` field before
		w.rc.preRequestIncrement(requestKey)
	}

	return err
}

func (w *wrappedStreamWithRC) SendMsg(m any) error {
	if requestKeyPtr := w.requestKey.Load(); requestKeyPtr != nil {
		if protoMsg, ok := m.(proto.Message); ok {
			w.rc.postRequestIncrement(*requestKeyPtr, 0, proto.Size(protoMsg), false)
		}
	} else {
		panic(fmt.Sprintf("Invariant failed. Key must exist for `postRequestIncrement`. Key: %v", w.requestKey.Load()))
	}

	err := w.ServerStream.SendMsg(m)
	return err
}

func extractViamAPI(fullMethod string) apiMethod {
	// Extract API information from `fullMethod` values such as:
	// - `/viam.component.motor.v1.MotorService/IsMoving` -> {
	//     full:      "/viam.component.motor.v1.MotorService/IsMoving",
	//     service:   "viam.component.motor.v1.MotorService",
	//     name:      "IsMoving",
	//     shortPath: "MotorService/IsMoving",
	//   }
	// - `/viam.robot.v1.RobotService/SendSessionHeartbeat` -> {
	//     full:      "/viam.robot.v1.RobotService/SendSessionHeartbeat",
	//     service:   "viam.robot.v1.RobotService",
	//     name:      "SendSessionHeartbeat",
	//     shortPath: "RobotService/SendSessionHeartbeat",
	//   }
	switch {
	case strings.HasPrefix(fullMethod, "/viam.component."):
		fallthrough
	case strings.HasPrefix(fullMethod, "/viam.service."):
		fallthrough
	case strings.HasPrefix(fullMethod, "/viam.robot."):
		split := strings.SplitN(fullMethod, "/", 3)
		service := split[1]
		method := split[2]
		return apiMethod{
			full:      fullMethod,
			name:      method,
			shortPath: service[strings.LastIndexByte(service, byte('.'))+1:] + "/" + method,
			service:   service,
		}
	default:
		return apiMethod{}
	}
}

// buildRCKey builds the key to be used in the RequestCounter's counts map.
// If the msg satisfies web.Namer, the key will be in the format "name.method",
// Otherwise, the key will be just "method".
func buildRCKey(clientMsg any, method apiMethod) string {
	if clientMsg != nil {
		if name := method.getResourceName(clientMsg); name != "" {
			return fmt.Sprintf("%v.%v", name, method.shortPath)
		}
	}
	return method.shortPath
}

func buildResourceLimitKey(clientMsg any, method apiMethod) string {
	if method.shortPath == "" {
		// Ignore for nun-Viam APIs
		return ""
	}
	if name := method.getResourceName(clientMsg); name != "" {
		return name + "." + method.service
	}
	if method.service == "viam.robot.v1.RobotService" {
		return method.service
	}
	return ""
}
