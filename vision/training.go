package vision

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"gocv.io/x/gocv"
)

type TrainingImage struct {
	ID       primitive.ObjectID `bson:"_id" json:"id,omitempty"`
	Data     []byte
	Labels   []string
	MetaData map[string]interface{}
}

type ImageTrainingStore struct {
	theClient     *mongo.Client
	theCollection *mongo.Collection
}

// -----

func NewImageTrainingStore(ctx context.Context, mongoURI string, db string, collection string) (*ImageTrainingStore, error) {
	client, err := mongo.NewClient(options.Client().ApplyURI(mongoURI))
	if err != nil {
		return nil, err
	}

	err = client.Connect(ctx)
	if err != nil {
		return nil, err
	}

	return &ImageTrainingStore{client, client.Database(db).Collection(collection)}, nil
}

func (its *ImageTrainingStore) reset(ctx context.Context) error {
	err := its.theCollection.Drop(ctx)
	if err != nil {
		return err
	}
	return its.BuildIndexes(ctx)
}

func (its *ImageTrainingStore) BuildIndexes(ctx context.Context) error {
	// TODO build indexes
	return nil
}

func (its *ImageTrainingStore) StoreImageFromDisk(ctx context.Context, fn string, labels []string) (primitive.ObjectID, error) {
	img := gocv.IMRead(fn, gocv.IMReadUnchanged)
	md := map[string]interface{}{"filename": fn}
	return its.StoreImage(ctx, img, md, labels)
}

// TODO(erh): don't use gocv.Mat here
func (its *ImageTrainingStore) StoreImage(ctx context.Context, img gocv.Mat, metaData map[string]interface{}, labels []string) (primitive.ObjectID, error) {

	ti := TrainingImage{}
	ti.ID = primitive.NewObjectID()
	ti.Data = img.ToBytes()
	ti.Labels = labels
	ti.MetaData = metaData

	_, err := its.theCollection.InsertOne(ctx, ti)
	return ti.ID, err
}

func (its *ImageTrainingStore) GetImage(ctx context.Context, id primitive.ObjectID) (TrainingImage, error) {
	ti := TrainingImage{}
	err := its.theCollection.FindOne(ctx, bson.M{"_id": id}).Decode(&ti)
	return ti, err
}

func (its *ImageTrainingStore) SetLabelsForImage(ctx context.Context, id primitive.ObjectID, labels []string) error {
	panic(1)
}

func (its *ImageTrainingStore) GetImagesForLabel(ctx context.Context, label string) ([]primitive.ObjectID, error) {
	cursor, err := its.theCollection.Find(ctx, bson.M{"labels": label}, options.Find().SetProjection(bson.M{"_id": 1}))
	if err != nil {
		return nil, err
	}

	all := []TrainingImage{}
	err = cursor.All(ctx, &all)
	if err != nil {
		return nil, err
	}

	res := []primitive.ObjectID{}
	for _, i := range all {
		res = append(res, i.ID)
	}

	return res, nil
}

func (its *ImageTrainingStore) GetLabels(ctx context.Context) (map[string]int, error) {

	agg := mongo.Pipeline{
		bson.D{{"$unwind", "$labels"}},
		bson.D{{"$group", bson.D{
			{"_id", "$labels"},
			{"num", bson.D{{"$sum", 1}}},
		}}},
	}

	cursor, err := its.theCollection.Aggregate(ctx, agg)

	if err != nil {
		return nil, err
	}

	type T struct {
		ID  string `bson:"_id"`
		Num int
	}

	var results []T
	err = cursor.All(ctx, &results)
	if err != nil {
		return nil, err
	}

	res := map[string]int{}
	for _, t := range results {
		res[t.ID] = t.Num
	}

	return res, nil
}
