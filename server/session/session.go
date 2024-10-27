package session

import (
    "sync"
    "time"

    "github.com/google/uuid"
)

type Session struct {
    Username     string
    TargetDomain string
    LastActive   time.Time
    ClientIP     string // Added to track client IP
}

type SessionStore struct {
    sessions map[string]*Session
    mutex    sync.RWMutex
}

// NewSessionStore creates a new instance of SessionStore and starts the cleanup process.
func NewSessionStore() *SessionStore {
    store := &SessionStore{
        sessions: make(map[string]*Session),
    }
    go store.cleanupExpiredSessions()
    return store
}

// CreateSession creates a new session and returns the session token.
func (store *SessionStore) CreateSession(username, targetDomain, clientIP string) string {
    store.mutex.Lock()
    defer store.mutex.Unlock()

    sessionToken := uuid.New().String()
    store.sessions[sessionToken] = &Session{
        Username:     username,
        TargetDomain: targetDomain,
        LastActive:   time.Now(),
        ClientIP:     clientIP,
    }
    return sessionToken
}

// GetSession retrieves a session by its token.
func (store *SessionStore) GetSession(token string) (*Session, bool) {
    store.mutex.RLock()
    defer store.mutex.RUnlock()

    session, exists := store.sessions[token]
    return session, exists
}

// cleanupExpiredSessions removes sessions that have been inactive for more than 1 minute.
func (store *SessionStore) cleanupExpiredSessions() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    for range ticker.C {
        store.mutex.Lock()
        for token, session := range store.sessions {
            if time.Since(session.LastActive) > time.Minute {
                delete(store.sessions, token)
            }
        }
        store.mutex.Unlock()
    }
}