package robot

import (
	"testing"
)

func TestConfig1(t *testing.T) {
	cfg, err := ReadConfig("data/robot.json")
	if err != nil {
		t.Fatal(err)
	}

	if len(cfg.Components) != 3 {
		t.Errorf("bad config read %v", cfg)
	}

}
