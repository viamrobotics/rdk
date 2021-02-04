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

	r, err := NewRobot(cfg)
	if err != nil {
		t.Fatal(err)
	}

	pic, _, err := r.Cameras[0].NextImageDepthPair(context.Background())
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
	cfg, err := ReadConfig("data/fake.json")
	if err != nil {
		t.Fatal(err)
	}

	r, err := NewRobot(cfg)
	if err != nil {
		t.Fatal(err)
	}
	r.Close()
}
