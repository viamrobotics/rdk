package vx300s

import (
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

func getProviderOrCreate(r robot.MutableRobot) *Provider {
	p, ok := r.ProviderByName("vx300s")
	if !ok {
		p = &Provider{&sync.Mutex{}}
		r.AddProvider(p, config.Component{})
	}
	return p.(*Provider)
}
