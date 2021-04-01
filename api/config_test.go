package api

import (
	"testing"

	"github.com/mitchellh/mapstructure"

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

	assert.Equal(t, 2, len(cfg.Remotes))
	assert.Equal(t, Remote{Name: "one", Address: "foo", Prefix: true}, cfg.Remotes[0])
	assert.Equal(t, Remote{Address: "bar"}, cfg.Remotes[1])
}

func TestConfig2(t *testing.T) {
	cfg, err := ReadConfig("data/cfgtest2.json")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, len(cfg.Boards))
	assert.Equal(t, "38", cfg.Boards[0].Motors[0].Pins["b"])
}

func TestConfig3(t *testing.T) {
	type temp struct {
		X int
		Y string
	}

	Register("foo", "eliot", "bar", func(sub interface{}) (interface{}, error) {
		t := &temp{}
		err := mapstructure.Decode(sub, t)
		return t, err
	},
	)

	cfg, err := ReadConfig("data/config3.json")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, len(cfg.Components))
	assert.Equal(t, 5, cfg.Components[0].Attributes.GetInt("foo", 0))
	assert.Equal(t, true, cfg.Components[0].Attributes.GetBool("foo2", false))
	assert.Equal(t, false, cfg.Components[0].Attributes.GetBool("foo3", false))
	assert.Equal(t, true, cfg.Components[0].Attributes.GetBool("xxxx", true))
	assert.Equal(t, false, cfg.Components[0].Attributes.GetBool("xxxx", false))
	assert.Equal(t, "no", cfg.Components[0].Attributes.GetString("foo4"))
	assert.Equal(t, "", cfg.Components[0].Attributes.GetString("xxxx"))
	assert.Equal(t, true, cfg.Components[0].Attributes.Has("foo"))
	assert.Equal(t, false, cfg.Components[0].Attributes.Has("xxxx"))

	bb := cfg.Components[0].Attributes["bar"]
	b := bb.(*temp)
	assert.Equal(t, 6, b.X)
	assert.Equal(t, "eliot", b.Y)

	assert.Equal(t, 5.17, cfg.Components[0].Attributes.GetFloat64("bar5", 1.1))
	assert.Equal(t, 1.1, cfg.Components[0].Attributes.GetFloat64("bar5-no", 1.1))
}

func TestConfigLoad1(t *testing.T) {
	cfg, err := ReadConfig("data/cfg3.json")
	if err != nil {
		t.Fatal(err)
	}

	c1 := cfg.FindComponent("c1")
	if c1 == nil {
		t.Fatalf("no c1")
	}

	_, ok := c1.Attributes["matrics"].(string)
	assert.False(t, ok)

	c2 := cfg.FindComponent("c2")
	if c2 == nil {
		t.Fatalf("no c2")
	}

	assert.Equal(t, 5.1, c2.Attributes["matrics"].(map[string]interface{})["a"])
}
