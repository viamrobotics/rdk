// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package ice

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/pion/logging"
	"github.com/pion/stun"
)

func newCandidatePair(local, remote Candidate, controlling bool, logger logging.LeveledLogger) *CandidatePair {
	ret := &CandidatePair{
		iceRoleControlling: controlling,
		Remote:             remote,
		Local:              local,
		state:              CandidatePairStateWaiting,
	}
	go logSize(ret, logger)
	return ret
}

func logSize(cp *CandidatePair, logger logging.LeveledLogger) {
	for {
		time.Sleep(5 * time.Second)
		fmt.Printf("Candidate pair bandwidth log. From: %v:%d To: %v:%d Nominated: %v Bytes: %v\n",
			cp.Local.Address(), cp.Local.Port(),
			cp.Remote.Address(), cp.Remote.Port(),
			cp.nominated, cp.bytesSent.Load())
	}
}

// CandidatePair is a combination of a
// local and remote candidate
type CandidatePair struct {
	iceRoleControlling       bool
	Remote                   Candidate
	Local                    Candidate
	bindingRequestCount      uint16
	state                    CandidatePairState
	nominated                bool
	nominateOnBindingSuccess bool
	bytesSent                atomic.Int64
}

func (p *CandidatePair) String() string {
	if p == nil {
		return ""
	}

	return fmt.Sprintf("prio %d (local, prio %d) %s <-> %s (remote, prio %d), state: %s, nominated: %v, nominateOnBindingSuccess: %v",
		p.priority(), p.Local.Priority(), p.Local, p.Remote, p.Remote.Priority(), p.state, p.nominated, p.nominateOnBindingSuccess)
}

func (p *CandidatePair) equal(other *CandidatePair) bool {
	if p == nil && other == nil {
		return true
	}
	if p == nil || other == nil {
		return false
	}
	return p.Local.Equal(other.Local) && p.Remote.Equal(other.Remote)
}

// RFC 5245 - 5.7.2.  Computing Pair Priority and Ordering Pairs
// Let G be the priority for the candidate provided by the controlling
// agent.  Let D be the priority for the candidate provided by the
// controlled agent.
// pair priority = 2^32*MIN(G,D) + 2*MAX(G,D) + (G>D?1:0)
func (p *CandidatePair) priority() uint64 {
	var g, d uint32
	if p.iceRoleControlling {
		g = p.Local.Priority()
		d = p.Remote.Priority()
	} else {
		g = p.Remote.Priority()
		d = p.Local.Priority()
	}

	// Just implement these here rather
	// than fooling around with the math package
	min := func(x, y uint32) uint64 {
		if x < y {
			return uint64(x)
		}
		return uint64(y)
	}
	max := func(x, y uint32) uint64 {
		if x > y {
			return uint64(x)
		}
		return uint64(y)
	}
	cmp := func(x, y uint32) uint64 {
		if x > y {
			return uint64(1)
		}
		return uint64(0)
	}

	// 1<<32 overflows uint32; and if both g && d are
	// maxUint32, this result would overflow uint64
	return (1<<32-1)*min(g, d) + 2*max(g, d) + cmp(g, d)
}

func (p *CandidatePair) Write(b []byte) (int, error) {
	p.bytesSent.Add(int64(len(b)))
	return p.Local.writeTo(b, p.Remote)
}

func (a *Agent) sendSTUN(msg *stun.Message, local, remote Candidate) {
	_, err := local.writeTo(msg.Raw, remote)
	if err != nil {
		a.log.Tracef("Failed to send STUN message: %s", err)
	}
}
