// Copyright 2016 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

//go:build !linux
// +build !linux

package sysfs

const isLinux = false

func isErrBusy(err error) bool {
	// This function is not used on non-linux.
	return false
}
