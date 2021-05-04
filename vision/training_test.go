package vision

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/edaniels/test"
	"go.viam.com/robotcore/artifact"
)

func TestTraining1(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	store, err := NewImageTrainingStore(ctx, "mongodb://127.0.0.1/", "test", "trainingtest")
	if err != nil {
		t.Skipf("cannot run TestTraining1 because no mongo: %s\n", err)
		return
	}
	defer func() {
		test.That(t, store.Close(), test.ShouldBeNil)
	}()
	err = store.reset(ctx)
	if err != nil {
		t.Skipf("couldn't reset training collection %s", err)
		return
	}

	w1, err := store.StoreImageFromDisk(ctx, artifact.MustPath("vision/white1.png"), []string{"white"})
	test.That(t, err, test.ShouldBeNil)

	w2, err := store.StoreImageFromDisk(ctx, artifact.MustPath("vision/white2.png"), []string{"white"})
	test.That(t, err, test.ShouldBeNil)

	b1, err := store.StoreImageFromDisk(ctx, artifact.MustPath("vision/black1.png"), []string{"black"})
	test.That(t, err, test.ShouldBeNil)

	b2, err := store.StoreImageFromDisk(ctx, artifact.MustPath("vision/black2.png"), []string{"black"})
	test.That(t, err, test.ShouldBeNil)

	fmt.Printf("%s %s %s %s\n", w1, w2, b1, b2)

	temp, err := store.GetImage(ctx, w1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, temp.Labels, test.ShouldHaveLength, 1)
	test.That(t, temp.Labels[0], test.ShouldEqual, "white")

	labels, err := store.GetLabels(ctx)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, labels, test.ShouldHaveLength, 2)
	test.That(t, labels["white"], test.ShouldEqual, 2)
	test.That(t, labels["black"], test.ShouldEqual, 2)

	ws, err := store.GetImagesForLabel(ctx, "white")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ws, test.ShouldHaveLength, 2)
}
