//go:build !no_cgo || android

package server

import (
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/robot/web"
)

func createRobotOptions() []robotimpl.Option {
	return []robotimpl.Option{robotimpl.WithWebOptions(web.WithStreamConfig(makeStreamConfig()))}
}
