package mongoutils

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"go.viam.com/utils"
)

// A ChangeEvent represents all possible fields that a change stream response document can have.
type ChangeEvent struct {
	ID                       bson.RawValue                `bson:"_id"`
	OperationType            ChangeEventOperationType     `bson:"operationType"`
	FullDocument             bson.RawValue                `bson:"fullDocument"`
	NS                       ChangeEventNamespace         `bson:"ns"`
	To                       ChangeEventTo                `bson:"to"`
	DocumentKey              bson.D                       `bson:"documentKey"`
	UpdateDescription        ChangeEventUpdateDescription `bson:"UpdateDescription"`
	ClusterTime              primitive.Timestamp          `bson:"clusterTime"`
	TransactionNumber        uint64                       `bson:"txnNumber"`
	LogicalSessionIdentifier bson.D                       `bson:"lsid"`
}

// ChangeEventOperationType is the type of operation that occurred.
type ChangeEventOperationType string

// ChangeEvent operation types.
const (
	ChangeEventOperationTypeInsert       = ChangeEventOperationType("insert")
	ChangeEventOperationTypeDelete       = ChangeEventOperationType("delete")
	ChangeEventOperationTypeReplace      = ChangeEventOperationType("replace")
	ChangeEventOperationTypeUpdate       = ChangeEventOperationType("update")
	ChangeEventOperationTypeDrop         = ChangeEventOperationType("drop")
	ChangeEventOperationTypeRename       = ChangeEventOperationType("rename")
	ChangeEventOperationTypeDropDatabase = ChangeEventOperationType("dropDatabase")
	ChangeEventOperationTypeInvalidate   = ChangeEventOperationType("invalidate")
)

// ChangeEventNamespace is the namespace (database and or collection) affected by the event.
type ChangeEventNamespace struct {
	Database   string `bson:"db"`
	Collection string `bson:"coll"`
}

// ChangeEventTo is used when operationType is rename; This document displays the
// new name for the ns collection. This document is omitted for all other values of operationType.
type ChangeEventTo ChangeEventNamespace

// ChangeEventUpdateDescription is a document describing the fields that were updated or removed
// by the update operation.
// This document and its fields only appears if the operationType is update.
type ChangeEventUpdateDescription struct {
	UpdatedFields bson.D   `bson:"updatedFields"`
	RemovedFields []string `bson:"removedFields"`
}

// ChangeEventResult represents either an event happening or an error that happened
// along the way.
type ChangeEventResult struct {
	Event       *ChangeEvent
	Error       error
	ResumeToken bson.Raw
}

// ErrChangeStreamInvalidateEvent is returned when a change stream is invalidated. When
// this happens, a corresponding resume token with an "invalidate" event can be used
// with the StartAfter option in Watch to restart.
var ErrChangeStreamInvalidateEvent = errors.New("change stream invalidated")

// ChangeStreamBackground calls Next in a background goroutine that returns a series of events
// that can be received after the call is done. It will run until the given context is done.
// Additionally, on the return of this call, the resume token and/or cluster time of the first getMore
// is returned.
// The presence of each has its own significance. When the cluster time is persent, it implies that
// the returned channel will contain an event that happened at that time. The resume token will also be
// present and refers to that event. Given how the change stream API works though, the cluster time
// can be used to restart at the time of that event while the resume token can be used to start after
// that event.
// For example, if the change stream were used to find an insertion of a document and find all updates after
// that insertion, you'd utilize the resume token from the channel. Without doing this you can either a) miss
// events or b) if no more events ever occurred, you may wait forever.
// Another example is starting a change stream to watch events for a document found/inserted out-of-band of the
// change stream. In this case you would use the resume token in the return value of this function.
// The cluster time can be used if the concurrency of the code is such that what consumes the change stream
// is concurrent with that which produces the change stream (rpc.mongoDBWebRTCCallQueue is one such case). This
// is frankly more complicated though.
// Note: It is encouraged your change stream match on the invalidate event for better error handling.
func ChangeStreamBackground(ctx context.Context, cs *mongo.ChangeStream) (<-chan ChangeEventResult, bson.Raw, primitive.Timestamp) {
	// having this be buffered probably does not matter very much but it allows for the background
	// goroutine to be slightly ahead of the consumer in some cases.
	results := make(chan ChangeEventResult, 1)
	type csStartedResult struct {
		ResumeToken bson.Raw
		ClusterTime primitive.Timestamp
	}
	csStarted := make(chan csStartedResult, 1)
	sendResult := func(result ChangeEventResult) {
		select {
		case <-ctx.Done():
			// try once more
			select {
			case results <- result:
			default:
			}
		case results <- result:
		}
	}
	utils.PanicCapturingGo(func() {
		defer close(results)

		csStartedOnce := false
		for {
			if ctx.Err() != nil {
				return
			}

			var ce ChangeEvent
			if cs.TryNext(ctx) {
				rt := cs.ResumeToken()
				if err := cs.Decode(&ce); err != nil {
					if !csStartedOnce {
						csStarted <- csStartedResult{ResumeToken: cs.ResumeToken()}
					}
					sendResult(ChangeEventResult{Error: err, ResumeToken: rt})
					return
				}
				if !csStartedOnce {
					csStartedOnce = true
					csStarted <- csStartedResult{ResumeToken: cs.ResumeToken(), ClusterTime: ce.ClusterTime}
				}
				sendResult(ChangeEventResult{Event: &ce, ResumeToken: rt})
				continue
			}
			if !csStartedOnce {
				csStartedOnce = true
				csStarted <- csStartedResult{ResumeToken: cs.ResumeToken()}
			}
			if cs.Next(ctx) {
				rt := cs.ResumeToken()
				if err := cs.Decode(&ce); err != nil {
					sendResult(ChangeEventResult{Error: err, ResumeToken: rt})
					return
				}
				if ce.OperationType == ChangeEventOperationTypeInvalidate {
					sendResult(ChangeEventResult{Error: ErrChangeStreamInvalidateEvent, ResumeToken: rt})
					return
				}
				sendResult(ChangeEventResult{Event: &ce, ResumeToken: rt})
				continue
			}
			var csErr error
			if cs.Err() == nil {
				// As far as we know this is an invalidating event like drop
				// but we are not seeing it. Better the user filter on invalidate
				// to catch it above. Otherwise they may need to resume more than
				// once (depending on oplog).
				csErr = ErrChangeStreamInvalidateEvent
			} else {
				csErr = cs.Err()
			}
			sendResult(ChangeEventResult{Error: csErr, ResumeToken: cs.ResumeToken()})
			return
		}
	})
	csRes := <-csStarted
	return results, csRes.ResumeToken, csRes.ClusterTime
}
