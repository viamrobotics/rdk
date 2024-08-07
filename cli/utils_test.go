//nolint:lll
package cli

import (
	"testing"

	"go.viam.com/test"
)

func TestSamePath(t *testing.T) {
	equal, _ := samePath("/x", "/x")
	test.That(t, equal, test.ShouldBeTrue)
	equal, _ = samePath("/x", "x")
	test.That(t, equal, test.ShouldBeFalse)
}

func TestGetMapString(t *testing.T) {
	m := map[string]any{
		"x": "x",
		"y": 10,
	}
	test.That(t, getMapString(m, "x"), test.ShouldEqual, "x")
	test.That(t, getMapString(m, "y"), test.ShouldEqual, "")
	test.That(t, getMapString(m, "z"), test.ShouldEqual, "")
}

func TestParseFileType(t *testing.T) {
	pairs := [][]string{
		{"linux/amd64", `filename: ELF 64-bit LSB executable, x86-64, version 1 (SYSV), statically linked, Go BuildID=2VnfLaDNwi7mhCjdDkAr/lpOa21AkXD_n1ZOzOBaE/WQvWVRjuvto6MgjwqbQ3/hja5tmvEfcE09ZLPl819, with debug_info, not stripped`},
		{"linux/arm64", `/path/to: ELF 64-bit LSB executable, ARM aarch64, version 1 (SYSV), statically linked, Go BuildID=xswztQyDYn9C34kIHH1c/al0YUQI7PfmFrMDS910o/BRt84DHJJKOoz3JFShfc/gCDP6LgcLRWk5l2TpxbR, with debug_info, not stripped`},
		{"linux/arm", `file: ELF 32-bit LSB executable, ARM, EABI5 version 1 (SYSV), statically linked, Go BuildID=1TRJ7vRfAd6gwe6x0c6d/m6JcXHPRiWLykXbmUtO5/vMbl6w2O72ILWCBSPVF3/l3HWqdJgAaP46rzUna4Y, with debug_info, not stripped`},
		{"darwin/amd64", `/bin/whatever: Mach-O 64-bit x86_64 executable`},
		{"darwin/arm64", `x/y/z: Mach-O 64-bit arm64 executable, flags:<|DYLDLINK|PIE>`},
	}
	for _, pair := range pairs {
		test.That(t, ParseFileType(pair[1]), test.ShouldResemble, pair[0])
	}
}
