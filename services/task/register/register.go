// Package register registers the task service and its implementations.
package register

import (
	// register task.
	_ "go.viam.com/rdk/services/task"
	_ "go.viam.com/rdk/services/task/fake"
)
