package mongoutils

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"go.viam.com/core/utils"
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

// ChangeEvent operation types
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

// ChangeEventTo is used when when operationType is rename; This document displays the
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
	Event *ChangeEvent
	Error error
}

// ChangeStreamNextBackground calls Next in the background and returns once at least one attempt has
// been made. It returns a result that can be received after the call is done.
func ChangeStreamNextBackground(ctx context.Context, cs *mongo.ChangeStream) (<-chan ChangeEventResult, func()) {
	result := make(chan ChangeEventResult, 1)
	nextCtx, cancel := context.WithCancel(ctx)
	csStarted := make(chan struct{}, 1)
	utils.PanicCapturingGo(func() {
		defer close(result)
		var ce ChangeEvent
		if cs.TryNext(nextCtx) {
			close(csStarted)
			if err := cs.Decode(&ce); err != nil {
				result <- ChangeEventResult{Error: err}
				return
			}
			result <- ChangeEventResult{Event: &ce}
			return
		}
		close(csStarted)
		if cs.Next(nextCtx) {
			if err := cs.Decode(&ce); err != nil {
				result <- ChangeEventResult{Error: err}
				return
			}
			result <- ChangeEventResult{Event: &ce}
			return
		}
		result <- ChangeEventResult{Error: cs.Err()}
	})
	<-csStarted
	return result, cancel
}
