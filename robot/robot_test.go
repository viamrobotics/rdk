package robot

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/edaniels/golog"
	"github.com/stretchr/testify/assert"

	"go.viam.com/robotcore/api"
)

func TestConfig1(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cfg, err := api.ReadConfig("data/cfgtest1.json")
	if err != nil {
		t.Fatal(err)
	}

	r, err := NewRobot(context.Background(), cfg, logger)
	if err != nil {
		t.Fatal(err)
	}

	pic, _, err := r.CameraByName("c1").Next(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	bounds := pic.Bounds()

	if bounds.Max.X < 100 {
		t.Errorf("pictures seems wrong %d %d", bounds.Max.X, bounds.Max.Y)
	}

	assert.Equal(t, fmt.Sprintf("a%sb%sc", os.Getenv("HOME"), os.Getenv("HOME")), cfg.Components[0].Attributes["bar"])
}

func TestConfigFake(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cfg, err := api.ReadConfig("data/fake.json")
	if err != nil {
		t.Fatal(err)
	}

	r, err := NewRobot(context.Background(), cfg, logger)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}
