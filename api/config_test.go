package api

import (
	"testing"

	"github.com/edaniels/test"
	"github.com/mitchellh/mapstructure"
	"go.viam.com/robotcore/board"
	"go.viam.com/robotcore/rexec"

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
	assert.Equal(t, RemoteConfig{Name: "one", Address: "foo", Prefix: true}, cfg.Remotes[0])
	assert.Equal(t, RemoteConfig{Name: "two", Address: "bar"}, cfg.Remotes[1])
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

func TestCreateCloudRequest(t *testing.T) {
	cfg := CloudConfig{
		ID:     "a",
		Secret: "b",
		Path:   "c",
	}
	r, err := createRequest(&cfg)
	if err != nil {
		t.Fatal(err)
	}

	test.That(t, r.Header.Get("Secret"), test.ShouldEqual, cfg.Secret)
	test.That(t, r.URL.String(), test.ShouldEqual, "c?id=a")
}

func TestConfigValidate(t *testing.T) {
	var emptyConfig Config
	test.That(t, emptyConfig.Validate(), test.ShouldBeNil)

	invalidCloud := Config{
		Cloud: &CloudConfig{},
	}
	err := invalidCloud.Validate()
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `cloud`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"id" is required`)
	invalidCloud.Cloud.ID = "some_id"
	err = invalidCloud.Validate()
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"secret" is required`)
	invalidCloud.Cloud.Secret = "my_secret"
	test.That(t, invalidCloud.Validate(), test.ShouldBeNil)

	invalidRemotes := Config{
		Remotes: []RemoteConfig{{}},
	}
	err = invalidRemotes.Validate()
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `remotes.0`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)
	invalidRemotes.Remotes[0].Name = "foo"
	err = invalidRemotes.Validate()
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"address" is required`)
	invalidRemotes.Remotes[0].Address = "bar"
	test.That(t, invalidRemotes.Validate(), test.ShouldBeNil)

	invalidBoards := Config{
		Boards: []board.Config{{}},
	}
	err = invalidBoards.Validate()
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `boards.0`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)
	invalidBoards.Boards[0].Name = "foo"
	test.That(t, invalidBoards.Validate(), test.ShouldBeNil)

	invalidComponents := Config{
		Components: []ComponentConfig{{}},
	}
	err = invalidComponents.Validate()
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `components.0`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)
	invalidComponents.Components[0].Name = "foo"
	test.That(t, invalidComponents.Validate(), test.ShouldBeNil)

	invalidProcesses := Config{
		Processes: []rexec.ProcessConfig{{}},
	}
	err = invalidProcesses.Validate()
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `processes.0`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"id" is required`)
	invalidProcesses.Processes[0].ID = "bar"
	err = invalidProcesses.Validate()
	test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)
	invalidProcesses.Processes[0].Name = "foo"
	test.That(t, invalidProcesses.Validate(), test.ShouldBeNil)
}
