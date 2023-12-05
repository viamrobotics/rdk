package navigation

import (
	"context"
	"math"
	"sync"
	"time"

	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.uber.org/multierr"
	mongoutils "go.viam.com/utils/mongo"
)

var errNoMoreWaypoints = errors.New("no more waypoints")

// NavStore handles the waypoints for a navigation service.
type NavStore interface {
	Waypoints(ctx context.Context) ([]Waypoint, error)
	AddWaypoint(ctx context.Context, point *geo.Point) (Waypoint, error)
	RemoveWaypoint(ctx context.Context, id primitive.ObjectID) error
	NextWaypoint(ctx context.Context) (Waypoint, error)
	WaypointVisited(ctx context.Context, id primitive.ObjectID) error
	Close(ctx context.Context) error
}

type storeType string

const (
	// StoreTypeUnset represents when a store type was not set.
	StoreTypeUnset = ""
	// StoreTypeMemory is the constant for the memory store type.
	StoreTypeMemory = "memory"
	// StoreTypeMongoDB is the constant for the mongodb store type.
	StoreTypeMongoDB = "mongodb"
)

// StoreConfig describes how to configure data storage.
type StoreConfig struct {
	Type   storeType              `json:"type"`
	Config map[string]interface{} `json:"config"`
}

// Validate ensures all parts of the config are valid.
func (config *StoreConfig) Validate(path string) error {
	switch config.Type {
	case StoreTypeMemory, StoreTypeMongoDB, StoreTypeUnset:
	default:
		return errors.Errorf("unknown store type %q", config.Type)
	}
	return nil
}

// NewStoreFromConfig builds a NavStore from the provided StoreConfig and returns it.
func NewStoreFromConfig(ctx context.Context, conf StoreConfig) (NavStore, error) {
	switch conf.Type {
	case StoreTypeMemory, StoreTypeUnset:
		return NewMemoryNavigationStore(), nil
	case StoreTypeMongoDB:
		return NewMongoDBNavigationStore(ctx, conf.Config)
	default:
		return nil, errors.Errorf("unknown store type %q", conf.Type)
	}
}

// A Waypoint designates a location within a path to navigate to.
type Waypoint struct {
	ID      primitive.ObjectID `bson:"_id"`
	Visited bool               `bson:"visited"`
	Order   int                `bson:"order"`
	Lat     float64            `bson:"latitude"`
	Long    float64            `bson:"longitude"`
}

// ToPoint converts the waypoint to a geo.Point.
func (wp *Waypoint) ToPoint() *geo.Point {
	return geo.NewPoint(wp.Lat, wp.Long)
}

// LatLongApproxEqual returns true if the lat / long of the waypoint is within a small epsilon of the parameter.
func (wp *Waypoint) LatLongApproxEqual(wp2 Waypoint) bool {
	const epsilon = 1e-16
	return math.Abs(wp.Lat-wp2.Lat) < epsilon && math.Abs(wp.Long-wp2.Long) < epsilon
}

// NewMemoryNavigationStore returns and empty MemoryNavigationStore.
func NewMemoryNavigationStore() *MemoryNavigationStore {
	return &MemoryNavigationStore{}
}

// MemoryNavigationStore holds the waypoints for the navigation service.
type MemoryNavigationStore struct {
	mu        sync.RWMutex
	waypoints []*Waypoint
}

// Waypoints returns a copy of all of the waypoints in the MemoryNavigationStore.
func (store *MemoryNavigationStore) Waypoints(ctx context.Context) ([]Waypoint, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	wps := make([]Waypoint, 0, len(store.waypoints))
	for _, wp := range store.waypoints {
		if wp.Visited {
			continue
		}
		wpCopy := *wp
		wps = append(wps, wpCopy)
	}
	return wps, nil
}

// AddWaypoint adds a waypoint to the MemoryNavigationStore.
func (store *MemoryNavigationStore) AddWaypoint(ctx context.Context, point *geo.Point) (Waypoint, error) {
	if ctx.Err() != nil {
		return Waypoint{}, ctx.Err()
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	newPoint := Waypoint{
		ID:   primitive.NewObjectID(),
		Lat:  point.Lat(),
		Long: point.Lng(),
	}
	store.waypoints = append(store.waypoints, &newPoint)
	return newPoint, nil
}

// RemoveWaypoint removes a waypoint from the MemoryNavigationStore.
func (store *MemoryNavigationStore) RemoveWaypoint(ctx context.Context, id primitive.ObjectID) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	// the math.Max is to avoid a panic if the store is already empty
	// when RemoveWaypoint is called.
	newCapacity := int(math.Max(float64(len(store.waypoints)-1), 0))
	newWps := make([]*Waypoint, 0, newCapacity)
	for _, wp := range store.waypoints {
		if wp.ID == id {
			continue
		}
		newWps = append(newWps, wp)
	}
	store.waypoints = newWps
	return nil
}

