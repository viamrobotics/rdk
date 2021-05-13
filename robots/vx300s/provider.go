package vx300s

import (
	"sync"

	"go.viam.com/core/config"
	"go.viam.com/core/robot"
)

type Provider struct {
	moveLock *sync.Mutex
}

func (p *Provider) Ready(r robot.Robot) error {
	return nil
}

func getProviderOrCreate(r robot.MutableRobot) *Provider {
	p := r.ProviderByName("vx300s")
	if p == nil {
		p = &Provider{&sync.Mutex{}}
		r.AddProvider(p, config.Component{})
	}
	return p.(*Provider)
}
