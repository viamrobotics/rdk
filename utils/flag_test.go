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
		ps1Old := ps1
		test.That(t, ParseFlags([]string{"1"}, &ps1), test.ShouldBeNil)
		test.That(t, ps1, test.ShouldResemble, ps1Old)
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
	})

	t.Run("StringFlags", func(t *testing.T) {
		var ps3 parseStruct3
		test.That(t, ParseFlags([]string{"1", "--a=2", "--a=3", "--a=4"}, &ps3), test.ShouldBeNil)
		test.That(t, ps3, test.ShouldResemble, parseStruct3{
			A: StringFlags{"2", "3", "4"},
		})
	})

	t.Run("flagUnmarshaler", func(t *testing.T) {
		var ps4 parseStruct4
		test.That(t, ParseFlags([]string{"1", "--a=hey there"}, &ps4), test.ShouldBeNil)
		test.That(t, ps4.A.flagName, test.ShouldEqual, "a")
		test.That(t, ps4.A.val, test.ShouldEqual, "hey there")

		var ps5 parseStruct5
		err := ParseFlags([]string{"1", "--a=hey there"}, &ps5)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")
	})

	t.Run("flag options", func(t *testing.T) {

	})
}

type parseStruct1 struct {
	A string
	b string //nolint
}

type parseStruct2 struct {
	A string `flag:"a"`
	b string `flag:"b"`
}

type parseStruct3 struct {
	A StringFlags `flag:"a"`
}

type parseStruct4 struct {
	A uflagStruct `flag:"a"`
}

type parseStruct5 struct {
	A uflagStructErr `flag:"a"`
}

// TODO(erd): remove nolint
//nolint
type parseStruct6 struct {
	A string `flag:""`
	B string `flag:"b,required,default=foo,usage=hello world"`
	C int    `flag:"b,required,default=foo,usage=hello world"`
	D int    `flag:"b,required,default=foo,usage=hello world"`
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
