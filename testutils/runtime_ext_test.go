package testutils_test

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.viam.com/test"

	"go.viam.com/core/testutils"
	mongoutils "go.viam.com/core/utils/mongo"
)

// TestTeardown modifies internal testutils state and as such no test in testutils should
// rely on this state, only external packages (run in separate test binaries).
func TestTeardown(t *testing.T) {
	dbName := primitive.NewObjectID().Hex()
	collName := primitive.NewObjectID().Hex()
	dbNameCopy := dbName
	collNameCopy := collName
	mongoutils.RegisterNamespace(&dbName, &collName)

	// this will do randomization
	testutils.SkipUnlessBackingMongoDBURI(t)

	test.That(t, dbNameCopy, test.ShouldNotEqual, dbName)
	test.That(t, collNameCopy, test.ShouldNotEqual, collName)

	client := testutils.BackingMongoDBClient(t)
	coll := client.Database(dbName).Collection(collName)
	_, err := coll.InsertOne(context.Background(), bson.D{})
	test.That(t, err, test.ShouldBeNil)

	count, err := coll.CountDocuments(context.Background(), bson.D{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, count, test.ShouldEqual, 1)

	dbNames, err := client.ListDatabaseNames(context.Background(), bson.D{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, dbNames, test.ShouldContain, dbName)

	testutils.Teardown()

	_, err = coll.CountDocuments(context.Background(), bson.D{})
	test.That(t, err, test.ShouldWrap, mongo.ErrClientDisconnected)

	client = testutils.BackingMongoDBClient(t)
	defer client.Disconnect(context.Background())
	dbNames, err = client.ListDatabaseNames(context.Background(), bson.D{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, dbNames, test.ShouldNotContain, dbName)
}
