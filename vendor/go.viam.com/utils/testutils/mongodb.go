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
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.uber.org/multierr"

	"go.viam.com/utils"
	mongoutils "go.viam.com/utils/mongo"
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

func setupMongoDBForTests() {
	usingMongoDBMu.Lock()
	if !usingMongoDB {
		usingMongoDB = true
		randomizedMongoDBNamespaces, restoreMongoDBNamespaces = mongoutils.RandomizeNamespaces()
		currentOpts := mongoutils.GlobalDatabaseOptions()
		currentOpts = append(currentOpts, options.Database().SetReadConcern(readconcern.Majority()))
		mongoutils.SetGlobalDatabaseOptions(currentOpts...)
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
	errCachedBackingMongoDBClient       error
)

func backingMongoDBClient() (*mongo.Client, error) {
	return backingMongoDBClientWithOptions(options.Client())
}

func backingMongoDBClientWithOptions(baseOptions *options.ClientOptions) (*mongo.Client, error) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if cachedBackingMongoDBClient != nil && cachedBackingMongoDBClientConnected {
		return cachedBackingMongoDBClient, nil
	}
	if errCachedBackingMongoDBClient != nil {
		return nil, errCachedBackingMongoDBClient
	}
	mongoURI, err := backingMongoDBURI()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var clientOptions *options.ClientOptions
	if baseOptions == nil {
		clientOptions = options.Client().ApplyURI(mongoURI)
	} else {
		clientOptions = baseOptions.ApplyURI(mongoURI)
	}

	client, err := mongo.NewClient(clientOptions)
	if err != nil {
		errCachedBackingMongoDBClient = err
		return nil, errCachedBackingMongoDBClient
	}
	if err := client.Connect(ctx); err != nil {
		errCachedBackingMongoDBClient = err
		return nil, errCachedBackingMongoDBClient
	}
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		errCachedBackingMongoDBClient = multierr.Combine(err, client.Disconnect(ctx))
		return nil, errCachedBackingMongoDBClient
	}
	if result := client.Database("admin").RunCommand(ctx, bson.D{{"replSetGetStatus", 1}}); result.Err() != nil {
		errCachedBackingMongoDBClient = multierr.Combine(result.Err(), client.Disconnect(ctx))
		return nil, errCachedBackingMongoDBClient
	}
	cachedBackingMongoDBClient = client
	cachedBackingMongoDBClientConnected = true
	errCachedBackingMongoDBClient = nil
	return client, nil
}

// BackingMongoDBClient returns a backing MongoDB client to use.
func BackingMongoDBClient(tb testing.TB) *mongo.Client {
	tb.Helper()
	client, err := backingMongoDBClient()
	if err != nil {
		skipWithError(tb, err)
		return nil
	}
	return client
}

// BackingMongoDBClientWithOptions returns a backing MongoDB client to use with the provided options.
func BackingMongoDBClientWithOptions(tb testing.TB, baseOptions *options.ClientOptions) *mongo.Client {
	tb.Helper()
	client, err := backingMongoDBClientWithOptions(baseOptions)
	if err != nil {
		skipWithError(tb, err)
		return nil
	}
	return client
}
