package navigation

import (
	"context"
	"sync"
	"time"

	"github.com/go-errors/errors"
	geo "github.com/kellydunn/golang-geo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.uber.org/multierr"
	mongoutils "go.viam.com/utils/mongo"
)

var (
	errNoMoreWaypoints = errors.New("no more waypoints")
)

type navStore interface {
	Waypoints(ctx context.Context) ([]waypoint, error)
	AddWaypoint(ctx context.Context, point *geo.Point) (waypoint, error)
	NextWaypoint(ctx context.Context) (waypoint, error)
	WaypointVisited(ctx context.Context, id primitive.ObjectID) error
}

type storeType string

const (
	storeTypeMemory  = "memory"
	storeTypeMongoDB = "mongodb"
)

// StoreConfig describes how to configure data storage.
type StoreConfig struct {
	Type   storeType              `json:"type"`
	Config map[string]interface{} `json:"config"`
}

// Validate ensures all parts of the config are valid.
func (config *StoreConfig) Validate(path string) error {
	switch config.Type {
	case storeTypeMemory, storeTypeMongoDB:
	default:
		return errors.Errorf("unknown store type %q", config.Type)
	}
	return nil
}

type waypoint struct {
	ID      primitive.ObjectID `bson:"_id"`
	Visited bool               `bson:"visited"`
	Order   int                `bson:"order"`
	Lat     float64            `bson:"latitude"`
	Long    float64            `bson:"longitude"`
}

func (wp *waypoint) ToPoint() *geo.Point {
	return geo.NewPoint(wp.Lat, wp.Long)
}

func newMemoryNavigationStore() *memoryNavigationStore {
	return &memoryNavigationStore{}
}

type memoryNavigationStore struct {
	mu        sync.RWMutex
	waypoints []*waypoint
}

func (store *memoryNavigationStore) Waypoints(ctx context.Context) ([]waypoint, error) {
	store.mu.RLock()
	defer store.mu.RUnlock()
	wps := make([]waypoint, 0, len(store.waypoints))
	for _, wp := range store.waypoints {
		if wp.Visited {
			continue
		}
		wpCopy := *wp
		wps = append(wps, wpCopy)
	}
	return wps, nil
}

func (store *memoryNavigationStore) AddWaypoint(ctx context.Context, point *geo.Point) (waypoint, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	newPoint := waypoint{
		ID:   primitive.NewObjectID(),
		Lat:  point.Lat(),
		Long: point.Lng(),
	}
	store.waypoints = append(store.waypoints, &newPoint)
	return newPoint, nil
}

func (store *memoryNavigationStore) NextWaypoint(ctx context.Context) (waypoint, error) {
	store.mu.RLock()
	defer store.mu.RUnlock()
	for _, wp := range store.waypoints {
		if !wp.Visited {
			return *wp, nil
		}
	}
	return waypoint{}, errNoMoreWaypoints
}

func (store *memoryNavigationStore) WaypointVisited(ctx context.Context, id primitive.ObjectID) error {
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

// Database and collection names used by the mongoDBNavigationStore.
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

func newMongoDBNavigationStore(ctx context.Context, config map[string]interface{}) (*mongoDBNavigationStore, error) {
	// TODO(erd): use config
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
		return nil, multierr.Combine(err, mongoClient.Disconnect(context.Background()))
	}

	waypoints := mongoClient.Database(MongoDBNavStoreDBName).Collection(MongoDBNavStoreWaypointsCollName)
	if err := mongoutils.EnsureIndexes(waypoints, mongoDBNavStoreIndexes...); err != nil {
		return nil, err
	}

	return &mongoDBNavigationStore{
		mongoClient:   mongoClient,
		waypointsColl: waypoints,
	}, nil
}

type mongoDBNavigationStore struct {
	mongoClient   *mongo.Client
	waypointsColl *mongo.Collection
}

func (store *mongoDBNavigationStore) Close() error {
	return store.mongoClient.Disconnect(context.Background())
}

func (store *mongoDBNavigationStore) Waypoints(ctx context.Context) ([]waypoint, error) {
	filter := bson.D{{"visited", false}}
	cursor, err :=
		store.waypointsColl.Find(
			ctx,
			filter,
			options.Find().SetSort(bson.D{{"order", -1}, {"_id", 1}}),
		)
	if err != nil {
		return nil, err
	}

	var all []waypoint
	if err := cursor.All(ctx, &all); err != nil {
		return nil, err
	}
	return all, nil
}

func (store *mongoDBNavigationStore) AddWaypoint(ctx context.Context, point *geo.Point) (waypoint, error) {
	newPoint := waypoint{
		ID:   primitive.NewObjectID(),
		Lat:  point.Lat(),
		Long: point.Lng(),
	}
	if _, err := store.waypointsColl.InsertOne(ctx, newPoint); err != nil {
		return waypoint{}, err
	}
	return newPoint, nil
}

func (store *mongoDBNavigationStore) NextWaypoint(ctx context.Context) (waypoint, error) {
	filter := bson.D{{"visited", false}}
	result :=
		store.waypointsColl.FindOne(
			ctx,
			filter,
			options.FindOne().SetSort(bson.D{{"order", -1}, {"_id", 1}}),
		)
	var wp waypoint
	if err := result.Decode(&wp); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return waypoint{}, errNoMoreWaypoints
		}
		return waypoint{}, err
	}

	return wp, nil
}

func (store *mongoDBNavigationStore) WaypointVisited(ctx context.Context, id primitive.ObjectID) error {
	_, err := store.waypointsColl.UpdateOne(ctx, bson.D{{"_id", id}}, bson.D{{"$set", bson.D{{"visited", true}}}})
	return err
}
