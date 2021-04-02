// +build !linux,!darwin

package search

import "go.viam.com/robotcore/api"

func Devices() []api.Component {
	return nil
}
