package robot

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigRobot(t *testing.T) {
	cfg, err := ReadConfig("data/robot.json")
	if err != nil {
		t.Fatal(err)
	}

	if len(cfg.Components) != 4 {
		t.Errorf("bad config read %v", cfg)
	}

}

func TestConfig1(t *testing.T) {
	cfg, err := ReadConfig("data/cfgtest1.json")
	if err != nil {
		t.Fatal(err)
	}

	r, err := NewRobot(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	pic, _, err := r.Cameras[0].Next(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	bounds := pic.Bounds()

	if bounds.Max.X < 100 {
		t.Errorf("pictures seems wrong %d %d", bounds.Max.X, bounds.Max.Y)
	}

	assert.Equal(t, fmt.Sprintf("a%sb%sc", os.Getenv("HOME"), os.Getenv("HOME")), cfg.Components[0].Attributes["bar"])

}

func TestConfig2(t *testing.T) {
	cfg, err := ReadConfig("data/cfgtest2.json")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, len(cfg.Boards))
	assert.Equal(t, "38", cfg.Boards[0].Motors[0].Pins["b"])
}

func TestConfigFake(t *testing.T) {
	cfg, err := ReadConfig("data/fake.json")
	if err != nil {
		t.Fatal(err)
	}

	r, err := NewRobot(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}
