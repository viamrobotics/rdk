//go:build !no_cgo

package main

import (
	_ "go.viam.com/rdk/motionplan/ik/nloptik"   // registers the nlopt-backed gradient-descent IK solver
	_ "go.viam.com/rdk/services/motion/builtin" // this is special
)
