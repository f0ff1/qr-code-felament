package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"

	"filamenttracker/internal/config"
	"filamenttracker/internal/domain"
	orgdomain "filamenttracker/internal/domain/org"
	userdomain "filamenttracker/internal/domain/user"
	"filamenttracker/internal/infrastructure/session"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var ErrUnauthorized = fmt.Errorf("%w: unauthorized", domain.ErrInvalid)

type Notifier interface {
	Publish(eventType, message string, payload map[string]any)
}

type Service struct {
	users    UserRepository
	resets   PasswordResetRepository
	sessions *session.Store
	cfg      config.Settings
	secure   bool
	notifier Notifier

	loginMu    sync.Mutex
	loginFails map[string]loginAttempt
}

type loginAttempt struct {
	count   int
	blocked time.Time
}

func NewService(cfg config.Settings, store *session.Store, users UserRepository, resets PasswordResetRepository, notifier Notifier) (*Service, error) {
	if store == nil {
		store = session.NewStore(cfg.SessionTTL)
	}
	if users == nil {
		return nil, fmt.Errorf("user repository required")
	}
	return &Service{
		users:      users,
		resets:     resets,
		sessions:   store,
		cfg:        cfg,
		secure:     cfg.CookieSecure || cfg.IsProduction(),
		notifier:   notifier,
		loginFails: make(map[string]loginAttempt),
	}, nil
}

// BootstrapAdmin ensures at least one admin exists (from env credentials).
func (s *Service) BootstrapAdmin(ctx context.Context) error {
	n, err := s.users.Count(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	username := strings.TrimSpace(s.cfg.AdminUsername)
	if username == "" {
		username = "admin"
	}
	password := strings.TrimSpace(s.cfg.AdminPassword)
	if password == "" && strings.TrimSpace(s.cfg.AdminPasswordHash) == "" {
		if s.cfg.IsProduction() {
			return fmt.Errorf("ADMIN_PASSWORD or ADMIN_PASSWORD_HASH required to bootstrap first admin")
		}
		password = "admin"
	}
	var hash []byte
	if strings.TrimSpace(s.cfg.AdminPasswordHash) != "" {
		hash = []byte(strings.TrimSpace(s.cfg.AdminPasswordHash))
	} else {
		hash, err = bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
	}
	now := time.Now().UTC()
	u := userdomain.User{
		ID:             uuid.New(),
		OrganizationID: orgdomain.DefaultOrganizationID,
		Username:       username,
		PasswordHash:   string(hash),
		FirstName:      "Admin",
		LastName:       "",
		Role:           userdomain.RoleAdmin,
		IsActive:       true,
		SiteIDs:        []uuid.UUID{orgdomain.DefaultSiteID},
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.users.Create(ctx, u); err != nil {
		return err
	}
	return s.users.SetSites(ctx, u.ID, u.SiteIDs)
}

func (s *Service) Enabled() bool { return true }

func (s *Service) Login(ctx context.Context, username, password string) (session.Session, error) {
	username = strings.TrimSpace(username)
	key := strings.ToLower(username)
	if err := s.checkLoginRate(key); err != nil {
		return session.Session{}, err
	}
	u, err := s.users.GetByUsername(ctx, username)
	if err != nil {
		s.recordLoginFail(key)
		return session.Session{}, ErrUnauthorized
	}
	if !u.IsActive {
		s.recordLoginFail(key)
		return session.Session{}, ErrUnauthorized
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		s.recordLoginFail(key)
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return session.Session{}, ErrUnauthorized
		}
		return session.Session{}, err
	}
	s.clearLoginFail(key)
	active := orgdomain.DefaultSiteID
	if u.Role.IsAdmin() {
		if len(u.SiteIDs) > 0 {
			active = u.SiteIDs[0]
		}
	} else if len(u.SiteIDs) > 0 {
		active = u.SiteIDs[0]
	} else {
		return session.Session{}, fmt.Errorf("%w: user has no site assignment", domain.ErrInvalid)
	}
	return s.sessions.Create(session.CreateInput{
		UserID:       u.ID,
		Username:     u.Username,
		Role:         string(u.Role),
		SiteIDs:      u.SiteIDs,
		ActiveSiteID: active,
	})
}

func (s *Service) Logout(sessionID string) {
	if sessionID == "" {
		return
	}
	s.sessions.Delete(sessionID)
}

func (s *Service) Authenticate(sessionID string) (session.Session, error) {
	sess, ok := s.sessions.Get(sessionID)
	if !ok {
		return session.Session{}, ErrUnauthorized
	}
	s.sessions.Touch(sessionID)
	return sess, nil
}

func (s *Service) SetActiveSite(sessionID string, siteID uuid.UUID, sess session.Session) error {
	if sess.Role != string(userdomain.RoleAdmin) {
		allowed := false
		for _, id := range sess.SiteIDs {
			if id == siteID {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("%w: site not allowed", domain.ErrInvalid)
		}
	}
	if !s.sessions.SetActiveSite(sessionID, siteID) {
		return ErrUnauthorized
	}
	return nil
}

func (s *Service) RequestPasswordReset(ctx context.Context, username string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return fmt.Errorf("%w: username required", domain.ErrInvalid)
	}
	u, err := s.users.GetByUsername(ctx, username)
	if err != nil {
		// Do not reveal whether user exists.
		return nil
	}
	if s.resets == nil {
		return nil
	}
	req := userdomain.PasswordResetRequest{
		ID:        uuid.New(),
		UserID:    u.ID,
		Status:    "open",
		CreatedAt: time.Now().UTC(),
	}
	if err := s.resets.Create(ctx, req); err != nil {
		return err
	}
	if s.notifier != nil {
		s.notifier.Publish("password_reset_request",
			fmt.Sprintf("Запрос сброса пароля: %s (%s)", u.Username, u.DisplayName()),
			map[string]any{
				"user_id":  u.ID.String(),
				"username": u.Username,
				"request_id": req.ID.String(),
			})
	}
	return nil
}

func (s *Service) CookieSecure() bool { return s.secure }

func (s *Service) SessionTTL() int { return int(s.cfg.SessionTTL.Seconds()) }

func (s *Service) InvalidateUserSessions(userID uuid.UUID) {
	s.sessions.DeleteByUserID(userID)
}

func HashPassword(password string) (string, error) {
	if err := ValidatePasswordStrength(password); err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func ValidatePasswordStrength(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("%w: password must be at least 8 characters", domain.ErrInvalid)
	}
	var letter, digit bool
	for _, r := range password {
		if unicode.IsLetter(r) {
			letter = true
		}
		if unicode.IsDigit(r) {
			digit = true
		}
	}
	if !letter || !digit {
		return fmt.Errorf("%w: password must include letters and digits", domain.ErrInvalid)
	}
	return nil
}

func GeneratePassword() (string, error) {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	// Ensure mix: prefix letter+digit pattern via hex (has digits) + "Aa"
	return "Aa" + hex.EncodeToString(buf), nil
}

func (s *Service) checkLoginRate(key string) error {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	a := s.loginFails[key]
	if !a.blocked.IsZero() && time.Now().Before(a.blocked) {
		return fmt.Errorf("%w: too many login attempts, try later", domain.ErrInvalid)
	}
	return nil
}

func (s *Service) recordLoginFail(key string) {
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	a := s.loginFails[key]
	a.count++
	if a.count >= 5 {
		a.blocked = time.Now().Add(60 * time.Second)
		a.count = 0
	}
	s.loginFails[key] = a
}

func (s *Service) clearLoginFail(key string) {
	s.loginMu.Lock()
	delete(s.loginFails, key)
	s.loginMu.Unlock()
}
