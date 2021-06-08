package xarm

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
	p := r.ProviderByName("xArm6")
	if p == nil {
		p = &Provider{&sync.Mutex{}}
		r.AddProvider(p, config.Component{})
	}
	return p.(*Provider)
}
