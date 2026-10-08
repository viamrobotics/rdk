package robot

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/internal/actuatorstop"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/session"
)

// NewSessionManager creates a new manager for holding sessions.
func NewSessionManager(robot Robot, heartbeatWindow time.Duration) *SessionManager {
	m := &SessionManager{
		robot:             robot,
		heartbeatWindow:   heartbeatWindow,
		logger:            robot.Logger().Sublogger("networking.session_manager"),
		sessions:          map[uuid.UUID]*session.Session{},
		resourceToSession: map[resource.Name]uuid.UUID{},
	}
	m.workers = utils.NewBackgroundStoppableWorkers(m.expireLoop)
	return m
}

// SessionManager holds sessions for a particular robot and manages their
// lifetime.
type SessionManager struct {
	robot           Robot
	heartbeatWindow time.Duration
	logger          logging.Logger

	sessionResourceMu sync.RWMutex
	sessions          map[uuid.UUID]*session.Session

	resourceToSession map[resource.Name]uuid.UUID

	workers *utils.StoppableWorkers
}

// All returns all active sessions.
func (m *SessionManager) All() []*session.Session {
	m.sessionResourceMu.RLock()
	defer m.sessionResourceMu.RUnlock()
	sessions := make([]*session.Session, 0, len(m.sessions))
	for _, sess := range m.sessions {
		sessions = append(sessions, sess)
	}
	return sessions
}

func (m *SessionManager) expireLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		if !utils.SelectContextOrWaitChan(ctx, ticker.C) {
			return
		}

		now := time.Now()

		toDelete := map[uuid.UUID]*session.Session{}
		stoppedBySession := map[uuid.UUID][]string{}
		var toStop []resource.Name
		m.sessionResourceMu.RLock()
		for id, sess := range m.sessions {
			if !sess.Active(now) {
				toDelete[id] = sess
			}
		}
		for res, sess := range m.resourceToSession {
			if _, ok := toDelete[sess]; ok {
				resCopy := res
				toStop = append(toStop, resCopy)
				stoppedBySession[sess] = append(stoppedBySession[sess], res.String())
			}
		}
		m.sessionResourceMu.RUnlock()

		m.sessionResourceMu.Lock()
		for id := range toDelete {
			delete(m.sessions, id)
		}
		m.sessionResourceMu.Unlock()

		if len(toStop) != 0 {
			m.workers.Add(func(ctx context.Context) {
				m.stopResources(ctx, toStop)
			})
		}

		if len(toDelete) != 0 {
			var deletedIDs []string
			for id, sess := range toDelete {
				deletedIDs = append(deletedIDs, id.String())
				pcInfo := sess.PeerConnectionInfo()
				m.logger.Activity("liveness", "expired",
					"session_id", id.String(),
					"connection_type", pcInfo.GetType().String(),
					"remote_address", pcInfo.GetRemoteAddress(),
					"stopped_resources", stoppedBySession[id])
			}
			m.logger.CDebugw(ctx, "sessions expired", "session_ids", deletedIDs)
		}
	}
}

// stopResources stops the actuators among toStop without holding sessionResourceMu, so a slow
// stop cannot stall heartbeats or the expiry of other sessions.
func (m *SessionManager) stopResources(ctx context.Context, toStop []resource.Name) {
	remoteNames := map[resource.Name]bool{}
	for _, name := range m.robot.ResourceNames() {
		if name.ContainsRemoteNames() {
			remoteNames[resource.Name{API: name.API, Name: name.Name}] = true
		}
	}

	var resourceErrs []error
	local := map[resource.Name]resource.Actuator{}
	remote := map[resource.Name]resource.Actuator{}
	for _, resName := range toStop {
		res, err := m.robot.ResourceByName(resName)
		if err != nil {
			// A closing robot removes its resources, which is not a failure to stop.
			if !resource.IsNotFoundError(err) || ctx.Err() == nil {
				resourceErrs = append(resourceErrs, err)
			}
			continue
		}

		actuator, ok := res.(resource.Actuator)
		if !ok {
			continue
		}
		if remoteNames[resource.Name{API: resName.API, Name: resName.Name}] {
			remote[resName] = actuator
		} else {
			local[resName] = actuator
		}
	}
	for _, err := range actuatorstop.Stop(ctx, local, remote, nil) {
		resourceErrs = append(resourceErrs, err)
	}

	m.logger.CDebugw(ctx, "tried to stop some resources", "resources", toStop)
	if len(resourceErrs) != 0 {
		m.logger.CErrorw(ctx, "failed to stop some resources", "errors", resourceErrs)
	}
}

const (
	maxSessions = 1024
)

// Start creates a new session that expects at least one heartbeat within the configured window.
func (m *SessionManager) Start(ctx context.Context, ownerID string) (*session.Session, error) {
	sess := session.New(ctx, ownerID, m.heartbeatWindow, m.AssociateResource)
	m.sessionResourceMu.Lock()
	defer m.sessionResourceMu.Unlock()
	if len(m.sessions) > maxSessions {
		return nil, errors.New("too many concurrent sessions")
	}
	m.sessions[sess.ID()] = sess
	return sess, nil
}

// FindByID finds a session by the given ID. If found, a heartbeat is triggered,
// extending the lifetime of the session. If ownerID is in use but the session
// in question has a different owner, this is a security violation and we report
// back no session found.
func (m *SessionManager) FindByID(ctx context.Context, id uuid.UUID, ownerID string) (*session.Session, error) {
	m.sessionResourceMu.RLock()
	sess, ok := m.sessions[id]
	if !ok || !sess.CheckOwnerID(ownerID) {
		m.sessionResourceMu.RUnlock()
		return nil, session.ErrNoSession
	}
	m.sessionResourceMu.RUnlock()
	sess.Heartbeat(ctx)
	return sess, nil
}

// AssociateResource associates a session ID to a monitored resource such that
// when a session expires, if a resource is currently associated with that ID
// based on the order of AssociateResource calls, then it will have its resourc
// stopped. If id is uuid.Nil, this has no effect other than disassociation with
// a session. Be sure to include any remote information in the name.
func (m *SessionManager) AssociateResource(id uuid.UUID, resourceName resource.Name) {
	m.sessionResourceMu.Lock()
	m.resourceToSession[resourceName] = id
	m.sessionResourceMu.Unlock()
}

// Close stops the session manager but will not explicitly expire any sessions.
func (m *SessionManager) Close() {
	m.workers.Stop()
}
