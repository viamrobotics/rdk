// Copyright 2018 The Periph Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// This file contains the parallelized driver loading logic. It is meant to be
// load the drivers as fast as possible by parallelising work.

//go:build !tinygo
// +build !tinygo

package driverreg

import (
	"errors"
	"strconv"
	"sync"

	"periph.io/x/conn/v3/driver"
)

func initImpl() (*State, error) {
	state = &State{}
	// At this point, byName is guaranteed to be immutable.
	cD := make(chan driver.Impl)
	cS := make(chan DriverFailure)
	cE := make(chan DriverFailure)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for d := range cD {
			state.Loaded = insertDriver(state.Loaded, d)
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for f := range cS {
			state.Skipped = insertDriverFailure(state.Skipped, f)
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for f := range cE {
			state.Failed = insertDriverFailure(state.Failed, f)
		}
	}()

	stages, err := explodeStages()
	if err != nil {
		return state, err
	}
	loaded := sync.Map{}
	for _, s := range stages {
		// It's very important that each of the stage is fully completed before the
		// next one is attempted.
		s.loadParallel(&loaded, cD, cS, cE)
	}
	close(cD)
	close(cS)
	close(cE)
	wg.Wait()
	return state, nil
}

// loadParallel loads all the drivers for this stage in parallel and returns
// once they are all loaded.
func (s *stage) loadParallel(loaded *sync.Map, cD chan<- driver.Impl, cS, cE chan<- DriverFailure) {
	success := make(chan string)
	go func() {
		defer close(success)
		wg := sync.WaitGroup{}
	loop:
		for name, drv := range s.drvs {
			// Intentionally do not look at After(), only Prerequisites().
			for _, dep := range drv.Prerequisites() {
				if _, ok := loaded.Load(dep); !ok {
					cS <- DriverFailure{drv, errors.New("dependency not loaded: " + strconv.Quote(dep))}
					continue loop
				}
			}

			// Not skipped driver, attempt loading in a goroutine.
			wg.Add(1)
			go func(n string, d driver.Impl) {
				defer wg.Done()
				if ok, err := d.Init(); ok {
					if err == nil {
						cD <- d
						success <- n
						return
					}
					cE <- DriverFailure{d, err}
				} else {
					cS <- DriverFailure{d, err}
				}
			}(name, drv)
		}
		wg.Wait()
	}()
	for s := range success {
		loaded.Store(s, nil)
	}
}
