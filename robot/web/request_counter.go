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
	"go.viam.com/utils/rpc"
	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/client"
	rutils "go.viam.com/rdk/utils"
	"go.viam.com/rdk/utils/ssync"
)

// ReqLimitExceededURL is the URL for the troubleshooting steps for request limit exceeded errors.
const ReqLimitExceededURL = "https://docs.viam.com/dev/tools/common-errors/#req-limit-exceeded"

// RequestLimitExceededError is an error returned when a request is rejected
// because it would exceed the limit for concurrent requests to a given
// resource.
type RequestLimitExceededError struct {
	resource                            string
	limit, numInFlightRequestsForClient int64
}

func (e RequestLimitExceededError) Error() string {
	return fmt.Sprintf(
		"exceeded request limit %v on resource %v (your client is responsible for %v). See %v for troubleshooting steps",
		e.limit, e.resource, e.numInFlightRequestsForClient, ReqLimitExceededURL)
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

type counterName string

const (
	inFlightCounterName counterName = "inFlight"
	rejectedCounterName counterName = "rejected"
)

type inFlightAndRejectedRequests struct {
	inFlightRequests, rejectedRequests *atomic.Int64
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

	// inFlightRequests maps resource names to how many in-flight requests are
	// currently targeting that resource name. There can only be `limit` API
	// calls for any resource. E.g: `motor-foo` can have 50 `IsPowered`
	// concurrent calls with 50 more `GoFor` calls, or instead 100 `IsPowered`
	// calls before it starts to reject new incoming requests. Unary and
	// streaming RPCs both count against the limit.`limit` defaults to 100 but
	// can be configured with the `VIAM_RESOURCE_REQUESTS_LIMIT`
	// environment variable.
	inFlightRequests ssync.Map[string, *atomic.Int64]
	inFlightLimit    int64

	// RSDK-12608:
	//
	// The two maps below exist so that diagnostic information (which client is flooding a
	// resource with requests) may be output when a "Resource limit exceeded for resource"
	// error is output.

	// requestsPerPC maps WebRTC connections (pcs) to _another_ map. That second map is a
	// mapping of resource names to how many in-flight and rejected (exceeded
	// `inFlightLimit`) requests the WebRTC connection is responsible for.
	requestsPerPC ssync.Map[*webrtc.PeerConnection, *ssync.Map[string, *inFlightAndRejectedRequests]]

	// pcToClientMetadata maps WebRTC connections (pcs) to the metadata of the connecting
	// client. This will be in the form "[type-of-sdk];[sdk-version];[api-version]"
	// potentially prefixed with "module-[name-of-module]-" to represent a module's
	// connection back to the RDK.
	pcToClientMetadata ssync.Map[*webrtc.PeerConnection, string]
}

// decrInFlight decrements the in-flight request counters for a given resource and pc.
func (rc *RequestCounter) decrInFlight(resource string, pc *webrtc.PeerConnection) {
	rc.ensureInFlightCounterForResource(resource).Add(-1)
	if pc != nil {
		rc.ensureCounterForResourceForPC(resource, pc, inFlightCounterName).Add(-1)
	}
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

// ClientInformation represents the metadata, connection information, and request counts
// of a connected client. Useful for logging client information when request limits are
// exceeded.
type ClientInformation struct {
	// ClientMetadata represents the type and version of SDK connected and potentially the
	// name of the module if it is a module->RDK connection. This will be in the form
	// "[type-of-sdk];[sdk-version];[api-version]" potentially prefixed with
	// "module-[name-of-module]-" to represent a module's connection back to the RDK.
	ClientMetadata string `json:"client_metadata"`
	// ConnectionID is the WebRTC peer connection ID of the connection; useful to associate
	// with other WebRTC logs.
	ConnectionID string `json:"connection_id"`
	// ConnectTime is the timestamp around which the client initially connected.
	ConnectTime string `json:"connect_time"`
	// TimeSinceConnect is the amount of time that has passed since `ConnectTime`.
	TimeSinceConnect string `json:"time_since_connect"`
	// ServerIP is the IP address used by the client to connect to this server.
	ServerIP string `json:"server_ip"`
	// ClientIP is the IP address of the client.
	ClientIP string `json:"client_ip"`
	// InFlightRequests is a map of resource names to the number of in-flight requests
	// against that resource this client is responsible for.
	InFlightRequests map[string]int64 `json:"inflight_requests"`
	// RejectedRequests is a map of resource names to the number of requests against that
	// resource that have exceeded the in-flight limit this client is responsible for (how
	// many times has this client caused a request limit exceeded error).
	RejectedRequests map[string]int64 `json:"rejected_requests"`
}

// Creates a client information object for logging from a passed in peer connection.
func (rc *RequestCounter) createClientInformationFromPC(
	pc *webrtc.PeerConnection,
) *ClientInformation {
	if pc == nil {
		return nil
	}

	ci := &ClientInformation{}

	if clientMetadata, ok := rc.pcToClientMetadata.Load(pc); ok {
		ci.ClientMetadata = clientMetadata
	}

	// Code to grab selected ICE candidate pair copied from goutils.
	if connectionState := pc.ICEConnectionState(); connectionState == webrtc.ICEConnectionStateConnected &&
		pc.SCTP() != nil &&
		pc.SCTP().Transport() != nil &&
		pc.SCTP().Transport().ICETransport() != nil {
		selectedCandPair, err := pc.SCTP().Transport().ICETransport().GetSelectedCandidatePair()

		// RSDK-8527: Surprisingly, `GetSelectedCandidatePair` can return `nil, nil` when the
		// ice agent has been shut down.
		if selectedCandPair != nil && err == nil {
			if selectedCandPair.Remote != nil {
				ci.ClientIP = selectedCandPair.Remote.Address
			}
			if selectedCandPair.Local != nil {
				ci.ServerIP = selectedCandPair.Local.Address
			}
		}
	}

	// The selected ICE candidate pair object above does _not_ have an associated timestamp.
	// That object also does _not_ expose its `statsID`. We loop through all peer connection
	// stats below, find the first candidate pair that's been nominated, and grab the ID of
	// its local candidate. We then use the `Timestamp` of the candidate associated with
	// that ID (when the local candidate was gathered) as a guess at the time the client
	// "connected." This guess could be wrong if multiple candidate pairs were nominated,
	// and does not really represent the point at which the client could start sending
	// requests. Those caveats are probably OK.
	//
	// TL;DR this is a hacky way to guess the connection time of the client.
	var localCandID string
	allCandsByID := make(map[string]webrtc.ICECandidateStats)
	stats := pc.GetStats()
	for _, stat := range stats {
		if candPairStat, ok := stat.(webrtc.ICECandidatePairStats); ok && candPairStat.Nominated {
			localCandID = candPairStat.LocalCandidateID
		} else if candStat, ok := stat.(webrtc.ICECandidateStats); ok {
			allCandsByID[candStat.ID] = candStat
		} else if pcStat, ok := stat.(webrtc.PeerConnectionStats); ok {
			ci.ConnectionID = pcStat.ID
		}
	}
	var timeSinceConnect time.Duration
	if localCand, ok := allCandsByID[localCandID]; ok {
		connectTime := localCand.Timestamp.Time()
		timeSinceConnect = time.Since(connectTime)
		ci.ConnectTime = connectTime.String()
		ci.TimeSinceConnect = timeSinceConnect.String()
	}

	ci.InFlightRequests = make(map[string]int64)
	ci.RejectedRequests = make(map[string]int64)
	if requestsForPC, ok := rc.requestsPerPC.Load(pc); ok {
		requestsForPC.Range(func(
			resourceName string,
			ifarr *inFlightAndRejectedRequests,
		) bool {
			if ifr := ifarr.inFlightRequests.Load(); ifr > 0 {
				ci.InFlightRequests[resourceName] = ifr
			}
			if rr := ifarr.rejectedRequests.Load(); rr > 0 {
				ci.RejectedRequests[resourceName] = rr
			}

			return true
		})
	}

	// If client has no in-flight requests, and connected >= 5 minutes ago, prune the client
	// from the `rc.requestsForPC` and `rc.pcToClientMetadata` maps so the maps do not grow
	// unboundedly. That client will re-appear in debug information if it makes another
	// request.
	if len(ci.InFlightRequests) == 0 && timeSinceConnect > 5*time.Minute {
		rc.requestsPerPC.Delete(pc)
		rc.pcToClientMetadata.Delete(pc)
	}

	return ci
}

// Logs that a particular request limit was exceeded. Outputs the following information
// (where possible):
//   - Which API method invocation was attempted
//   - Which resource the API method was invoked upon
//   - All fields of the `ClientInformation` struct for both the offending client and all
//     other clients
//
// The method also returns the number of in-flight requests against the invoked resource
// for the offending client (to be included in returned error).
func (rc *RequestCounter) logRequestLimitExceeded(
	apiMethodString, resource string,
	pc *webrtc.PeerConnection,
) int64 {
	offendingClientInformation := rc.createClientInformationFromPC(pc)
	offendingClientInformationJSON, err := json.Marshal(offendingClientInformation)
	if err != nil {
		rc.logger.Errorf("Failed to marshal client information %+v", offendingClientInformation)
	}

	var allOtherClientInformationStrs []string
	rc.requestsPerPC.Range(func(
		pcKey *webrtc.PeerConnection,
		_ *ssync.Map[string, *inFlightAndRejectedRequests],
	) bool {
		// Do not include offending client.
		if pc != pcKey {
			clientInformation := rc.createClientInformationFromPC(pcKey)
			clientInformationJSON, err := json.Marshal(clientInformation)
			if err != nil {
				rc.logger.Errorf("Failed to marshal client information %+v", clientInformation)
				return true
			}
			allOtherClientInformationStrs = append(allOtherClientInformationStrs, string(clientInformationJSON))
		}
		return true
	})

	msg := fmt.Sprintf(
		"Request limit exceeded for resource. See %s for troubleshooting steps. "+
			`{"method":%q,"resource":%q,"offending_client_information":%v,"all_other_client_information":%v}`,
		ReqLimitExceededURL,
		apiMethodString,
		resource,
		string(offendingClientInformationJSON),
		fmt.Sprintf("[%v]", strings.Join(allOtherClientInformationStrs, ",")),
	)
	rc.logger.Warnw(msg)

	return offendingClientInformation.InFlightRequests[resource]
}

// UnaryInterceptor returns an incoming server interceptor that will pull method information and
// optionally resource information to bump the request counters.
func (rc *RequestCounter) UnaryInterceptor(
	ctx context.Context, req any, info *googlegrpc.UnaryServerInfo, handler googlegrpc.UnaryHandler,
) (resp any, err error) {
	apiMethod := extractViamAPI(info.FullMethod)
	pc, pcSet := rpc.ContextPeerConnection(ctx)
	if pcSet {
		rc.setClientMetadataForPC(ctx, pc)
	}

	if resource := buildResourceLimitKey(req, apiMethod); resource != "" {
		if ok := rc.incrInFlight(resource, pc); !ok {
			numInFlightRequestsForClient := rc.logRequestLimitExceeded(apiMethod.full, resource, pc)
			return nil, &RequestLimitExceededError{
				resource:                     resource,
				limit:                        rc.inFlightLimit,
				numInFlightRequestsForClient: numInFlightRequestsForClient,
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

func (rc *RequestCounter) ensureCounterForResourceForPC(
	resource string,
	pc *webrtc.PeerConnection,
	cn counterName,
) *atomic.Int64 {
	// The same Load/LoadOrStore reasoning applies in this method as in
	// ensureInFlightCounterForResource.
	requestsForConnection, ok := rc.requestsPerPC.Load(pc)
	if !ok {
		requestsForConnection, _ = rc.requestsPerPC.LoadOrStore(
			pc,
			&ssync.Map[string, *inFlightAndRejectedRequests]{},
		)
	}
	requestsForConnectionForResource, ok := requestsForConnection.Load(resource)
	if !ok {
		requestsForConnectionForResource, _ = requestsForConnection.LoadOrStore(
			resource,
			&inFlightAndRejectedRequests{
				&atomic.Int64{},
				&atomic.Int64{},
			})
	}

	switch cn {
	case inFlightCounterName:
		return requestsForConnectionForResource.inFlightRequests
	case rejectedCounterName:
		return requestsForConnectionForResource.rejectedRequests
	default:
		rc.logger.Errorf("unrecognized counter name %s for PC; returning nil", cn)
		return nil
	}
}

// Sets the client metadata for the passed in peer connection given the passed in ctx if
// the metadata has not been set already. We assume that peer connection metadata does
// vary over the connection's lifetime.
func (rc *RequestCounter) setClientMetadataForPC(ctx context.Context, pc *webrtc.PeerConnection) {
	_, ok := rc.pcToClientMetadata.Load(pc)
	if ok {
		return
	}

	clientMetadata, clientMetadataSet := client.GetViamClientInfo(ctx)
	if !clientMetadataSet {
		// The Typescript SDK does not seem to be correctly attaching the `viam_client`
		// metadata, so absence here may mean typescript.
		clientMetadata = "maybe-typescript;unknown;unknown"
	}

	if moduleName := grpc.GetModuleName(ctx); moduleName != "" {
		clientMetadata = "module-" + moduleName + "-" + clientMetadata
	}

	rc.pcToClientMetadata.Store(pc, clientMetadata)
}

// incrInFlight attempts to increment the in-flight request counters for a given
// resource. It returns true if it was successful and false if an additional
// request would exceed the configured limit.
func (rc *RequestCounter) incrInFlight(resource string, pc *webrtc.PeerConnection) bool {
	counter := rc.ensureInFlightCounterForResource(resource)
	if newCount := counter.Add(1); newCount > rc.inFlightLimit {
		counter.Add(-1)
		if pc != nil {
			rc.ensureCounterForResourceForPC(resource, pc, rejectedCounterName).Add(1)
		}
		return false
	}
	if pc != nil {
		rc.ensureCounterForResourceForPC(resource, pc, inFlightCounterName).Add(1)
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
