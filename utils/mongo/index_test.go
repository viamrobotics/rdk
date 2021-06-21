package mongoutils_test

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"go.viam.com/test"

	"go.viam.com/core/testutils"
	mongoutils "go.viam.com/core/utils/mongo"
)

func TestEnsureIndexes(t *testing.T) {
	client := testutils.BackingMongoDBClient(t)
	dbName, collName := testutils.NewMongoDBNamespace()
	coll := client.Database(dbName).Collection(collName)

	barIndexName := "bar"
	expireAfter := int32(2)
	test.That(t, mongoutils.EnsureIndexes(
		coll,
		mongo.IndexModel{Keys: bson.D{{"foo", 1}}},
		mongo.IndexModel{Keys: bson.D{{"bar", 1}}, Options: &options.IndexOptions{
			Name:               &barIndexName,
			ExpireAfterSeconds: &expireAfter,
		}},
	), test.ShouldBeNil)
	test.That(t, mongoutils.EnsureIndexes(
		coll,
		mongo.IndexModel{Keys: bson.D{{"foo", 1}}},
		mongo.IndexModel{Keys: bson.D{{"bar", 1}}, Options: &options.IndexOptions{
			Name:               &barIndexName,
			ExpireAfterSeconds: &expireAfter,
		}},
	), test.ShouldBeNil)
	err := mongoutils.EnsureIndexes(
		coll,
		mongo.IndexModel{Keys: bson.D{{"foo", 1}}},
		mongo.IndexModel{Keys: bson.D{{"bar", 1}}},
	)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "already exists")

	indexCursor, err := coll.Indexes().List(context.Background())
	test.That(t, err, test.ShouldBeNil)
	var indexes []bson.M
	test.That(t, indexCursor.All(context.Background(), &indexes), test.ShouldBeNil)
	test.That(t, indexes, test.ShouldHaveLength, 3)
	nameToIndex := make(map[string]bson.M, len(indexes))
	for _, index := range indexes {
		nameToIndex[index["name"].(string)] = index
	}
	delete(nameToIndex, "_id_")
	delete(nameToIndex, "foo_1")

	barIndex, ok := nameToIndex["bar"]
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, barIndex["expireAfterSeconds"], test.ShouldEqual, 2)
	delete(nameToIndex, "bar")
	test.That(t, nameToIndex, test.ShouldHaveLength, 0)
}
