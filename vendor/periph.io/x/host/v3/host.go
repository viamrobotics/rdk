// Copyright 2016 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package host

import (
	"periph.io/x/conn/v3/driver/driverreg"
	// TODO(maruel): For now do not include ftdi by default. It's not stable
	// enough to warrant being included.
	// _ "periph.io/x/host/v3/ftdi"
)

// Init calls driverreg.Init() and returns it as-is.
//
// The only difference is that by calling host.Init(), you are guaranteed to
// have all the host drivers implemented in this library to be implicitly
// loaded.
func Init() (*driverreg.State, error) {
	return driverreg.Init()
}