// NextWaypoint gets the next waypoint that has not been visited.
func (store *MemoryNavigationStore) NextWaypoint(ctx context.Context) (Waypoint, error) {
	if ctx.Err() != nil {
		return Waypoint{}, ctx.Err()
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	for _, wp := range store.waypoints {
		if !wp.Visited {
			return *wp, nil
		}
	}
	return Waypoint{}, errNoMoreWaypoints
}

// WaypointVisited sets that a waypoint has been visited.
func (store *MemoryNavigationStore) WaypointVisited(ctx context.Context, id primitive.ObjectID) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	for _, wp := range store.waypoints {
		if wp.ID != id {
			continue
		}
		wp.Visited = true
	}
	return nil
}

// Close does nothing.
func (store *MemoryNavigationStore) Close(ctx context.Context) error {
	return nil
}

// Database and collection names used by the MongoDBNavigationStore.
var (
	defaultMongoDBURI                = "mongodb://127.0.0.1:27017"
	MongoDBNavStoreDBName            = "navigation"
	MongoDBNavStoreWaypointsCollName = "waypoints"
	mongoDBNavStoreIndexes           = []mongo.IndexModel{
		{
			Keys: bson.D{
				{"order", -1},
				{"_id", 1},
			},
		},
	}
)

// NewMongoDBNavigationStore creates a new navigation store using MongoDB.
func NewMongoDBNavigationStore(ctx context.Context, config map[string]interface{}) (*MongoDBNavigationStore, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	uri, ok := config["uri"].(string)
	if !ok {
		uri = defaultMongoDBURI
	}

	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}
	if err := mongoClient.Ping(ctx, readpref.Primary()); err != nil {
		return nil, multierr.Combine(err, mongoClient.Disconnect(ctx))
	}

	waypoints := mongoClient.Database(MongoDBNavStoreDBName).Collection(MongoDBNavStoreWaypointsCollName)
	if err := mongoutils.EnsureIndexes(ctx, waypoints, mongoDBNavStoreIndexes...); err != nil {
		return nil, err
	}

	return &MongoDBNavigationStore{
		mongoClient:   mongoClient,
		waypointsColl: waypoints,
	}, nil
}

// MongoDBNavigationStore holds the mongodb client and waypoints collection.
type MongoDBNavigationStore struct {
	mongoClient   *mongo.Client
	waypointsColl *mongo.Collection
}

// Close closes the connection with the mongodb client.
func (store *MongoDBNavigationStore) Close(ctx context.Context) error {
	return store.mongoClient.Disconnect(ctx)
}

// Waypoints returns a copy of all the waypoints in the MongoDBNavigationStore.
func (store *MongoDBNavigationStore) Waypoints(ctx context.Context) ([]Waypoint, error) {
	filter := bson.D{{"visited", false}}
	cursor, err := store.waypointsColl.Find(
		ctx,
		filter,
		options.Find().SetSort(bson.D{{"order", -1}, {"_id", 1}}),
	)
	if err != nil {
		return nil, err
	}

	var all []Waypoint
	if err := cursor.All(ctx, &all); err != nil {
		return nil, err
	}
	return all, nil
}

// AddWaypoint adds a waypoint to the MongoDBNavigationStore.
func (store *MongoDBNavigationStore) AddWaypoint(ctx context.Context, point *geo.Point) (Waypoint, error) {
	newPoint := Waypoint{
		ID:   primitive.NewObjectID(),
		Lat:  point.Lat(),
		Long: point.Lng(),
	}
	if _, err := store.waypointsColl.InsertOne(ctx, newPoint); err != nil {
		return Waypoint{}, err
	}
	return newPoint, nil
}

// RemoveWaypoint removes a waypoint from the MongoDBNavigationStore.
func (store *MongoDBNavigationStore) RemoveWaypoint(ctx context.Context, id primitive.ObjectID) error {
	_, err := store.waypointsColl.DeleteOne(ctx, bson.D{{"_id", id}})
	return err
}

// NextWaypoint gets the next waypoint that has not been visited.
func (store *MongoDBNavigationStore) NextWaypoint(ctx context.Context) (Waypoint, error) {
	filter := bson.D{{"visited", false}}
	result := store.waypointsColl.FindOne(
		ctx,
		filter,
		options.FindOne().SetSort(bson.D{{"order", -1}, {"_id", 1}}),
	)
	var wp Waypoint
	if err := result.Decode(&wp); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return Waypoint{}, errNoMoreWaypoints
		}
		return Waypoint{}, err
	}

	return wp, nil
}

// WaypointVisited sets that a waypoint has been visited.
func (store *MongoDBNavigationStore) WaypointVisited(ctx context.Context, id primitive.ObjectID) error {
	_, err := store.waypointsColl.UpdateOne(ctx, bson.D{{"_id", id}}, bson.D{{"$set", bson.D{{"visited", true}}}})
	return err
}
