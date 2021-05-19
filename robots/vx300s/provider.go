package vx300s

import (
	"fmt"
	"sync"

	"go.viam.com/core/config"
	"go.viam.com/core/robot"
)

// Provider TODO
type Provider struct {
	moveLock *sync.Mutex
}

// Ready TODO
func (p *Provider) Ready(r robot.Robot) error {
	return nil
}

// Reconfigure replaces this provider with the given provider.
func (p *Provider) Reconfigure(newProvider robot.Provider) {
	actual, ok := newProvider.(*Provider)
	if !ok {
		panic(fmt.Errorf("expected new provider to be %T but got %T", actual, newProvider))
	}
	*p = *actual
}

func getProviderOrCreate(r robot.MutableRobot) *Provider {
	p := r.ProviderByName("vx300s")
	if p == nil {
		p = &Provider{&sync.Mutex{}}
		r.AddProvider(p, config.Component{})
	}
	return p.(*Provider)
}
