package session

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Session struct {
	ID           string
	UserID       uuid.UUID
	Username     string
	Role         string
	SiteIDs      []uuid.UUID
	ActiveSiteID uuid.UUID
	ExpiresAt    time.Time
}

type Store struct {
	mu    sync.RWMutex
	items map[string]Session
	ttl   time.Duration
}

func NewStore(ttl time.Duration) *Store {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &Store{items: make(map[string]Session), ttl: ttl}
}

type CreateInput struct {
	UserID       uuid.UUID
	Username     string
	Role         string
	SiteIDs      []uuid.UUID
	ActiveSiteID uuid.UUID
}

func (s *Store) Create(in CreateInput) (Session, error) {
	id, err := newID()
	if err != nil {
		return Session{}, err
	}
	sites := append([]uuid.UUID(nil), in.SiteIDs...)
	sess := Session{
		ID:           id,
		UserID:       in.UserID,
		Username:     in.Username,
		Role:         in.Role,
		SiteIDs:      sites,
		ActiveSiteID: in.ActiveSiteID,
		ExpiresAt:    time.Now().UTC().Add(s.ttl),
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

func (s *Store) SetActiveSite(id string, siteID uuid.UUID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.items[id]
	if !ok {
		return false
	}
	sess.ActiveSiteID = siteID
	sess.ExpiresAt = time.Now().UTC().Add(s.ttl)
	s.items[id] = sess
	return true
}

func (s *Store) Delete(id string) {
	s.mu.Lock()
	delete(s.items, id)
	s.mu.Unlock()
}

func (s *Store) DeleteByUserID(userID uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, sess := range s.items {
		if sess.UserID == userID {
			delete(s.items, id)
		}
	}
}

func newID() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
