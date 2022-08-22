// Package web contains the root of a web server.
package web

import "embed"

// AppFS is the embedded FS to control a robot with.
//
//go:embed runtime-shared
var AppFS embed.FS
