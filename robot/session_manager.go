package robot

import (
	"context"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	pb "go.viam.com/api/robot/v1"
	"go.viam.com/utils"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/session"
)

// NewSessionManager creates a new manager for holding sessions.
func NewSessionManager(robot Robot, heartbeatWindow time.Duration) *SessionManager {
	cancelCtx, cancel := context.WithCancel(context.Background())
	m := &SessionManager{
		robot:             robot,
		heartbeatWindow:   heartbeatWindow,
		logger:            robot.Logger().Named("session_manager"),
		sessions:          map[uuid.UUID]*session.Session{},
		resourceToSession: map[resource.Name]uuid.UUID{},
		cancel:            cancel,
	}
	m.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() { m.expireLoop(cancelCtx) }, m.activeBackgroundWorkers.Done)
	return m
}

// SessionManager holds sessions for a particular robot and manages their
// lifetime.
type SessionManager struct {
	robot           Robot
	heartbeatWindow time.Duration
	logger          golog.Logger

	sessionResourceMu sync.RWMutex
	sessions          map[uuid.UUID]*session.Session

	resourceToSession map[resource.Name]uuid.UUID

	cancel                  func()
	activeBackgroundWorkers sync.WaitGroup
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
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()

	for {
		if !utils.SelectContextOrWaitChan(ctx, ticker.C) {
			return
		}

		now := time.Now()

		toDelete := map[uuid.UUID]struct{}{}
		var toStop []resource.Name
		m.sessionResourceMu.RLock()
		for id, sess := range m.sessions {
			if !sess.Active(now) {
				toDelete[id] = struct{}{}
			}
		}
		for res, sess := range m.resourceToSession {
			if _, ok := toDelete[sess]; ok {
				resCopy := res
				toStop = append(toStop, resCopy)
			}
		}
		m.sessionResourceMu.RUnlock()

		var resourceErrs []error
		func() {
			m.sessionResourceMu.Lock()
			defer m.sessionResourceMu.Unlock()
			for id := range toDelete {
				delete(m.sessions, id)
			}

			if len(toStop) == 0 {
				return
			}
			for _, resName := range toStop {
				func() {
					defer func() {
						if err := recover(); err != nil {
							resourceErrs = append(resourceErrs, errors.Errorf("panic stopping %q: %v", resName, err))
						}
					}()
					res, err := m.robot.ResourceByName(resName)
					if err != nil {
						resourceErrs = append(resourceErrs, err)
						return
					}

					if err := resource.StopResource(ctx, res, nil); err != nil {
						resourceErrs = append(resourceErrs, err)
					}
				}()
			}
		}()

		if len(toDelete) != 0 {
			m.logger.Debugw("sessions expired", "session_ids", toDelete)
		}
		if len(toStop) != 0 {
			m.logger.Debugw("tried to stop some resources", "resources", toStop)
		}
		if len(resourceErrs) != 0 {
			m.logger.Errorw("failed to stop some resources", "errors", resourceErrs)
		}
	}
}

const (
	defaultHeartbeatWindow = 500 * time.Millisecond
	maxHeartbeatWindow     = time.Minute
	maxSessions            = 1024
)

// Start creates a new session that expects at least one heartbeat within the configured window.
func (m *SessionManager) Start(
	ownerID string,
	peerConnInfo *pb.PeerConnectionInfo,
) (*session.Session, error) {
	sess := session.New(ownerID, peerConnInfo, m.heartbeatWindow, m.AssociateResource)
	m.sessionResourceMu.Lock()
	if len(m.sessions) > maxSessions {
		return nil, errors.New("too many concurrent sessions")
	}
	m.sessions[sess.ID()] = sess
	m.sessionResourceMu.Unlock()
	return sess, nil
}

// FindByID finds a session by the given ID. If found, a heartbeat is triggered,
// extending the lifetime of the session. If ownerID is in use but the session
// in question has a different owner, this is a security violation and we report
// back no session found.
func (m *SessionManager) FindByID(id uuid.UUID, ownerID string) (*session.Session, error) {
	m.sessionResourceMu.RLock()
	sess, ok := m.sessions[id]
	if !ok || !sess.CheckOwnerID(ownerID) {
		m.sessionResourceMu.RUnlock()
		return nil, session.ErrNoSession
	}
	m.sessionResourceMu.RUnlock()
	sess.Heartbeat()
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
	m.cancel()
	m.activeBackgroundWorkers.Wait()
}
