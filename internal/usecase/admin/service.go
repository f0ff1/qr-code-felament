package admin

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"filamenttracker/internal/domain"
	orgdomain "filamenttracker/internal/domain/org"
	sitedomain "filamenttracker/internal/domain/site"
	userdomain "filamenttracker/internal/domain/user"
	authusecase "filamenttracker/internal/usecase/auth"

	"github.com/google/uuid"
)

type OrgRepository interface {
	EnsureDefault(ctx context.Context) error
	GetDefault(ctx context.Context) (orgdomain.Organization, error)
	GetByID(ctx context.Context, id uuid.UUID) (orgdomain.Organization, error)
	FindByName(ctx context.Context, name string) (orgdomain.Organization, error)
	Update(ctx context.Context, o orgdomain.Organization) error
	Upsert(ctx context.Context, o orgdomain.Organization) error
}

type SiteRepository interface {
	EnsureDefault(ctx context.Context) error
	Create(ctx context.Context, s sitedomain.Site) error
	Update(ctx context.Context, s sitedomain.Site) error
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]sitedomain.Site, error)
	ListAll(ctx context.Context) ([]sitedomain.Site, error)
	GetByID(ctx context.Context, id uuid.UUID) (sitedomain.Site, error)
}

type UserRepository interface {
	Create(ctx context.Context, u userdomain.User) error
	Update(ctx context.Context, u userdomain.User) error
	GetByID(ctx context.Context, id uuid.UUID) (userdomain.User, error)
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]userdomain.User, error)
	ListAll(ctx context.Context) ([]userdomain.User, error)
	SetSites(ctx context.Context, userID uuid.UUID, siteIDs []uuid.UUID) error
}

type ResetRepository interface {
	ListOpen(ctx context.Context) ([]userdomain.PasswordResetRequest, error)
	Resolve(ctx context.Context, id uuid.UUID) error
	GetByID(ctx context.Context, id uuid.UUID) (userdomain.PasswordResetRequest, error)
}

type SessionInvalidator interface {
	InvalidateUserSessions(userID uuid.UUID)
}

type Service struct {
	orgs     OrgRepository
	sites    SiteRepository
	users    UserRepository
	resets   ResetRepository
	sessions SessionInvalidator
}

func NewService(orgs OrgRepository, sites SiteRepository, users UserRepository, resets ResetRepository, sessions SessionInvalidator) *Service {
	return &Service{orgs: orgs, sites: sites, users: users, resets: resets, sessions: sessions}
}

type CreateSiteInput struct {
	OrganizationName string
	Name             string
	Address          string
}

func (s *Service) EnsureDefaults(ctx context.Context) error {
	if err := s.orgs.EnsureDefault(ctx); err != nil {
		return err
	}
	return s.sites.EnsureDefault(ctx)
}

func (s *Service) ListSites(ctx context.Context) ([]sitedomain.Site, error) {
	return s.sites.ListAll(ctx)
}

func (s *Service) resolveOrganization(ctx context.Context, name string) (orgdomain.Organization, error) {
	if err := s.orgs.EnsureDefault(ctx); err != nil {
		return orgdomain.Organization{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return s.orgs.GetDefault(ctx)
	}
	existing, err := s.orgs.FindByName(ctx, name)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return orgdomain.Organization{}, err
	}
	org := orgdomain.New(name)
	if err := s.orgs.Upsert(ctx, org); err != nil {
		return orgdomain.Organization{}, err
	}
	return org, nil
}

func (s *Service) CreateSite(ctx context.Context, in CreateSiteInput) (sitedomain.Site, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return sitedomain.Site{}, fmt.Errorf("%w: site name required", domain.ErrInvalid)
	}
	org, err := s.resolveOrganization(ctx, in.OrganizationName)
	if err != nil {
		return sitedomain.Site{}, err
	}
	site := sitedomain.New(org.ID, name, strings.TrimSpace(in.Address))
	if err := s.sites.Create(ctx, site); err != nil {
		return sitedomain.Site{}, err
	}
	return site, nil
}

type CreateUserInput struct {
	Username  string
	Password  string
	FirstName string
	LastName  string
	Role      userdomain.Role
	SiteIDs   []uuid.UUID
}

