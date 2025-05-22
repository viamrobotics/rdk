// Copyright 2022 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package allwinner

import (
	"strings"
	"sync"

	"periph.io/x/host/v3/distro"
)

// Present detects whether the host CPU is an Allwinner CPU.
//
// https://en.wikipedia.org/wiki/Allwinner_Technology
func Present() bool {
	detection.do()
	return detection.isAllwinner
}

// IsR8 detects whether the host CPU is an Allwinner R8 CPU.
//
// It looks for the string "sun5i-r8" in /proc/device-tree/compatible.
func IsR8() bool {
	detection.do()
	return detection.isR8
}

// IsA20 detects whether the host CPU is an Allwinner A20 CPU.
//
// It first looks for the string "sun71-a20" in /proc/device-tree/compatible,
// and if that fails it checks for "Hardware : sun7i" in /proc/cpuinfo.
func IsA20() bool {
	detection.do()
	return detection.isA20
}

// IsA64 detects whether the host CPU is an Allwinner A64 CPU.
//
// It looks for the string "sun50iw1p1" in /proc/device-tree/compatible.
func IsA64() bool {
	detection.do()
	return detection.isA64
}

// IsH3 detects whether the host CPU is an Allwinner H3/H2+ Plus CPU.
//
// It looks for the string "sun8i-h2-plus" or "sun8i-h3" in /proc/device-tree/compatible.
func IsH3() bool {
	detection.do()
	return detection.isH3
}

// IsH5 detects whether the host CPU is an Allwinner H5 CPU.
//
// It looks for the string "sun50i-h5" in /proc/device-tree/compatible.
func IsH5() bool {
	detection.do()
	return detection.isH5
}

//

type detectionS struct {
	mu          sync.Mutex
	done        bool
	isAllwinner bool
	isR8        bool
	isA20       bool
	isA64       bool
	isH3        bool
	isH5        bool
}

var detection detectionS

// do contains the CPU detection logic that determines whether we have an
// Allwinner CPU and if so, which exact model.
//
// Sadly there is no science behind this, it's more of a trial and error using
// as many boards and OS flavors as possible.
func (d *detectionS) do() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.done {
		d.done = true
		if isArm {
			for _, c := range distro.DTCompatible() {
				if strings.Contains(c, "sun50iw1p1") {
					d.isA64 = true
				}
				if strings.Contains(c, "sun5i-r8") {
					d.isR8 = true
				}
				if strings.Contains(c, "sun7i-a20") {
					d.isA20 = true
				}
				// H2+ is a subtype of H3 and nearly compatible (only lacks GBit MAC and
				// 4k HDMI Output), so it is safe to map H2+ as an H3.
				if strings.Contains(c, "sun8i-h2-plus") || strings.Contains(c, "sun8i-h3") {
					d.isH3 = true
				}
				if strings.Contains(c, "sun50i-h5") {
					d.isH5 = true
				}
			}
			d.isAllwinner = d.isA64 || d.isR8 || d.isA20 || d.isH3 || d.isH5

			if !d.isAllwinner {
				// The kernel in the image that comes pre-installed on the pcDuino3 Nano
				// is an old 3.x kernel that doesn't expose the device-tree in procfs,
				// so do an extra check in cpuinfo as well if we haven't detected
				// anything yet.
				// Distros based on 4.x kernels do expose it.
				if hw, ok := distro.CPUInfo()["Hardware"]; ok {
					if hw == "sun7i" {
						d.isA20 = true
					}
				}
			}
		}
	}
}
