package config_test

import (
	"testing"

	"github.com/go-errors/errors"
	"github.com/mitchellh/mapstructure"
	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/utils/pexec"

	"go.viam.com/core/board"
	"go.viam.com/core/config"
	functionvm "go.viam.com/core/function/vm"
	"go.viam.com/core/testutils/inject"
)

func TestConfigRobot(t *testing.T) {
	cfg, err := config.Read("data/robot.json")
	test.That(t, err, test.ShouldBeNil)

	test.That(t, cfg.Components, test.ShouldHaveLength, 4)
	test.That(t, len(cfg.Remotes), test.ShouldEqual, 2)
	test.That(t, cfg.Remotes[0], test.ShouldResemble, config.Remote{Name: "one", Address: "foo", Prefix: true})
	test.That(t, cfg.Remotes[1], test.ShouldResemble, config.Remote{Name: "two", Address: "bar"})
}

func TestConfig2(t *testing.T) {
	cfg, err := config.Read("data/cfgtest2.json")
	test.That(t, err, test.ShouldBeNil)

	test.That(t, len(cfg.Boards), test.ShouldEqual, 1)
	test.That(t, cfg.Boards[0].Motors[0].Pins["b"], test.ShouldEqual, "38")
}

func TestConfig3(t *testing.T) {
	type temp struct {
		X int
		Y string
	}

	config.RegisterAttributeConverter("foo", "eliot", "bar", func(sub interface{}) (interface{}, error) {
		t := &temp{}
		err := mapstructure.Decode(sub, t)
		return t, err
	},
	)

	cfg, err := config.Read("data/config3.json")
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
	cfg, err := config.Read("data/cfg3.json")
	test.That(t, err, test.ShouldBeNil)

	c1 := cfg.FindComponent("c1")
	test.That(t, c1, test.ShouldNotBeNil)

	_, ok := c1.Attributes["matrics"].(string)
	test.That(t, ok, test.ShouldBeFalse)

	c2 := cfg.FindComponent("c2")
	test.That(t, c2, test.ShouldNotBeNil)
	test.That(t, c2.DependsOn, test.ShouldResemble, []string{"c1"})

	test.That(t, c2.Attributes["matrics"].(map[string]interface{})["a"], test.ShouldEqual, 5.1)
}

func TestCreateCloudRequest(t *testing.T) {
	cfg := config.Cloud{
		ID:     "a",
		Secret: "b",
		Path:   "c",
	}
	r, err := config.CreateCloudRequest(&cfg)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, r.Header.Get("Secret"), test.ShouldEqual, cfg.Secret)
	test.That(t, r.URL.String(), test.ShouldEqual, "c?id=a")
}

