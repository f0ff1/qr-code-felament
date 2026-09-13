package session

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type Session struct {
	ID        string
	Username  string
	ExpiresAt time.Time
}

type Store struct {
	mu      sync.RWMutex
	items   map[string]Session
	ttl     time.Duration
}

func NewStore(ttl time.Duration) *Store {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &Store{items: make(map[string]Session), ttl: ttl}
}

func (s *Store) Create(username string) (Session, error) {
	id, err := newID()
	if err != nil {
		return Session{}, err
	}
	sess := Session{
		ID:        id,
		Username:  username,
		ExpiresAt: time.Now().UTC().Add(s.ttl),
	}
	s.mu.Lock()
	s.items[id] = sess
	s.mu.Unlock()
	return sess, nil
}

func (s *Store) Get(id string) (Session, bool) {
	s.mu.RLock()
	sess, ok := s.items[id]
	s.mu.RUnlock()
	if !ok {
		return Session{}, false
	}
	if time.Now().UTC().After(sess.ExpiresAt) {
		s.Delete(id)
		return Session{}, false
	}
	return sess, true
}

func (s *Store) Touch(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.items[id]
	if !ok {
		return
	}
	sess.ExpiresAt = time.Now().UTC().Add(s.ttl)
	s.items[id] = sess
}

func (s *Store) Delete(id string) {
	s.mu.Lock()
	delete(s.items, id)
	s.mu.Unlock()
}

func newID() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
