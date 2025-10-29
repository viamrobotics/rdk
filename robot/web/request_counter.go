package web

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/viamrobotics/webrtc/v3"
	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	rutils "go.viam.com/rdk/utils"
	"go.viam.com/rdk/utils/ssync"
	"go.viam.com/utils/rpc"
)

// RequestLimitExceededError is an error returned when a request is rejected
// because it would exceed the limit for concurrent requests to a given
// resource.
type RequestLimitExceededError struct {
	resource string
	limit    int64
}

func (e RequestLimitExceededError) Error() string {
	return fmt.Sprintf("exceeded request limit %v on resource %v", e.limit, e.resource)
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

type pcStats struct {
	inFlight ssync.Map[string, *atomic.Int64]
}

// RequestCounter is used to track and limit incoming requests. It instruments
// every unary and streaming request coming in from both external clients and
// internal modules.
type RequestCounter struct {
	logger logging.Logger

	errors     atomic.Int64
	pcCounters ssync.Map[*webrtc.PeerConnection, *pcStats]

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
func (rc *RequestCounter) decrInFlight(resource string, pc *webrtc.PeerConnection) {
	rc.ensureInFlightCounterForResource(resource).Add(-1)

	stats, ok := rc.pcCounters.Load(pc)
	if !ok {
		panic("not ok")
	}

	oldCnt, ok := stats.inFlight.Load(resource)
	if !ok {
		panic("not ok")
	}
	oldCnt.Add(-1)
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

func (rc *RequestCounter) dumpPCs(pc *webrtc.PeerConnection) {
	myConnName := "module conn"
	if pc != nil {
		for statsKey := range pc.GetStats() {
			if strings.HasPrefix(statsKey, "PeerConnection-") {
				myConnName = statsKey
				break
			}
		}
	}

	errId := rc.errors.Add(1)
	rc.pcCounters.Range(func(key *webrtc.PeerConnection, val *pcStats) bool {
		if key == nil {
			output := make(map[string]int64)
			val.inFlight.Range(func(apiName string, cnt *atomic.Int64) bool {
				output[apiName] = cnt.Load()
				return true
			})

			jsonLog, err := json.MarshalIndent(output, fmt.Sprintf("ErrorID: %v", errId), "  ")
			if err != nil {
				panic(err)
			}

			rc.logger.Infof("DBG. Too many in-flight dump. RC: %p MyConn: %v. Other module connection. ErrID: %v\n%v",
				rc, myConnName, errId, string(jsonLog))
			return true
		}

		pcStats := key.GetStats()

		var pcStat string
		// JSON-ified stat blob
		var candPair map[string]any
		// map of cand name to JSON-ified stat blob
		cands := make(map[string]map[string]any)
		var localCand map[string]any
		var remoteCand map[string]any
		for statKey, statVal := range pcStats {
			// statVal is an opaque blob. JSON-ify it.
			//
			// Searching for:
			// 1. statKey: PeerConnection-1761759940137764346
			// 2. statVal.Type: "candidate-pair"
			// 3. statKey: <pair>.LocalCandidateID
			// 4. statKey: <pair>.RemoteCandidateID
			if strings.HasPrefix(statKey, "PeerConnection-") {
				pcStat = statKey
			} else if strings.HasPrefix(statKey, "candidate:") {
				statValJSON, err := json.Marshal(statVal)
				if err != nil {
					panic(err)
				}

				statValMap := make(map[string]any)
				err = json.Unmarshal(statValJSON, &statValMap)
				if err != nil {
					panic(err)
				}

				if statValMap["type"].(string) == "candidate-pair" {
					candPair = statValMap
				} else {
					cands[statKey] = statValMap
				}
			}
		}

		if pcStat == "" {
			panic("no peer connection")
		}

		if candPair == nil {
			panic("no candidate pair bad")
		}

		for candName, candStats := range cands {
			if candName == candPair["localCandidateId"].(string) {
				localCand = candStats
			} else if candName == candPair["remoteCandidateId"].(string) {
				remoteCand = candStats
			}
		}

		if localCand == nil || remoteCand == nil {
			panic(fmt.Sprintf("Nil candidates. Local: %v Remote: %v", localCand, remoteCand))
		}

		seconds, millis :=
			int64(localCand["timestamp"].(float64))/1000,
			int64(localCand["timestamp"].(float64))%1000
		connectTime := time.Unix(seconds, millis)

		logOutput := struct {
			PeerConn     string
			ConnectTime  string
			SinceConnect string
			LocalIP      string
			RemoteIP     string
			APICnt       map[string]int64
		}{
			PeerConn:     pcStat,
			ConnectTime:  fmt.Sprintf("%v", connectTime),
			SinceConnect: fmt.Sprintf("%v", time.Since(connectTime)),
			LocalIP:      fmt.Sprintf("%v", localCand["ip"]),
			RemoteIP:     fmt.Sprintf("%v", remoteCand["ip"]),
			APICnt:       make(map[string]int64),
		}
		val.inFlight.Range(func(apiName string, cnt *atomic.Int64) bool {
			logOutput.APICnt[apiName] = cnt.Load()
			return true
		})

		jsonLog, err := json.MarshalIndent(logOutput, fmt.Sprintf("ErrorID: %v", errId), "  ")
		if err != nil {
			panic(err)
		}

		rc.logger.Infof("DBG. Too many in-flight dump. RC: %p MyConn: %v. ErrorID: %v\n%v",
			rc, myConnName, errId, string(jsonLog))
		return true
	})
}

// UnaryInterceptor returns an incoming server interceptor that will pull method information and
// optionally resource information to bump the request counters.
func (rc *RequestCounter) UnaryInterceptor(
	ctx context.Context, req any, info *googlegrpc.UnaryServerInfo, handler googlegrpc.UnaryHandler,
) (resp any, err error) {
	apiMethod := extractViamAPI(info.FullMethod)
	pc, _ := rpc.ContextPeerConnection(ctx)

	if resource := buildResourceLimitKey(req, apiMethod); resource != "" {
		if ok := rc.incrInFlight(resource, pc); !ok {
			rc.logger.Warnw("Request limit exceeded for resource",
				"method", apiMethod.full, "resource", resource)
			rc.dumpPCs(pc)
			return nil, &RequestLimitExceededError{
				resource: resource,
				limit:    rc.inFlightLimit,
			}
		}
		defer rc.decrInFlight(resource, pc)
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
func (rc *RequestCounter) incrInFlight(resource string, pc *webrtc.PeerConnection) bool {
	counter := rc.ensureInFlightCounterForResource(resource)
	if newCount := counter.Add(1); newCount > rc.inFlightLimit {
		counter.Add(-1)
		return false
	}

	stats, ok := rc.pcCounters.Load(pc)
	if !ok {
		stats, _ = rc.pcCounters.LoadOrStore(pc, &pcStats{})
	}
	pcCnt, ok := stats.inFlight.Load(resource)
	if !ok {
		pcCnt, _ = stats.inFlight.LoadOrStore(resource, &atomic.Int64{})
	}
	pcCnt.Add(1)

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
