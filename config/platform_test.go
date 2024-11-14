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

	t.Run("apt-cache", func(t *testing.T) {
		jp5 := `Package: nvidia-jetpack
Version: 5.1.1-b56
Architecture: arm64
Maintainer: NVIDIA Corporation
Installed-Size: 194
Depends: nvidia-jetpack-runtime (= 5.1.1-b56), nvidia-jetpack-dev (= 5.1.1-b56)
Homepage: http://developer.nvidia.com/jetson
Priority: standard
Section: metapackages`
		match := aptCacheVersionRegex.FindSubmatch([]byte(jp5))
		test.That(t, match, test.ShouldNotBeNil)
		test.That(t, string(match[1]), test.ShouldResemble, "5")

		jp6 := `Package: nvidia-jetpack
Source: nvidia-jetpack (6.1)
Version: 6.1+b123
Architecture: arm64
Maintainer: NVIDIA Corporation
Installed-Size: 194
Depends: nvidia-jetpack-runtime (= 6.1+b123), nvidia-jetpack-dev (= 6.1+b123)
Homepage: http://developer.nvidia.com/jetson
Priority: standard
Section: metapackages`
		match = aptCacheVersionRegex.FindSubmatch([]byte(jp6))
		test.That(t, match, test.ShouldNotBeNil)
		test.That(t, string(match[1]), test.ShouldResemble, "6")
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
