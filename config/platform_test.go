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
}
