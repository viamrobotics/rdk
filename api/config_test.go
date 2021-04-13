package api

import (
	"testing"

	"github.com/edaniels/test"
	"github.com/mitchellh/mapstructure"
	"go.viam.com/robotcore/utils"

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

func TestComponentFlag(t *testing.T) {
	type MyStruct struct {
		Comp  Component `flag:"comp"`
		Comp2 Component `flag:"0"`
	}
	var myStruct MyStruct
	err := utils.ParseFlags([]string{"main", "--comp=foo"}, &myStruct)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "format")

	err = utils.ParseFlags([]string{"main", "--comp=host=foo"}, &myStruct)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "required")

	err = utils.ParseFlags([]string{"main", "--comp=type=foo,host=bar,attr=wee:woo"}, &myStruct)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, myStruct.Comp.Type, test.ShouldEqual, ComponentType("foo"))
	test.That(t, myStruct.Comp.Host, test.ShouldEqual, "bar")
	test.That(t, myStruct.Comp.Attributes, test.ShouldResemble, AttributeMap{
		"wee": "woo",
	})

	err = utils.ParseFlags([]string{"main", "type=foo,host=bar,attr=wee:woo"}, &myStruct)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, myStruct.Comp2.Type, test.ShouldEqual, ComponentType("foo"))
	test.That(t, myStruct.Comp2.Host, test.ShouldEqual, "bar")
	test.That(t, myStruct.Comp2.Attributes, test.ShouldResemble, AttributeMap{
		"wee": "woo",
	})
}

func TestParseComponentFlag(t *testing.T) {
	_, err := ParseComponentFlag("foo")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "format")

	_, err = ParseComponentFlag("host=foo")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "required")

	_, err = ParseComponentFlag("port=foo")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "syntax")

	comp, err := ParseComponentFlag("type=foo,host=bar,port=5,model=bar,name=baz,attr=wee:woo,subtype=who,attr=one:two")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, comp.Name, test.ShouldEqual, "baz")
	test.That(t, comp.Host, test.ShouldEqual, "bar")
	test.That(t, comp.Port, test.ShouldEqual, 5)
	test.That(t, comp.Type, test.ShouldEqual, ComponentType("foo"))
	test.That(t, comp.SubType, test.ShouldEqual, "who")
	test.That(t, comp.Model, test.ShouldEqual, "bar")
	test.That(t, comp.Attributes, test.ShouldResemble, AttributeMap{
		"wee": "woo",
		"one": "two",
	})
}

func TestCreateCloudRequest(t *testing.T) {
	cfg := CloudConfig{
		ID:     "a",
		Secret: "b",
		Path:   "c",
	}
	r, err := createRequest(cfg)
	if err != nil {
		t.Fatal(err)
	}

	test.That(t, r.Header.Get("Secret"), test.ShouldEqual, cfg.Secret)
	test.That(t, r.URL.String(), test.ShouldEqual, "c?id=a")
}
