package config

import (
	"runtime"
	"testing"

	"go.viam.com/test"
)

func TestReadExtendedPlatformTags(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("skipping platform tags test on non-linux")
	}
	tags := readExtendedPlatformTags(true)
	test.That(t, len(tags), test.ShouldBeGreaterThanOrEqualTo, 2)
}

func TestAppendPairIfNonempty(t *testing.T) {
	arr := make([]string, 0, 1)
	arr = appendPairIfNonempty(arr, "x", "y")
	arr = appendPairIfNonempty(arr, "a", "")
	test.That(t, arr, test.ShouldResemble, []string{"x:y"})
}

func TestCudaRegexes(t *testing.T) {
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

	t.Run("dpkg", func(t *testing.T) {
		output := `Package: libwebkit2gtk-4.0-37
Status: install ok installed
Priority: optional
Section: libs
Installed-Size: 81548
Maintainer: Ubuntu Developers <ubuntu-devel-discuss@lists.ubuntu.com>
Architecture: amd64
Multi-Arch: same
Source: webkit2gtk
Version: 2.46.1-0ubuntu0.22.04.3
Depends: libjavascriptcoregtk-4.0-18 (= 2.46.1-0ubuntu0.22.04.3), gstreamer1.0-plugins-base
`
		match := dpkgVersionRegex.FindSubmatch([]byte(output))
		test.That(t, match, test.ShouldNotBeNil)
		test.That(t, string(match[1]), test.ShouldResemble, "2")
	})
}
