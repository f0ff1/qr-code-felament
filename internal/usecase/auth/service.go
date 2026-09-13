package auth

import (
	"errors"
	"fmt"
	"strings"

	"filamenttracker/internal/config"
	"filamenttracker/internal/domain"
	"filamenttracker/internal/infrastructure/session"

	"golang.org/x/crypto/bcrypt"
)

var ErrUnauthorized = fmt.Errorf("%w: unauthorized", domain.ErrInvalid)

type Service struct {
	username string
	hash     []byte
	sessions *session.Store
	ttl      config.Settings
	secure   bool
	cfg      config.Settings
}

func NewService(cfg config.Settings, store *session.Store) (*Service, error) {
	username := strings.TrimSpace(cfg.AdminUsername)
	if username == "" {
		username = "admin"
	}
	var hash []byte
	switch {
	case strings.TrimSpace(cfg.AdminPasswordHash) != "":
		hash = []byte(strings.TrimSpace(cfg.AdminPasswordHash))
	case strings.TrimSpace(cfg.AdminPassword) != "":
		generated, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("hash admin password: %w", err)
		}
		hash = generated
	default:
		if cfg.IsProduction() {
			return nil, fmt.Errorf("admin credentials required")
		}
		// development without password: auth middleware may be disabled
		hash = nil
	}
	if store == nil {
		store = session.NewStore(cfg.SessionTTL)
	}
	return &Service{
		username: username,
		hash:     hash,
		sessions: store,
		cfg:      cfg,
		secure:   cfg.CookieSecure,
	}, nil
}

func (s *Service) Enabled() bool {
	return s != nil && len(s.hash) > 0
}

func (s *Service) Login(username, password string) (session.Session, error) {
	if !s.Enabled() {
		return session.Session{}, fmt.Errorf("%w: auth is not configured", domain.ErrInvalid)
	}
	if !strings.EqualFold(strings.TrimSpace(username), s.username) {
		return session.Session{}, ErrUnauthorized
	}
	if err := bcrypt.CompareHashAndPassword(s.hash, []byte(password)); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return session.Session{}, ErrUnauthorized
		}
		return session.Session{}, err
	}
	return s.sessions.Create(s.username)
}

func (s *Service) Logout(sessionID string) {
	if s == nil || sessionID == "" {
		return
	}
	s.sessions.Delete(sessionID)
}

func (s *Service) Authenticate(sessionID string) (session.Session, error) {
	if !s.Enabled() {
		return session.Session{}, ErrUnauthorized
	}
	sess, ok := s.sessions.Get(sessionID)
	if !ok {
		return session.Session{}, ErrUnauthorized
	}
	s.sessions.Touch(sessionID)
	return sess, nil
}

func (s *Service) CookieSecure() bool {
	return s.secure
}

func (s *Service) SessionTTL() int {
	return int(s.cfg.SessionTTL.Seconds())
}

func (s *Service) Username() string {
	return s.username
}
