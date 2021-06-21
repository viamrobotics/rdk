package mongoutils_test

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"

	"go.viam.com/test"

	"go.viam.com/core/testutils"
	mongoutils "go.viam.com/core/utils/mongo"
)

func TestChangeStreamNextBackground(t *testing.T) {
	client := testutils.BackingMongoDBClient(t)
	dbName, collName := testutils.NewMongoDBNamespace()
	coll := client.Database(dbName).Collection(collName)

	cs, err := coll.Watch(context.Background(), []bson.D{
		{
			{"$match", bson.D{}},
		},
	}, options.ChangeStream().SetFullDocument(options.UpdateLookup))
	test.That(t, err, test.ShouldBeNil)

	result, cancel := mongoutils.ChangeStreamNextBackground(context.Background(), cs)
	cancel()
	next := <-result
	test.That(t, next.Error, test.ShouldWrap, context.Canceled)

	cs, err = coll.Watch(context.Background(), []bson.D{
		{
			{"$match", bson.D{}},
		},
	}, options.ChangeStream().SetFullDocument(options.UpdateLookup))
	test.That(t, err, test.ShouldBeNil)

	cancelCtx, ctxCancel := context.WithCancel(context.Background())
	result, cancel = mongoutils.ChangeStreamNextBackground(cancelCtx, cs)
	ctxCancel()
	next = <-result
	test.That(t, next.Error, test.ShouldWrap, context.Canceled)
	cancel()

	cs, err = coll.Watch(context.Background(), []bson.D{
		{
			{"$match", bson.D{}},
		},
	}, options.ChangeStream().SetFullDocument(options.UpdateLookup))
	test.That(t, err, test.ShouldBeNil)

	result, cancel = mongoutils.ChangeStreamNextBackground(context.Background(), cs)
	defer cancel()
	doc := bson.D{{"_id", primitive.NewObjectID()}}
	errCh := make(chan error, 1)
	go func() {
		_, err := coll.InsertOne(context.Background(), doc)
		errCh <- err
	}()
	next = <-result
	test.That(t, next.Error, test.ShouldBeNil)
	test.That(t, <-errCh, test.ShouldBeNil)
	var retDoc bson.D
	test.That(t, next.Event.FullDocument.Unmarshal(&retDoc), test.ShouldBeNil)
	test.That(t, retDoc, test.ShouldResemble, doc)
}
