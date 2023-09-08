//go:build notc

package server

import (
	robotimpl "go.viam.com/rdk/robot/impl"
)

func createRobotOptions() []robotimpl.Option {
	return []robotimpl.Option{}
}
