package mongoutils

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
	"go.opencensus.io/trace"
)

// EnsureIndexes ensures that the given indexes are created on the given collection.
func EnsureIndexes(ctx context.Context, coll *mongo.Collection, indexes ...mongo.IndexModel) error {
	ctx, span := trace.StartSpan(ctx, "EnsureIndexes")
	defer span.End()

	_, err := coll.Indexes().CreateMany(ctx, indexes)
	return err
}
