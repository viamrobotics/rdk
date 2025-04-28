package mongoutils

import (
	"sync"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var (
	// SecondaryPreferredDatabaseOption is used to have all database and lower operations use secondary preferred.
	SecondaryPreferredDatabaseOption = options.Database().SetReadPreference(readpref.SecondaryPreferred())

	globalDBOptsMu sync.RWMutex
	globalDBOpts   []*options.DatabaseOptions
)

// GlobalDatabaseOptions gets the options to use on all calls to DatabaseFromClient.
func GlobalDatabaseOptions() []*options.DatabaseOptions {
	globalDBOptsMu.RLock()
	copiedOpts := make([]*options.DatabaseOptions, len(globalDBOpts))
	copy(copiedOpts, globalDBOpts)
	globalDBOptsMu.RUnlock()
	return copiedOpts
}

// SetGlobalDatabaseOptions sets the options to use on all calls to DatabaseFromClient.
func SetGlobalDatabaseOptions(opts ...*options.DatabaseOptions) {
	newOpts := make([]*options.DatabaseOptions, len(opts))
	copy(newOpts, opts)
	globalDBOptsMu.Lock()
	globalDBOpts = newOpts
	globalDBOptsMu.Unlock()
}

// DatabaseFromClient returns the given database from the client.
func DatabaseFromClient(client *mongo.Client, dbName string, opts ...*options.DatabaseOptions) *mongo.Database {
	var allOpts []*options.DatabaseOptions
	allOpts = append(allOpts, GlobalDatabaseOptions()...)
	allOpts = append(allOpts, opts...)
	return client.Database(dbName, allOpts...)
}
