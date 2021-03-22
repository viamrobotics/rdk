package utils

import (
	"errors"
	"testing"

	"github.com/edaniels/test"
)

func TestParseFlags(t *testing.T) {
	t.Run("simple", func(t *testing.T) {
		test.That(t, ParseFlags(nil, nil), test.ShouldBeNil)
		test.That(t, ParseFlags([]string{"1"}, nil), test.ShouldBeNil)
		err := ParseFlags([]string{"1"}, 1)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "struct")
		s := struct{}{}
		err = ParseFlags([]string{"1"}, s)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "addressable")
		test.That(t, ParseFlags([]string{"1"}, &s), test.ShouldBeNil)
		var ps1 parseStruct1
		test.That(t, ParseFlags([]string{"1"}, &ps1), test.ShouldBeNil)
		test.That(t, ps1.A, test.ShouldEqual, "")
		test.That(t, ps1.b, test.ShouldEqual, "")
		var ps2 parseStruct2
		ps2Old := ps2
		test.That(t, ParseFlags([]string{"1"}, &ps2), test.ShouldBeNil)
		test.That(t, ps2, test.ShouldResemble, ps2Old)
		test.That(t, ParseFlags([]string{"1", "--a=5"}, &ps2), test.ShouldBeNil)
		test.That(t, ps2, test.ShouldNotResemble, ps2Old)
		test.That(t, ps2.A, test.ShouldEqual, "5")
		err = ParseFlags([]string{"1", "--a=5", "--b=6"}, &ps2)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "defined")
		test.That(t, ps2, test.ShouldNotResemble, ps2Old)
		test.That(t, ps2.A, test.ShouldEqual, "5")
		test.That(t, ps2.b, test.ShouldEqual, "")
		err = ParseFlags([]string{"1", "--a=5", "--c=foo"}, &ps2)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "parse")
		err = ParseFlags([]string{"1", "--a=5", "--d=foo"}, &ps2)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "parse")
	})

	t.Run("StringFlags", func(t *testing.T) {
		var ps3 parseStruct3
		test.That(t, ParseFlags([]string{"1", "--a=2", "--a=3", "--a=4"}, &ps3), test.ShouldBeNil)
		test.That(t, ps3, test.ShouldResemble, parseStruct3{
			A: StringFlags{"2", "3", "4"},
		})
	})

	t.Run("NetPortFlag", func(t *testing.T) {
		var psn parseStructNet
		test.That(t, ParseFlags([]string{"1", "--a=2"}, &psn), test.ShouldBeNil)
		test.That(t, psn, test.ShouldResemble, parseStructNet{
			A: NetPortFlag(2),
		})
		test.That(t, ParseFlags([]string{"1", "--a=0"}, &psn), test.ShouldBeNil)
		test.That(t, psn, test.ShouldResemble, parseStructNet{
			A: NetPortFlag(0),
		})
		test.That(t, ParseFlags([]string{"1"}, &psn), test.ShouldBeNil)
		test.That(t, psn, test.ShouldResemble, parseStructNet{
			A: NetPortFlag(5555),
		})
		err := ParseFlags([]string{"1", "--a=-1"}, &psn)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "syntax")
		err = ParseFlags([]string{"1", "--a=-10000"}, &psn)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "syntax")
	})

	t.Run("flagUnmarshaler", func(t *testing.T) {
		var ps4 parseStruct4
		test.That(t, ParseFlags([]string{"1", "--a=hey there", "--b=one", "--b=two"}, &ps4), test.ShouldBeNil)
		test.That(t, ps4.A.flagName, test.ShouldEqual, "a")
		test.That(t, ps4.A.val, test.ShouldEqual, "hey there")
		test.That(t, ps4.B, test.ShouldResemble, []uflagStruct{
			{"b", "one"},
			{"b", "two"},
		})

		var ps5 parseStruct5
		err := ParseFlags([]string{"1", "--a=hey there"}, &ps5)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")
	})

	t.Run("flag options", func(t *testing.T) {
		var ps6 parseStruct6
		err := ParseFlags([]string{"1", "--A=hey there"}, &ps6)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "Usage")
		err = ParseFlags([]string{"1", "--A=hey there", "--b=one"}, &ps6)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "Usage")
		err = ParseFlags([]string{"1", "--A=hey there", "--b=one", "--c=2"}, &ps6)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ps6.A, test.ShouldEqual, "hey there")
		test.That(t, ps6.B, test.ShouldEqual, "one")
		test.That(t, ps6.C, test.ShouldEqual, 2)
		test.That(t, ps6.D, test.ShouldEqual, 1)

		err = ParseFlags([]string{"1", "--a=hey there"}, &parseStruct7{})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "syntax")

		err = ParseFlags([]string{"1", "--a=hey there"}, &parseStruct8{})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "syntax")
	})
}

type parseStruct1 struct {
	A string
	b string //nolint
}

type parseStruct2 struct {
	A string `flag:"a"`
	b string `flag:"b"`
	C int    `flag:"c"`
	D bool   `flag:"d"`
}

type parseStruct3 struct {
	A StringFlags `flag:"a"`
}

type parseStruct4 struct {
	A uflagStruct   `flag:"a"`
	B []uflagStruct `flag:"b"`
}

type parseStruct5 struct {
	A uflagStructErr `flag:"a"`
}

type parseStruct6 struct {
	A string `flag:""`
	B string `flag:"b,required,default=foo,usage=hello alice"`
	C int    `flag:"c,required,usage=hello bob"`
	D int    `flag:"d,default=1,usage=hello charlie"`
}

type parseStruct7 struct {
	A int `flag:",default=foo"`
}

type parseStruct8 struct {
	A bool `flag:",default=foo"`
}

type parseStructNet struct {
	A NetPortFlag `flag:"a"`
}

type uflagStruct struct {
	flagName string
	val      string
}

func (ufs *uflagStruct) UnmarshalFlag(flagName, val string) error {
	ufs.flagName = flagName
	ufs.val = val
	return nil
}

type uflagStructErr struct {
}

func (ufse *uflagStructErr) UnmarshalFlag(flagName, val string) error {
	return errors.New("whoops")
}
