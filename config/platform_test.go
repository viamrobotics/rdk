package config

import (
	"runtime"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
)

func TestReadExtendedPlatformTags(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("skipping platform tags test on non-linux")
	}
	logger := logging.NewTestLogger(t)
	tags := readExtendedPlatformTags(logger, true)
	test.That(t, len(tags), test.ShouldBeGreaterThanOrEqualTo, 2)
}

func TestAppendPairIfNonempty(t *testing.T) {
	arr := make([]string, 0, 1)
	arr = appendPairIfNonempty(arr, "x", "y")
	arr = appendPairIfNonempty(arr, "a", "")
	test.That(t, arr, test.ShouldResemble, []string{"x:y"})
}

func TestRegexes(t *testing.T) {
	t.Run("cuda", func(t *testing.T) {
		output := `nvcc: NVIDIA (R) Cuda compiler driver
Copyright (c) 2005-2021 NVIDIA Corporation
Built on Thu_Nov_18_09:45:30_PST_2021
Cuda compilation tools, release 11.5, V11.5.119
Build cuda_11.5.r11.5/compiler.30672275_0
`
		match := cudaRegex.FindSubmatch([]byte(output))
		test.That(t, match, test.ShouldNotBeNil)
		test.That(t, string(match[1]), test.ShouldResemble, "11")
	})

	t.Run("l4t-release", func(t *testing.T) {
		// The L4T major is parsed from the first line of /etc/nv_tegra_release and mapped to
		// the JetPack major. This sample is verbatim from a JetPack 6 Jetson Orin Nano.
		jp6 := "# R36 (release), REVISION: 4.4, GCID: 41062509, BOARD: generic, EABI: aarch64, DATE: Mon Jun 16 16:07:13 UTC 2025\n" +
			"# KERNEL_VARIANT: oot\nTARGET_USERSPACE_LIB_DIR=nvidia\n"
		match := l4tReleaseRegex.FindSubmatch([]byte(jp6))
		test.That(t, match, test.ShouldNotBeNil)
		test.That(t, l4tToJetpack[string(match[1])], test.ShouldEqual, "6")

		// JetPack 5 (L4T R35) and JetPack 4 (L4T R32).
		match = l4tReleaseRegex.FindSubmatch([]byte("# R35 (release), REVISION: 4.1, GCID: 12345, BOARD: generic\n"))
		test.That(t, match, test.ShouldNotBeNil)
		test.That(t, l4tToJetpack[string(match[1])], test.ShouldEqual, "5")

		match = l4tReleaseRegex.FindSubmatch([]byte("# R32 (release), REVISION: 7.1\n"))
		test.That(t, match, test.ShouldNotBeNil)
		test.That(t, l4tToJetpack[string(match[1])], test.ShouldEqual, "4")

		// JetPack 7 spans two L4T majors: R38 (JetPack 7.0/7.1) and R39 (JetPack 7.2).
		match = l4tReleaseRegex.FindSubmatch([]byte("# R38 (release), REVISION: 2.0, GCID: 67890, BOARD: generic\n"))
		test.That(t, match, test.ShouldNotBeNil)
		test.That(t, l4tToJetpack[string(match[1])], test.ShouldEqual, "7")

		match = l4tReleaseRegex.FindSubmatch([]byte("# R39 (release), REVISION: 2.1, GCID: 13579, BOARD: generic\n"))
		test.That(t, match, test.ShouldNotBeNil)
		test.That(t, l4tToJetpack[string(match[1])], test.ShouldEqual, "7")

		// A future/unknown L4T major parses but has no mapping (so no tag is emitted).
		match = l4tReleaseRegex.FindSubmatch([]byte("# R40 (release), REVISION: 0.0\n"))
		test.That(t, match, test.ShouldNotBeNil)
		_, ok := l4tToJetpack[string(match[1])]
		test.That(t, ok, test.ShouldBeFalse)

		// Non-Jetson / unparseable contents must not match.
		test.That(t, l4tReleaseRegex.FindSubmatch([]byte("")), test.ShouldBeNil)
		test.That(t, l4tReleaseRegex.FindSubmatch([]byte("not an nv_tegra_release file\n")), test.ShouldBeNil)
	})

	t.Run("pi", func(t *testing.T) {
		type Pair struct {
			a string
			b *piModel
		}
		// these strings come from running `strings start*.elf` in here:
		// https://github.com/raspberrypi/firmware/tree/master/boot
		pairs := []Pair{
			{"Raspberry Pi Compute Module Rev", &piModel{version: "1", longVersion: "cm1"}},
			{"Raspberry Pi Compute Module 2 Rev", &piModel{version: "2", longVersion: "cm2"}},
			{"Raspberry Pi Compute Module 3 Rev", &piModel{version: "3", longVersion: "cm3"}},
			{"Raspberry Pi Compute Module 3 Plus Rev", &piModel{version: "3", longVersion: "cm3p"}},
			{"Raspberry Pi Compute Module 4 Rev", &piModel{version: "4", longVersion: "cm4"}},
			{"Raspberry Pi Compute Module 4S Rev", &piModel{version: "4", longVersion: "cm4S"}},
			{"Raspberry Pi Compute Module 3E Rev", &piModel{version: "3", longVersion: "cm3E"}},
			{"Raspberry Pi Compute Module 5 Rev", &piModel{version: "5", longVersion: "cm5"}},
			{"Raspberry Pi Compute Module 5 Lite Rev", &piModel{version: "5", longVersion: "cm5l"}},

			{"Raspberry Pi Model A Plus Rev", &piModel{version: "1", longVersion: "1Ap"}},
			{"Raspberry Pi Model B Plus Rev", &piModel{version: "1", longVersion: "1Bp"}},
			{"Raspberry Pi 2 Model B Rev", &piModel{version: "2", longVersion: "2B"}},
			{"Raspberry Pi 3 Model B Rev", &piModel{version: "3", longVersion: "3B"}},
			{"Raspberry Pi 3 Model B Plus Rev", &piModel{version: "3", longVersion: "3Bp"}},
			{"Raspberry Pi 3 Model A Plus Rev", &piModel{version: "3", longVersion: "3Ap"}},
			{"Raspberry Pi 4 Model B Rev", &piModel{version: "4", longVersion: "4B"}},
			{"Raspberry Pi 5 Model B Rev", &piModel{version: "5", longVersion: "5B"}},
			{"Raspberry Pi Model A Rev", &piModel{version: "1", longVersion: "1A"}},
			{"Raspberry Pi Model B Rev", &piModel{version: "1", longVersion: "1B"}},
		}

		logger := logging.NewTestLogger(t)
		for _, pair := range pairs {
			parsed := parsePi(logger, []byte(pair.a))
			test.That(t, parsed, test.ShouldResemble, pair.b)
		}
	})
}
