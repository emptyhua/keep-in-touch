package kit

import (
	"sync"
	"time"
)

type SessionManager struct {
	sync.RWMutex
	pool map[string]*Session
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		pool: make(map[string]*Session),
	}
}

func (m *SessionManager) GetSessionById(id string) *Session {
	m.RLock()
	defer m.RUnlock()
	if session, ok := m.pool[id]; ok {
		return session
	}
	return nil
}

func (m *SessionManager) createSession() *Session {
	m.Lock()
	defer m.Unlock()

	session := newSession(m)
	m.pool[session.Id] = session
	return session
}

func (m *SessionManager) removeSession(s *Session) {
	m.Lock()
	delete(m.pool, s.Id)
	m.Unlock()
}

func (m *SessionManager) CheckExpire() {
	for {
		// 20秒超时
		expiredTime := time.Now().Add(-20 * time.Second)
		for _, session := range m.pool {
			if !session.LostConnection.IsZero() && session.LostConnection.Before(expiredTime) {
				session.Close("lose connection and expired")
			}
		}
		time.Sleep(time.Second)
	}
}
