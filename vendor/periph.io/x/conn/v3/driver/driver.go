// Copyright 2016 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package driver devices a host peripheral driver to register when
// initializing.
//
// Drivers that can be automatically discovered should be registered in
// driverreg so discovery is done automatically.
package driver

// Impl is a host peripheral driver implementation.
type Impl interface {
	// String returns the name of the driver, as to be presented to the user.
	//
	// It must be unique in the list of registered drivers.
	String() string
	// Prerequisites returns a list of drivers that must be successfully loaded
	// first before attempting to load this driver.
	//
	// A driver listing a prerequisite not registered is a fatal failure at
	// initialization time.
	Prerequisites() []string
	// After returns a list of drivers that must be loaded first before
	// attempting to load this driver.
	//
	// Unlike Prerequisites(), this driver will still be attempted even if the
	// listed driver is missing or failed to load.
	//
	// This permits serialization without hard requirement.
	After() []string
	// Init initializes the driver.
	//
	// A driver may enter one of the three following state: loaded successfully,
	// was skipped as irrelevant on this host, failed to load.
	//
	// On success, it must return true, nil.
	//
	// When irrelevant (skipped), it must return false, errors.New(<reason>).
	//
	// On failure, it must return true, errors.New(<reason>). The failure must
	// state why it failed, for example an expected OS provided driver couldn't
	// be opened, e.g. /dev/gpiomem on Raspbian.
	Init() (bool, error)
}
