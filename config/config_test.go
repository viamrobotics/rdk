package config

import (
	"testing"

	"github.com/mitchellh/mapstructure"
	"go.viam.com/test"

	"go.viam.com/robotcore/board"
	"go.viam.com/robotcore/rexec"
)

func TestConfigRobot(t *testing.T) {
	cfg, err := Read("data/robot.json")
	test.That(t, err, test.ShouldBeNil)

	test.That(t, cfg.Components, test.ShouldHaveLength, 4)
	test.That(t, len(cfg.Remotes), test.ShouldEqual, 2)
	test.That(t, cfg.Remotes[0], test.ShouldResemble, Remote{Name: "one", Address: "foo", Prefix: true})
	test.That(t, cfg.Remotes[1], test.ShouldResemble, Remote{Name: "two", Address: "bar"})
}

func TestConfig2(t *testing.T) {
	cfg, err := Read("data/cfgtest2.json")
	test.That(t, err, test.ShouldBeNil)

	test.That(t, len(cfg.Boards), test.ShouldEqual, 1)
	test.That(t, cfg.Boards[0].Motors[0].Pins["b"], test.ShouldEqual, "38")
}

func TestConfig3(t *testing.T) {
	type temp struct {
		X int
		Y string
	}

	RegisterAttributeConverter("foo", "eliot", "bar", func(sub interface{}) (interface{}, error) {
		t := &temp{}
		err := mapstructure.Decode(sub, t)
		return t, err
	},
	)

	cfg, err := Read("data/config3.json")
	test.That(t, err, test.ShouldBeNil)

	test.That(t, len(cfg.Components), test.ShouldEqual, 1)
	test.That(t, cfg.Components[0].Attributes.Int("foo", 0), test.ShouldEqual, 5)
	test.That(t, cfg.Components[0].Attributes.Bool("foo2", false), test.ShouldEqual, true)
	test.That(t, cfg.Components[0].Attributes.Bool("foo3", false), test.ShouldEqual, false)
	test.That(t, cfg.Components[0].Attributes.Bool("xxxx", true), test.ShouldEqual, true)
	test.That(t, cfg.Components[0].Attributes.Bool("xxxx", false), test.ShouldEqual, false)
	test.That(t, cfg.Components[0].Attributes.String("foo4"), test.ShouldEqual, "no")
	test.That(t, cfg.Components[0].Attributes.String("xxxx"), test.ShouldEqual, "")
	test.That(t, cfg.Components[0].Attributes.Has("foo"), test.ShouldEqual, true)
	test.That(t, cfg.Components[0].Attributes.Has("xxxx"), test.ShouldEqual, false)

	bb := cfg.Components[0].Attributes["bar"]
	b := bb.(*temp)
	test.That(t, b.X, test.ShouldEqual, 6)
	test.That(t, b.Y, test.ShouldEqual, "eliot")

	test.That(t, cfg.Components[0].Attributes.Float64("bar5", 1.1), test.ShouldEqual, 5.17)
	test.That(t, cfg.Components[0].Attributes.Float64("bar5-no", 1.1), test.ShouldEqual, 1.1)
}

func TestConfigLoad1(t *testing.T) {
	cfg, err := Read("data/cfg3.json")
	test.That(t, err, test.ShouldBeNil)

	c1 := cfg.FindComponent("c1")
	test.That(t, c1, test.ShouldNotBeNil)

	_, ok := c1.Attributes["matrics"].(string)
	test.That(t, ok, test.ShouldBeFalse)

	c2 := cfg.FindComponent("c2")
	test.That(t, c2, test.ShouldNotBeNil)

	test.That(t, c2.Attributes["matrics"].(map[string]interface{})["a"], test.ShouldEqual, 5.1)
}

func TestCreateCloudRequest(t *testing.T) {
	cfg := Cloud{
		ID:     "a",
		Secret: "b",
		Path:   "c",
	}
	r, err := createCloudRequest(&cfg)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, r.Header.Get("Secret"), test.ShouldEqual, cfg.Secret)
	test.That(t, r.URL.String(), test.ShouldEqual, "c?id=a")
}

func TestConfigValidate(t *testing.T) {
	var emptyConfig Config
	test.That(t, emptyConfig.Validate(), test.ShouldBeNil)

	invalidCloud := Config{
		Cloud: &Cloud{},
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
		Remotes: []Remote{{}},
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
		Components: []Component{{}},
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
