package vision

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestTraining1(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	store, err := NewImageTrainingStore(ctx, "mongodb://127.0.0.1/", "test", "trainingtest")
	if err != nil {
		t.Skipf("cannot run TestTraining1 because no mongo: %s\n", err)
		return
	}
	err = store.reset(ctx)
	if err != nil {
		t.Skipf("couldn't reset training collection %s", err)
		return
	}

	w1, err := store.StoreImageFromDisk(ctx, "data/white1.png", []string{"white"})
	if err != nil {
		t.Fatal(err)
	}

	w2, err := store.StoreImageFromDisk(ctx, "data/white2.png", []string{"white"})
	if err != nil {
		t.Fatal(err)
	}

	b1, err := store.StoreImageFromDisk(ctx, "data/black1.png", []string{"black"})
	if err != nil {
		t.Fatal(err)
	}

	b2, err := store.StoreImageFromDisk(ctx, "data/black2.png", []string{"black"})
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf("%s %s %s %s\n", w1, w2, b1, b2)

	temp, err := store.GetImage(ctx, w1)
	if err != nil {
		t.Fatal(err)
	}
	if len(temp.Labels) != 1 || temp.Labels[0] != "white" {
		t.Fatalf("wtf")
	}

	labels, err := store.GetLabels(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(labels) != 2 || labels["white"] != 2 || labels["black"] != 2 {
		t.Fatalf("labels wrong :( %d %d %d", len(labels), labels["white"], labels["black"])
	}

	ws, err := store.GetImagesForLabel(ctx, "white")
	if err != nil {
		t.Fatal(err)
	}
	if len(ws) != 2 {
		t.Fatalf("ws wrong %d", len(ws))
	}
}
