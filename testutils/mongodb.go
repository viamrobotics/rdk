package testutils

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.uber.org/multierr"

	"go.viam.com/core/utils"
	mongoutils "go.viam.com/core/utils/mongo"
)

var (
	cacheMu                     sync.Mutex
	usingMongoDB                bool
	usingMongoDBMu              sync.Mutex
	restoreMongoDBNamespaces    func()
	randomizedMongoDBNamespaces map[string][]string
)

// NewMongoDBNamespace returns a new random namespace to use.
func NewMongoDBNamespace() (string, string) {
	dbName, collName := utils.RandomAlphaString(5), utils.RandomAlphaString(5)
	if err := mongoutils.RegisterUnmanagedNamespace(dbName, collName); err != nil {
		panic(err)
	}
	return dbName, collName
}

func randomizeMongoDBNamespaces() {
	usingMongoDBMu.Lock()
	if !usingMongoDB {
		usingMongoDB = true
		randomizedMongoDBNamespaces, restoreMongoDBNamespaces = mongoutils.RandomizeNamespaces()
	}
	usingMongoDBMu.Unlock()
}

func teardownMongoDB() {
	usingMongoDBMu.Lock()
	if !usingMongoDB {
		usingMongoDBMu.Unlock()
		return
	}
	usingMongoDBMu.Unlock()
	defer func() {
		client, err := backingMongoDBClient()
		if err != nil {
			return
		}
		if err := client.Disconnect(context.Background()); err != nil && !errors.Is(err, mongo.ErrClientDisconnected) {
			utils.UncheckedError(err)
		}
		cachedBackingMongoDBClientConnected = false
	}()
	cleanupMongoDBNamespaces()
}

func cleanupMongoDBNamespaces() {
	if restoreMongoDBNamespaces == nil {
		return
	}
	defer restoreMongoDBNamespaces()
	client, err := backingMongoDBClient()
	if err != nil {
		logger.Debugw("error getting backing MongoDB client; will not clean up", "error", err)
		return
	}
	for db := range randomizedMongoDBNamespaces {
		if err := client.Database(db).Drop(context.Background()); err != nil {
			logger.Debugw("error dropping randomized namespace", "error", err)
		}
	}
	for db := range mongoutils.UnmanagedNamespaces() {
		if err := client.Database(db).Drop(context.Background()); err != nil {
			logger.Debugw("error dropping unmanaged namespace", "error", err)
		}
	}
}

var (
	cachedBackingMongoDBClient          *mongo.Client
	cachedBackingMongoDBClientConnected bool
	cachedBackingMongoDBClientErr       error
)

func backingMongoDBClient() (*mongo.Client, error) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if cachedBackingMongoDBClient != nil && cachedBackingMongoDBClientConnected {
		return cachedBackingMongoDBClient, nil
	}
	if cachedBackingMongoDBClientErr != nil {
		return nil, cachedBackingMongoDBClientErr
	}
	mongoURI, err := backingMongoDBURI()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := mongo.NewClient(options.Client().ApplyURI(mongoURI))
	if err != nil {
		cachedBackingMongoDBClientErr = err
		return nil, cachedBackingMongoDBClientErr
	}
	if err := client.Connect(ctx); err != nil {
		cachedBackingMongoDBClientErr = err
		return nil, cachedBackingMongoDBClientErr
	}
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		cachedBackingMongoDBClientErr = multierr.Combine(err, client.Disconnect(ctx))
		return nil, cachedBackingMongoDBClientErr
	}
	if result := client.Database("admin").RunCommand(ctx, bson.D{{"replSetGetStatus", 1}}); result.Err() != nil {
		cachedBackingMongoDBClientErr = multierr.Combine(result.Err(), client.Disconnect(ctx))
		return nil, cachedBackingMongoDBClientErr
	}
	cachedBackingMongoDBClient = client
	cachedBackingMongoDBClientConnected = true
	cachedBackingMongoDBClientErr = nil
	return client, nil
}

// BackingMongoDBClient returns a backing MongoDB client to use.
func BackingMongoDBClient(t *testing.T) *mongo.Client {
	client, err := backingMongoDBClient()
	if err != nil {
		skipWithError(t, err)
		return nil
	}
	return client
}
