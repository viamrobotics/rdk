// Package vision implements computer vision algorithms.
package vision

import (
	"bytes"
	"context"
	"image"
	"image/png"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"go.viam.com/rdk/rimage"
)

// TrainingImage TODO.
type TrainingImage struct {
	ID       primitive.ObjectID `bson:"_id" json:"id,omitempty"`
	Data     []byte
	Labels   []string
	MetaData map[string]interface{}
}

// ImageTrainingStore TODO.
type ImageTrainingStore struct {
	theClient     *mongo.Client
	theCollection *mongo.Collection
}

// NewImageTrainingStore TODO.
func NewImageTrainingStore(ctx context.Context, mongoURI, db, collection string) (*ImageTrainingStore, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
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

// Close TODO.
func (its *ImageTrainingStore) Close(ctx context.Context) error {
	return its.theClient.Disconnect(ctx)
}

// BuildIndexes TODO.
func (its *ImageTrainingStore) BuildIndexes(ctx context.Context) error {
	// TODO(erh): build indexes
	return nil
}

// StoreImageFromDisk TODO.
func (its *ImageTrainingStore) StoreImageFromDisk(ctx context.Context, fn string, labels []string) (primitive.ObjectID, error) {
	img, err := rimage.NewImageFromFile(fn)
	if err != nil {
		return primitive.ObjectID{}, err
	}
	md := map[string]interface{}{"filename": fn}
	return its.StoreImage(ctx, img, md, labels)
}

// StoreImage TODO.
func (its *ImageTrainingStore) StoreImage(
	ctx context.Context,
	img image.Image,
	metaData map[string]interface{},
	labels []string,
) (primitive.ObjectID, error) {
	ti := TrainingImage{}
	ti.ID = primitive.NewObjectID()

	bb := bytes.Buffer{}
	err := png.Encode(&bb, img)
	if err != nil {
		return ti.ID, err
	}
	ti.Data = bb.Bytes()
	ti.Labels = labels
	ti.MetaData = metaData

	_, err = its.theCollection.InsertOne(ctx, ti)
	return ti.ID, err
}

// GetImage TODO.
func (its *ImageTrainingStore) GetImage(ctx context.Context, id primitive.ObjectID) (TrainingImage, error) {
	ti := TrainingImage{}
	err := its.theCollection.FindOne(ctx, bson.M{"_id": id}).Decode(&ti)
	return ti, err
}

// SetLabelsForImage TODO.
func (its *ImageTrainingStore) SetLabelsForImage(ctx context.Context, id primitive.ObjectID, labels []string) error {
	panic(1)
}

// GetImagesForLabel TODO.
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

// GetLabels TODO.
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
