package mongoutils

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// EnsureIndexes ensures that the given indexes are created on the given collection.
func EnsureIndexes(coll *mongo.Collection, indexes ...mongo.IndexModel) error {
	_, err := coll.Indexes().CreateMany(context.Background(), indexes)
	return err
}
