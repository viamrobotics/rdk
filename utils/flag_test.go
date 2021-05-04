package utils

import (
	"testing"

	"go.viam.com/test"
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
		err = ParseFlags([]string{"1", "--a=100000"}, &psn)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "range")

		test.That(t, ParseFlags([]string{"1", "--b=2", "--b=3"}, &psn), test.ShouldBeNil)
		test.That(t, psn, test.ShouldResemble, parseStructNet{
			A: NetPortFlag(5555),
			B: []NetPortFlag{2, 3},
		})

		err = ParseFlags([]string{"1", "--b=2", "--b=foo"}, &psn)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "syntax")

		err = ParseFlags([]string{"1", "--b=2", "--b=100000"}, &psn)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "range")

		psn = parseStructNet{}
		test.That(t, ParseFlags([]string{"1", "--d=2"}, &psn), test.ShouldBeNil)
		nf := NetPortFlag(2)
		test.That(t, psn, test.ShouldResemble, parseStructNet{
			A: NetPortFlag(5555),
			D: &nf,
		})
		err = ParseFlags([]string{"1", "--d=foo"}, &psn)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "syntax")
	})

	t.Run("flag options", func(t *testing.T) {
		var ps3 parseStruct3
		err := ParseFlags([]string{"1", "--A=hey there"}, &ps3)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "Usage")
		err = ParseFlags([]string{"1", "--A=hey there", "--b=one"}, &ps3)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "Usage")
		err = ParseFlags([]string{"1", "--A=hey there", "--b=one", "--c=2"}, &ps3)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ps3.A, test.ShouldEqual, "hey there")
		test.That(t, ps3.B, test.ShouldEqual, "one")
		test.That(t, ps3.C, test.ShouldEqual, 2)
		test.That(t, ps3.D, test.ShouldEqual, 1)
		test.That(t, ps3.E, test.ShouldEqual, "whatisup")

		err = ParseFlags([]string{"1", "--a=hey there"}, &parseStruct4{})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "syntax")

		err = ParseFlags([]string{"1", "--a=hey there"}, &parseStruct5{})
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

type stringFlag string

func (sf *stringFlag) String() string {
	return string(*sf)
}

func (sf *stringFlag) Set(val string) error {
	if val != "" {
		*sf = stringFlag(val)
		return nil
	}
	*sf = stringFlag("whatisup")
	return nil
}

func (sf *stringFlag) Get() interface{} {
	return string(*sf)
}

type parseStruct3 struct {
	A string     `flag:""`
	B string     `flag:"b,required,default=foo,usage=hello alice"`
	C int        `flag:"c,required,usage=hello bob"`
	D int        `flag:"d,default=1,usage=hello charlie"`
	E stringFlag `flag:"e,default="`
}

type parseStruct4 struct {
	A int `flag:",default=foo"`
}

type parseStruct5 struct {
	A bool `flag:",default=foo"`
}

type parseStructNet struct {
	A NetPortFlag   `flag:"a,default=5555"`
	B []NetPortFlag `flag:"b"`
	C NetPortFlag   `flag:"c"`
	D *NetPortFlag  `flag:"d"`
}