func TestConfigEnsure(t *testing.T) {
	var emptyConfig config.Config
	test.That(t, emptyConfig.Ensure(false), test.ShouldBeNil)

	invalidCloud := config.Config{
		Cloud: &config.Cloud{},
	}
	err := invalidCloud.Ensure(false)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `cloud`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"id" is required`)
	invalidCloud.Cloud.ID = "some_id"
	err = invalidCloud.Ensure(false)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"secret" is required`)
	err = invalidCloud.Ensure(true)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"self" is required`)
	invalidCloud.Cloud.Secret = "my_secret"
	test.That(t, invalidCloud.Ensure(false), test.ShouldBeNil)
	test.That(t, invalidCloud.Ensure(true), test.ShouldNotBeNil)
	invalidCloud.Cloud.Secret = ""
	invalidCloud.Cloud.Self = "wooself"
	test.That(t, invalidCloud.Ensure(true), test.ShouldBeNil)

	invalidRemotes := config.Config{
		Remotes: []config.Remote{{}},
	}
	err = invalidRemotes.Ensure(false)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `remotes.0`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)
	invalidRemotes.Remotes[0].Name = "foo"
	err = invalidRemotes.Ensure(false)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"address" is required`)
	invalidRemotes.Remotes[0].Address = "bar"
	test.That(t, invalidRemotes.Ensure(false), test.ShouldBeNil)

	invalidBoards := config.Config{
		Boards: []board.Config{{}},
	}
	err = invalidBoards.Ensure(false)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `boards.0`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)
	invalidBoards.Boards[0].Name = "foo"
	test.That(t, invalidBoards.Ensure(false), test.ShouldBeNil)

	invalidComponents := config.Config{
		Components: []config.Component{{}},
	}
	err = invalidComponents.Ensure(false)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `components.0`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)
	invalidComponents.Components[0].Name = "foo"
	test.That(t, invalidComponents.Ensure(false), test.ShouldBeNil)

	c1 := config.Component{Name: "c1"}
	c2 := config.Component{Name: "c2", DependsOn: []string{"c1"}}
	c3 := config.Component{Name: "c3", DependsOn: []string{"c1", "c2"}}
	c4 := config.Component{Name: "c4", DependsOn: []string{"c1", "c3"}}
	c5 := config.Component{Name: "c5", DependsOn: []string{"c2", "c4"}}
	c6 := config.Component{Name: "c6"}
	c7 := config.Component{Name: "c7", DependsOn: []string{"c6", "c4"}}
	unsortedComponents := config.Config{
		Components: []config.Component{c7, c6, c5, c3, c4, c1, c2},
	}
	err = unsortedComponents.Ensure(false)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, unsortedComponents.Components, test.ShouldResemble, []config.Component{c6, c1, c2, c3, c4, c7, c5})

	invalidProcesses := config.Config{
		Processes: []pexec.ProcessConfig{{}},
	}
	err = invalidProcesses.Ensure(false)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `processes.0`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"id" is required`)
	invalidProcesses.Processes[0].ID = "bar"
	err = invalidProcesses.Ensure(false)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)
	invalidProcesses.Processes[0].Name = "foo"
	test.That(t, invalidProcesses.Ensure(false), test.ShouldBeNil)

	invalidFunctions := config.Config{
		Functions: []functionvm.FunctionConfig{{}},
	}
	err = invalidFunctions.Ensure(false)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `functions.0`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)

	engName1 := utils.RandomAlphaString(64)
	injectEngine1 := &inject.Engine{}
	functionvm.RegisterEngine(functionvm.EngineName(engName1), func() (functionvm.Engine, error) {
		return injectEngine1, nil
	})

	injectEngine1.ValidateSourceFunc = func(_ string) error {
		return errors.New("whoops")
	}

	invalidFunctions.Functions[0] = functionvm.FunctionConfig{
		Name: "one",
		AnonymousFunctionConfig: functionvm.AnonymousFunctionConfig{
			Engine: functionvm.EngineName(engName1),
			Source: "three",
		},
	}
	err = invalidFunctions.Ensure(false)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `whoops`)

	injectEngine1.ValidateSourceFunc = func(_ string) error {
		return nil
	}
	test.That(t, invalidFunctions.Ensure(false), test.ShouldBeNil)
}

func TestConfigSortComponents(t *testing.T) {
	c1 := config.Component{Name: "c1"}
	c2 := config.Component{Name: "c2", DependsOn: []string{"c1"}}
	c3 := config.Component{Name: "c3", DependsOn: []string{"c1", "c2"}}
	c4 := config.Component{Name: "c4", DependsOn: []string{"c1", "c3"}}
	c5 := config.Component{Name: "c5", DependsOn: []string{"c2", "c4"}}
	c6 := config.Component{Name: "c6"}
	c7 := config.Component{Name: "c7", DependsOn: []string{"c6", "c4"}}
	c8 := config.Component{Name: "c8", DependsOn: []string{"c6"}}

	circularC1 := config.Component{Name: "c1", DependsOn: []string{"c2"}}
	circularC2 := config.Component{Name: "c2", DependsOn: []string{"c3"}}
	circularC3 := config.Component{Name: "c3", DependsOn: []string{"c1"}}
	for _, tc := range []struct {
		Name       string
		Components []config.Component
		Expected   []config.Component
		Err        string
	}{
		{
			"empty",
			[]config.Component{},
			[]config.Component{},
			"",
		},
		{
			"no change",
			[]config.Component{c1, c2},
			[]config.Component{c1, c2},
			"",
		},
		{
			"simple",
			[]config.Component{c2, c1},
			[]config.Component{c1, c2},
			"",
		},
		{
			"another simple",
			[]config.Component{c3, c2, c1},
			[]config.Component{c1, c2, c3},
			"",
		},
		{
			"complex",
			[]config.Component{c2, c3, c1},
			[]config.Component{c1, c2, c3},
			"",
		},
		{
			"more complex",
			[]config.Component{c7, c6, c5, c3, c4, c1, c2},
			[]config.Component{c6, c1, c2, c3, c4, c7, c5},
			"",
		},
		{
			"duplicate name",
			[]config.Component{c1, c1},
			nil,
			"not unique",
		},
		{
			"dependency not found",
			[]config.Component{c2},
			nil,
			"does not exist",
		},
		{
			"circular dependency",
			[]config.Component{circularC1, c2},
			nil,
			"circular dependency detected in component list between c1, c2",
		},
		{
			"circular dependency 2",
			[]config.Component{circularC1, circularC2, circularC3},
			nil,
			"circular dependency detected in component list between c1, c2, c3",
		},
		{
			"circular dependency 3",
			[]config.Component{c6, c4, circularC1, c8, circularC2, circularC3},
			nil,
			"circular dependency detected in component list between c1, c2, c3",
		},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			sorted, err := config.SortComponents(tc.Components)
			if tc.Err == "" {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, sorted, test.ShouldResemble, tc.Expected)
			} else {
				test.That(t, sorted, test.ShouldBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tc.Err)
			}
		})
	}
}
