package actions

import (
	"fmt"

	"go.viam.com/robotcore/api"
)

type Action func(r api.Robot)

var (
	actionRegistry = map[string]Action{}
)

func RegisterAction(name string, action Action) {
	_, old := actionRegistry[name]
	if old {
		panic(fmt.Errorf("trying to register 2 actions with the same name (%s)", name))
	}
	actionRegistry[name] = action
}

func LookupAction(name string) Action {
	return actionRegistry[name]
}

func AllActionNames() []string {
	names := []string{}
	for k := range actionRegistry {
		names = append(names, k)
	}
	return names
}