type CreateUserResult struct {
	User            userdomain.User
	PlaintextPassword string // only returned once to admin UI
}

func (s *Service) CreateUser(ctx context.Context, in CreateUserInput) (CreateUserResult, error) {
	username := strings.TrimSpace(in.Username)
	if username == "" {
		return CreateUserResult{}, fmt.Errorf("%w: username required", domain.ErrInvalid)
	}
	if !in.Role.Valid() {
		return CreateUserResult{}, fmt.Errorf("%w: invalid role", domain.ErrInvalid)
	}
	password := strings.TrimSpace(in.Password)
	if password == "" {
		generated, err := authusecase.GeneratePassword()
		if err != nil {
			return CreateUserResult{}, err
		}
		password = generated
	}
	hash, err := authusecase.HashPassword(password)
	if err != nil {
		return CreateUserResult{}, err
	}
	if in.Role != userdomain.RoleAdmin && len(in.SiteIDs) == 0 {
		return CreateUserResult{}, fmt.Errorf("%w: at least one site required", domain.ErrInvalid)
	}
	siteIDs := in.SiteIDs
	orgID := orgdomain.DefaultOrganizationID
	if len(siteIDs) > 0 {
		if site, err := s.sites.GetByID(ctx, siteIDs[0]); err == nil && site.OrganizationID != uuid.Nil {
			orgID = site.OrganizationID
		}
	}
	if in.Role == userdomain.RoleAdmin && len(siteIDs) == 0 {
		siteIDs = []uuid.UUID{orgdomain.DefaultSiteID}
	}
	now := time.Now().UTC()
	u := userdomain.User{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Username:       username,
		PasswordHash:   hash,
		FirstName:      strings.TrimSpace(in.FirstName),
		LastName:       strings.TrimSpace(in.LastName),
		Role:           in.Role,
		IsActive:       true,
		SiteIDs:        siteIDs,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.users.Create(ctx, u); err != nil {
		return CreateUserResult{}, err
	}
	if err := s.users.SetSites(ctx, u.ID, siteIDs); err != nil {
		return CreateUserResult{}, err
	}
	return CreateUserResult{User: u, PlaintextPassword: password}, nil
}

func (s *Service) ListUsers(ctx context.Context) ([]userdomain.User, error) {
	return s.users.ListAll(ctx)
}

func (s *Service) SetUserPassword(ctx context.Context, userID uuid.UUID, password string) (string, error) {
	if strings.TrimSpace(password) == "" {
		generated, err := authusecase.GeneratePassword()
		if err != nil {
			return "", err
		}
		password = generated
	}
	hash, err := authusecase.HashPassword(password)
	if err != nil {
		return "", err
	}
	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return "", err
	}
	u.PasswordHash = hash
	u.UpdatedAt = time.Now().UTC()
	if err := s.users.Update(ctx, u); err != nil {
		return "", err
	}
	if s.sessions != nil {
		s.sessions.InvalidateUserSessions(userID)
	}
	return password, nil
}

func (s *Service) SetUserActive(ctx context.Context, userID uuid.UUID, active bool) error {
	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	u.IsActive = active
	u.UpdatedAt = time.Now().UTC()
	if err := s.users.Update(ctx, u); err != nil {
		return err
	}
	if !active && s.sessions != nil {
		s.sessions.InvalidateUserSessions(userID)
	}
	return nil
}

func (s *Service) ListOpenResetRequests(ctx context.Context) ([]userdomain.PasswordResetRequest, error) {
	if s.resets == nil {
		return nil, nil
	}
	return s.resets.ListOpen(ctx)
}

func (s *Service) ResolveResetAndSetPassword(ctx context.Context, requestID uuid.UUID, password string) (string, error) {
	if s.resets == nil {
		return "", fmt.Errorf("%w: resets not configured", domain.ErrInvalid)
	}
	req, err := s.resets.GetByID(ctx, requestID)
	if err != nil {
		return "", err
	}
	plain, err := s.SetUserPassword(ctx, req.UserID, password)
	if err != nil {
		return "", err
	}
	if err := s.resets.Resolve(ctx, requestID); err != nil {
		return "", err
	}
	return plain, nil
}
