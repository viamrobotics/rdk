package api

import (
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

func TestConfig2(t *testing.T) {
	cfg, err := ReadConfig("data/cfgtest2.json")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, len(cfg.Boards))
	assert.Equal(t, "38", cfg.Boards[0].Motors[0].Pins["b"])
}
