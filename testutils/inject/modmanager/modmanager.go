// Package injectmod contains an injectable module manager.
package injectmod

import (
	"context"

	"go.viam.com/rdk/module/modmanager"
	"go.viam.com/rdk/resource"
)

// ModuleManager is an injected ModuleManager.
type ModuleManager struct {
	*modmanager.Manager
	RemoveFunc func(modName string) ([]resource.Name, error)

	IsModularResourceFunc func(name resource.Name) bool

	CloseFunc func(ctx context.Context) error
}

// IsModularResource calls the injected IsModularResourceFunc or it will return false.
func (m *ModuleManager) IsModularResource(name resource.Name) bool {
	if m.IsModularResourceFunc == nil {
		return false
	}
	return m.IsModularResourceFunc(name)
}

// Remove calls the injected RemoveFunc or it will return an empty list.
func (m *ModuleManager) Remove(modName string) ([]resource.Name, error) {
	if m.RemoveFunc == nil {
		return []resource.Name{}, nil
	}
	return m.RemoveFunc(modName)
}

// Close calls the injected CloseFunc or it will return nil.
func (m *ModuleManager) Close(ctx context.Context) error {
	if m.CloseFunc == nil {
		return nil
	}
	return m.CloseFunc(ctx)
}
